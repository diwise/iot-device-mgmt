package devicemanagement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/infrastructure/storage"
	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/device")

var ErrDeviceNotFound = fmt.Errorf("device not found")
var ErrDeviceAlreadyExist = fmt.Errorf("device already exists")
var ErrDeviceProfileNotFound = fmt.Errorf("device profile not found")
var ErrMissingTenant = fmt.Errorf("missing tenant")

type DeviceReader interface {
	Query(ctx context.Context, conditions ...conditions.ConditionFunc) (types.Collection[types.Device], error)
	GetDeviceBySensorID(ctx context.Context, sensorID string) (types.Device, error)
	GetTenants(ctx context.Context) (types.Collection[string], error)
	GetDeviceAlarms(ctx context.Context, deviceID string) (types.Collection[types.AlarmDetails], error)
	GetDeviceMeasurements(ctx context.Context, deviceID string, conditions ...conditions.ConditionFunc) (types.Collection[types.Measurement], error)
	GetDeviceStatus(ctx context.Context, deviceID string, conditions ...conditions.ConditionFunc) (types.Collection[types.SensorStatus], error)
}

type DeviceWriter interface {
	CreateOrUpdateDevice(ctx context.Context, d types.Device) error
	UpdateDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error
	SetDeviceProfileTypes(ctx context.Context, deviceID string, types []types.Lwm2mType) error
	SetSensorProfile(ctx context.Context, deviceID string, dp types.SensorProfile) error
}

type DeviceStatusWriter interface {
	SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error
	AddDeviceStatus(ctx context.Context, status types.StatusMessage) error
}

type DeviceProfileStore interface {
	CreateSensorProfile(ctx context.Context, p types.SensorProfile) error
	CreateSensorProfileType(ctx context.Context, t types.Lwm2mType) error
}

type DeviceQueryService interface {
	GetBySensorID(ctx context.Context, sensorID string, tenants []string) (types.Device, error)
	GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (types.Device, error)
	GetDeviceStatus(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.SensorStatus], error)
	GetDeviceAlarms(ctx context.Context, deviceID string, tenants []string) (types.Collection[types.AlarmDetails], error)
	GetDeviceMeasurements(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.Measurement], error)
	Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Device], error)
	GetTenants(ctx context.Context) (types.Collection[string], error)
	GetLwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error)
	GetDeviceProfiles(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error)
}

type DeviceCommandService interface {
	NewDevice(ctx context.Context, device types.Device) error
	UpdateDevice(ctx context.Context, device types.Device) error
	MergeDevice(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error
	UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error
}

type DeviceBulkCreateService interface {
	CreateMany(ctx context.Context, devices io.ReadCloser, validTenants []string) error
}

type DeviceBootstrapService interface {
	Seed(ctx context.Context, devices io.ReadCloser, validTenants []string) error
	SeedLwm2mTypes(ctx context.Context, lwm2m []types.Lwm2mType) error
	SeedSensorProfiles(ctx context.Context, profiles []types.SensorProfile) error
}

type DeviceStatusHandler interface {
	HandleStatusMessage(ctx context.Context, status types.StatusMessage) error
}

type DeviceAPIService interface {
	DeviceQueryService
	DeviceCommandService
	DeviceBulkCreateService
}

//go:generate moq -rm -out devicereader_mock.go . DeviceReader
//go:generate moq -rm -out devicewriter_mock.go . DeviceWriter
//go:generate moq -rm -out devicestatuswriter_mock.go . DeviceStatusWriter
//go:generate moq -rm -out deviceprofilestore_mock.go . DeviceProfileStore
//go:generate moq -rm -out devicemanagement_mock.go . DeviceManagement

type DeviceManagement interface {
	DeviceAPIService
	DeviceBootstrapService
	DeviceStatusHandler
}

type Config struct {
	DeviceProfiles      []types.SensorProfile `yaml:"deviceprofiles"`
	Types               []types.Lwm2mType     `yaml:"types"`
	SeedExistingDevices bool                  `yaml:"seedExistingDevices"`
}

type service struct {
	reader       DeviceReader
	writer       DeviceWriter
	statusWriter DeviceStatusWriter
	profiles     DeviceProfileStore
	config       *Config
	messenger    messaging.MsgContext
}

func New(reader DeviceReader, writer DeviceWriter, statusWriter DeviceStatusWriter, profiles DeviceProfileStore, messenger messaging.MsgContext, config *Config) DeviceManagement {
	return service{
		reader:       reader,
		writer:       writer,
		statusWriter: statusWriter,
		profiles:     profiles,
		messenger:    messenger,
		config:       config,
	}
}

func RegisterTopicMessageHandler(ctx context.Context, svc DeviceStatusHandler, messenger messaging.MsgContext) error {
	return messenger.RegisterTopicMessageHandler("device-status", newDeviceStatusHandler(svc))
}

func (s service) HandleStatusMessage(ctx context.Context, status types.StatusMessage) error {
	state := types.DeviceState{
		Online:     true,
		State:      types.DeviceStateOK,
		ObservedAt: status.Timestamp,
	}

	if status.Code != nil {
		state.State = types.DeviceStateWarning
	}

	err := s.statusWriter.SetDeviceState(ctx, status.DeviceID, state)
	if err != nil {
		return err
	}

	if status.BatteryLevel == nil && status.DR == nil && status.Frequency == nil && status.LoRaSNR == nil && status.RSSI == nil && status.SpreadingFactor == nil {
		return nil
	}

	return s.statusWriter.AddDeviceStatus(ctx, status)
}

func (s service) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (types.Device, error) {
	d, err := s.reader.GetDeviceBySensorID(ctx, sensorID)
	if err != nil {
		if errors.Is(err, storage.ErrNoRows) {
			return types.Device{}, ErrDeviceNotFound
		}
		return types.Device{}, err
	}

	if slices.Contains(tenants, d.Tenant) {
		return d, nil
	}

	return types.Device{}, ErrDeviceNotFound
}

func (s service) GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (types.Device, error) {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(deviceID), conditions.WithTenants(tenants))
	if err != nil {
		if errors.Is(err, storage.ErrNoRows) {
			return types.Device{}, ErrDeviceNotFound
		}
		return types.Device{}, err
	}

	if result.Count != 1 {
		return types.Device{}, ErrDeviceNotFound
	}

	return result.Data[0], nil
}

func (s service) GetDeviceStatus(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.SensorStatus], error) {
	if deviceID == "" {
		return types.Collection[types.SensorStatus]{}, ErrDeviceNotFound
	}

	if len(tenants) == 0 {
		return types.Collection[types.SensorStatus]{}, ErrMissingTenant
	}

	conds := conditions.Parse(ctx, params)
	conds = append(conds, conditions.WithTenants(tenants))

	return s.reader.GetDeviceStatus(ctx, deviceID, conds...)
}

func (s service) GetDeviceAlarms(ctx context.Context, deviceID string, tenants []string) (types.Collection[types.AlarmDetails], error) {
	_, err := s.GetByDeviceID(ctx, deviceID, tenants)
	if err != nil {
		return types.Collection[types.AlarmDetails]{}, err
	}

	return s.reader.GetDeviceAlarms(ctx, deviceID)
}

func (s service) NewDevice(ctx context.Context, device types.Device) error {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(device.DeviceID))
	if err != nil {
		return err
	}

	if result.Count > 0 {
		return ErrDeviceAlreadyExist
	}

	err = s.writer.CreateOrUpdateDevice(ctx, device)
	if err != nil {
		return err
	}

	if len(device.SensorProfile.Types) > 0 {
		l := []types.Lwm2mType{}
		for _, t := range device.SensorProfile.Types {
			l = append(l, types.Lwm2mType{
				Urn:  t,
				Name: t,
			})
		}

		s.writer.SetDeviceProfileTypes(ctx, device.DeviceID, l)
	}

	return nil
}

func (s service) UpdateDevice(ctx context.Context, device types.Device) error {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(device.DeviceID))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	err = s.writer.CreateOrUpdateDevice(ctx, device)
	if err != nil {
		return err
	}

	return nil
}

func (s service) MergeDevice(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
	log := logging.GetFromContext(ctx)

	result, err := s.reader.Query(ctx, conditions.WithDeviceID(deviceID), conditions.WithTenants(tenants))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	if result.Count > 1 {
		return fmt.Errorf("too many devices found")
	}

	var active *bool
	var name, description, environment, source, tenant, deviceProfile *string
	var location *types.Location
	var lwm2m []string
	var interval *int

	for k, v := range fields {
		switch k {
		case "deviceID":
			continue
		case "active":
			b := v.(bool)
			active = &b
		case "description":
			s := v.(string)
			description = &s
		case "latitude":
			lat := v.(float64)
			if location == nil {
				location = &types.Location{}
			}
			location.Latitude = lat
		case "longitude":
			lon := v.(float64)
			if location == nil {
				location = &types.Location{}
			}
			location.Longitude = lon
		case "name":
			s := v.(string)
			name = &s
		case "environment":
			s := v.(string)
			environment = &s
		case "source":
			s := v.(string)
			source = &s
		case "tenant":
			s := v.(string)
			tenant = &s
		case "types":
			types := v.([]any)
			for _, typ := range types {
				s := typ.(string)
				lwm2m = append(lwm2m, s)
			}
		case "deviceProfile":
			s := v.(string)
			deviceProfile = &s
		case "interval":
			s := v.(string)
			if i, err := strconv.Atoi(s); err == nil {
				interval = &i
			}
		default:
			log.Debug("field not mapped for merge", "device_id", deviceID, "name", k)
		}
	}

	err = s.writer.UpdateDevice(ctx, deviceID, active, name, description, environment, source, tenant, location, interval)
	if err != nil {
		log.Error("could not update device information", "err", err.Error())
		return err
	}

	if deviceProfile != nil {
		err = s.writer.SetSensorProfile(ctx, deviceID, types.SensorProfile{
			Decoder: *deviceProfile,
		})
		if err != nil {
			log.Error("could not set device profile for device", "device_id", deviceID, "profile", deviceProfile, "err", err.Error())
			return err
		}
	}

	if len(lwm2m) > 0 {
		l := []types.Lwm2mType{}
		for _, t := range lwm2m {
			if t == "" {
				continue
			}
			l = append(l, types.Lwm2mType{
				Urn: strings.ToLower(strings.TrimSpace(t)),
			})
		}

		err = s.writer.SetDeviceProfileTypes(ctx, deviceID, l)
		if err != nil {
			log.Error("could not set lwm2m types for device", "device_id", deviceID, "err", err.Error())
			return err
		}
	}

	return nil
}
func (s service) UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(deviceID), conditions.WithTenant(tenant))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	return s.statusWriter.SetDeviceState(ctx, deviceID, deviceState)
}

func (s service) Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Device], error) {
	conds := conditions.Parse(ctx, params)
	conds = append(conds, conditions.WithTenants(tenants))

	return s.reader.Query(ctx, conds...)
}

func (s service) GetTenants(ctx context.Context) (types.Collection[string], error) {
	return s.reader.GetTenants(ctx)
}

func (s service) GetLwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
	var collection types.Collection[types.Lwm2mType]

	if len(urn) > 0 && urn[0] != "" {
		lwm2mTypes := []types.Lwm2mType{}

		for _, u := range urn {
			id := slices.IndexFunc(s.config.Types, func(p types.Lwm2mType) bool {
				return u == p.Urn
			})
			if id > -1 {
				lwm2mTypes = append(lwm2mTypes, s.config.Types[id])
			}
		}

		if len(lwm2mTypes) > 0 {
			collection = types.Collection[types.Lwm2mType]{
				Data:       lwm2mTypes,
				Count:      uint64(len(lwm2mTypes)),
				Offset:     0,
				Limit:      uint64(len(lwm2mTypes)),
				TotalCount: uint64(len(lwm2mTypes)),
			}
			return collection, nil
		}

		return types.Collection[types.Lwm2mType]{}, ErrDeviceProfileNotFound
	}

	collection = types.Collection[types.Lwm2mType]{
		Data:       s.config.Types,
		Count:      uint64(len(s.config.Types)),
		Offset:     0,
		Limit:      uint64(len(s.config.Types)),
		TotalCount: uint64(len(s.config.Types)),
	}

	return collection, nil
}

func (s service) GetDeviceProfiles(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
	var collection types.Collection[types.SensorProfile]

	if len(name) > 0 && name[0] != "" {
		profiles := []types.SensorProfile{}

		for _, n := range name {
			id := slices.IndexFunc(s.config.DeviceProfiles, func(p types.SensorProfile) bool {
				return n == p.Name
			})
			if id > -1 {
				profiles = append(profiles, s.config.DeviceProfiles[id])
			}
		}

		if len(profiles) > 0 {
			collection = types.Collection[types.SensorProfile]{
				Data:       profiles,
				Count:      uint64(len(profiles)),
				Offset:     0,
				Limit:      uint64(len(profiles)),
				TotalCount: uint64(len(profiles)),
			}
			return collection, nil
		}

		return types.Collection[types.SensorProfile]{}, ErrDeviceProfileNotFound
	}

	collection = types.Collection[types.SensorProfile]{
		Data:       s.config.DeviceProfiles,
		Count:      uint64(len(s.config.DeviceProfiles)),
		Offset:     0,
		Limit:      uint64(len(s.config.DeviceProfiles)),
		TotalCount: uint64(len(s.config.DeviceProfiles)),
	}

	return collection, nil
}

func (s service) GetDeviceMeasurements(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.Measurement], error) {
	conds := conditions.Parse(ctx, params)

	conds = append(conds, conditions.WithDeviceID(deviceID))
	conds = append(conds, conditions.WithTenants(tenants))

	return s.reader.GetDeviceMeasurements(ctx, deviceID, conds...)
}

func newDeviceStatusHandler(svc DeviceStatusHandler) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		//log.Debug("received device status", "service", "devicemanagement", "body", string(itm.Body()))

		m := types.StatusMessage{}
		err = json.Unmarshal(itm.Body(), &m)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		ctx = logging.NewContextWithLogger(ctx, log, slog.String("device_id", m.DeviceID), slog.String("tenant", m.Tenant))

		err = svc.HandleStatusMessage(ctx, m)
		if err != nil {
			log.Error("could not add device status", "err", err.Error())
			return
		}
	}
}
