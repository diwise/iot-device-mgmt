package devicemanagement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
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

//go:generate moq -rm -out devicemanagement_mock.go . DeviceManagement
type DeviceManagement interface {
	GetBySensorID(ctx context.Context, sensorID string, tenants []string) (types.Device, error)
	GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (types.Device, error)
	GetDeviceStatus(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.DeviceStatus], error)
	GetDeviceAlarms(ctx context.Context, deviceID string, tenants []string) (types.Collection[types.AlarmDetails], error)

	NewDevice(ctx context.Context, device types.Device) error
	UpdateDevice(ctx context.Context, device types.Device) error
	MergeDevice(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error

	UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error

	GetLwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error)
	GetDeviceProfiles(ctx context.Context, name ...string) (types.Collection[types.DeviceProfile], error)
	GetTenants(ctx context.Context) (types.Collection[string], error)

	Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Device], error)

	// -----------------
	HandleStatusMessage(ctx context.Context, status types.StatusMessage) error
	Config() *DeviceManagementConfig

	GetDeviceMeasurements(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.Measurement], error)

	RegisterTopicMessageHandler(ctx context.Context) error
}

type DeviceManagementConfig struct {
	DeviceProfiles []types.DeviceProfile `yaml:"deviceprofiles"`
	Types          []types.Lwm2mType     `yaml:"types"`
}

type service struct {
	storage   DeviceStorage
	config    *DeviceManagementConfig
	messenger messaging.MsgContext
}

func (s service) Config() *DeviceManagementConfig {
	return s.config
}

//go:generate moq -rm -out devicestorage_mock.go . DeviceStorage
type DeviceStorage interface {
	AddDeviceStatus(ctx context.Context, status types.StatusMessage) error
	Query(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error)
	CreateOrUpdateDevice(ctx context.Context, d types.Device) error
	SetDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error
	SetDeviceProfile(ctx context.Context, deviceID string, dp types.DeviceProfile) error
	SetDeviceProfileTypes(ctx context.Context, deviceID string, types []types.Lwm2mType) error
	SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error
	GetTenants(ctx context.Context) (types.Collection[string], error)
	GetDeviceStatus(ctx context.Context, deviceID string, conditions ...storage.ConditionFunc) (types.Collection[types.DeviceStatus], error)
	GetDeviceAlarms(ctx context.Context, deviceID string) (types.Collection[types.AlarmDetails], error)
	GetDeviceMeasurements(ctx context.Context, deviceID string, conditions ...storage.ConditionFunc) (types.Collection[types.Measurement], error)
	GetDeviceBySensorID(ctx context.Context, sensorID string) (types.Device, error)
}
type deviceStorageImpl struct {
	s storage.Store
}

func (d deviceStorageImpl) AddDeviceStatus(ctx context.Context, status types.StatusMessage) error {
	return d.s.AddDeviceStatus(ctx, status)
}
func (d deviceStorageImpl) Query(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
	return d.s.Query(ctx, conditions...)
}
func (d deviceStorageImpl) CreateOrUpdateDevice(ctx context.Context, device types.Device) error {
	return d.s.CreateOrUpdateDevice(ctx, device)
}
func (d deviceStorageImpl) SetDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error {
	return d.s.SetDevice(ctx, deviceID, active, name, description, environment, source, tenant, location, interval)
}
func (d deviceStorageImpl) SetDeviceProfile(ctx context.Context, deviceID string, dp types.DeviceProfile) error {
	return d.s.SetDeviceProfile(ctx, deviceID, dp)
}
func (d deviceStorageImpl) SetDeviceProfileTypes(ctx context.Context, deviceID string, types []types.Lwm2mType) error {
	return d.s.SetDeviceProfileTypes(ctx, deviceID, types)
}
func (d deviceStorageImpl) SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error {
	return d.s.SetDeviceState(ctx, deviceID, state)
}
func (d deviceStorageImpl) GetTenants(ctx context.Context) (types.Collection[string], error) {
	return d.s.GetTenants(ctx)
}
func (d deviceStorageImpl) GetDeviceStatus(ctx context.Context, deviceID string, conditions ...storage.ConditionFunc) (types.Collection[types.DeviceStatus], error) {
	return d.s.GetDeviceStatus(ctx, deviceID, conditions...)
}
func (d deviceStorageImpl) GetDeviceAlarms(ctx context.Context, deviceID string) (types.Collection[types.AlarmDetails], error) {
	return d.s.GetDeviceAlarms(ctx, deviceID)
}
func (d deviceStorageImpl) GetDeviceMeasurements(ctx context.Context, deviceID string, conditions ...storage.ConditionFunc) (types.Collection[types.Measurement], error) {
	return d.s.GetDeviceMeasurements(ctx, deviceID, conditions...)
}
func (d deviceStorageImpl) GetDeviceBySensorID(ctx context.Context, sensorID string) (types.Device, error) {
	return d.s.GetDeviceBySensorID(ctx, sensorID)
}

func NewStorage(s storage.Store) DeviceStorage {
	return &deviceStorageImpl{
		s: s,
	}
}

func New(storage DeviceStorage, messenger messaging.MsgContext, config *DeviceManagementConfig) DeviceManagement {
	s := service{
		storage:   storage,
		messenger: messenger,
		config:    config,
	}

	return s
}

func (s service) RegisterTopicMessageHandler(ctx context.Context) error {
	return s.messenger.RegisterTopicMessageHandler("device-status", NewDeviceStatusHandler(s))
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

	err := s.storage.SetDeviceState(ctx, status.DeviceID, state)
	if err != nil {
		return err
	}

	if status.BatteryLevel == nil && status.DR == nil && status.Frequency == nil && status.LoRaSNR == nil && status.RSSI == nil && status.SpreadingFactor == nil {
		return nil
	}

	return s.storage.AddDeviceStatus(ctx, status)
}

func (s service) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (types.Device, error) {
	d, err := s.storage.GetDeviceBySensorID(ctx, sensorID)
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
	result, err := s.storage.Query(ctx, storage.WithDeviceID(deviceID), storage.WithTenants(tenants))
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

func (s service) GetDeviceStatus(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.DeviceStatus], error) {
	if deviceID == "" {
		return types.Collection[types.DeviceStatus]{}, ErrDeviceNotFound
	}

	if len(tenants) == 0 {
		return types.Collection[types.DeviceStatus]{}, ErrMissingTenant
	}

	conditions := storage.ParseConditions(ctx, params)
	conditions = append(conditions, storage.WithTenants(tenants))

	return s.storage.GetDeviceStatus(ctx, deviceID, conditions...)
}

func (s service) GetDeviceAlarms(ctx context.Context, deviceID string, tenants []string) (types.Collection[types.AlarmDetails], error) {
	_, err := s.GetByDeviceID(ctx, deviceID, tenants)
	if err != nil {
		return types.Collection[types.AlarmDetails]{}, err
	}

	return s.storage.GetDeviceAlarms(ctx, deviceID)
}

func (s service) NewDevice(ctx context.Context, device types.Device) error {
	result, err := s.storage.Query(ctx, storage.WithDeviceID(device.DeviceID))
	if err != nil {
		return err
	}

	if result.Count > 0 {
		return ErrDeviceAlreadyExist
	}

	err = s.storage.CreateOrUpdateDevice(ctx, device)
	if err != nil {
		return err
	}

	if len(device.DeviceProfile.Types) > 0 {
		l := []types.Lwm2mType{}
		for _, t := range device.DeviceProfile.Types {
			l = append(l, types.Lwm2mType{
				Urn:  t,
				Name: t,
			})
		}

		s.storage.SetDeviceProfileTypes(ctx, device.DeviceID, l)
	}

	return nil
}

func (s service) UpdateDevice(ctx context.Context, device types.Device) error {
	result, err := s.storage.Query(ctx, storage.WithDeviceID(device.DeviceID))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	err = s.storage.CreateOrUpdateDevice(ctx, device)
	if err != nil {
		return err
	}

	return nil
}

func (s service) MergeDevice(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
	log := logging.GetFromContext(ctx)

	result, err := s.storage.Query(ctx, storage.WithDeviceID(deviceID), storage.WithTenants(tenants))
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

	err = s.storage.SetDevice(ctx, deviceID, active, name, description, environment, source, tenant, location, interval)
	if err != nil {
		log.Error("could not set device information", "err", err.Error())
		return err
	}

	if deviceProfile != nil {
		err = s.storage.SetDeviceProfile(ctx, deviceID, types.DeviceProfile{
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

		err = s.storage.SetDeviceProfileTypes(ctx, deviceID, l)
		if err != nil {
			log.Error("could not set lwm2m types for device", "device_id", deviceID, "err", err.Error())
			return err
		}
	}

	return nil
}
func (s service) UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error {
	result, err := s.storage.Query(ctx, storage.WithDeviceID(deviceID), storage.WithTenant(tenant))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	return s.storage.SetDeviceState(ctx, deviceID, deviceState)
}

func (s service) Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Device], error) {
	conditions := storage.ParseConditions(ctx, params)
	conditions = append(conditions, storage.WithTenants(tenants))

	return s.storage.Query(ctx, conditions...)
}

func (s service) GetTenants(ctx context.Context) (types.Collection[string], error) {
	return s.storage.GetTenants(ctx)
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

func (s service) GetDeviceProfiles(ctx context.Context, name ...string) (types.Collection[types.DeviceProfile], error) {
	var collection types.Collection[types.DeviceProfile]

	if len(name) > 0 && name[0] != "" {
		profiles := []types.DeviceProfile{}

		for _, n := range name {
			id := slices.IndexFunc(s.config.DeviceProfiles, func(p types.DeviceProfile) bool {
				return n == p.Name
			})
			if id > -1 {
				profiles = append(profiles, s.config.DeviceProfiles[id])
			}
		}

		if len(profiles) > 0 {
			collection = types.Collection[types.DeviceProfile]{
				Data:       profiles,
				Count:      uint64(len(profiles)),
				Offset:     0,
				Limit:      uint64(len(profiles)),
				TotalCount: uint64(len(profiles)),
			}
			return collection, nil
		}

		return types.Collection[types.DeviceProfile]{}, ErrDeviceProfileNotFound
	}

	collection = types.Collection[types.DeviceProfile]{
		Data:       s.config.DeviceProfiles,
		Count:      uint64(len(s.config.DeviceProfiles)),
		Offset:     0,
		Limit:      uint64(len(s.config.DeviceProfiles)),
		TotalCount: uint64(len(s.config.DeviceProfiles)),
	}

	return collection, nil
}

func (s service) GetDeviceMeasurements(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.Measurement], error) {
	conditions := storage.ParseConditions(ctx, params)

	conditions = append(conditions, storage.WithDeviceID(deviceID))
	conditions = append(conditions, storage.WithTenants(tenants))

	return s.storage.GetDeviceMeasurements(ctx, deviceID, conditions...)
}

func NewDeviceStatusHandler(svc DeviceManagement) messaging.TopicMessageHandler {
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
