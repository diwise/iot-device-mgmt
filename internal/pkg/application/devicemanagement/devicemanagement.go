package devicemanagement

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

var ErrDeviceNotFound = fmt.Errorf("device not found")
var ErrDeviceProfileNotFound = fmt.Errorf("device profile not found")

//go:generate moq -rm -out devicemanagement_mock.go . DeviceManagement
type DeviceManagement interface {
	GetBySensorID(ctx context.Context, sensorID string, tenants []string) (types.Device, error)
	GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (types.Device, error)
	GetOnlineDevices(ctx context.Context, offset, limit int) (types.Collection[types.Device], error)
	GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (types.Device, error)
	GetWithinBounds(ctx context.Context, bounds types.Bounds) (types.Collection[types.Device], error)

	Create(ctx context.Context, device types.Device) error
	Update(ctx context.Context, device types.Device) error
	Merge(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error

	UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus types.DeviceStatus) error
	UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error

	Seed(ctx context.Context, reader io.Reader, tenants []string) error

	GetLwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error)
	GetDeviceProfiles(ctx context.Context, name ...string) (types.Collection[types.DeviceProfile], error)
	GetTenants(ctx context.Context) (types.Collection[string], error)

	Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Device], error)
}

type DeviceManagementConfig struct {
	DeviceProfiles []types.DeviceProfile `yaml:"deviceprofiles"`
	Types          []types.Lwm2mType     `yaml:"types"`
}

//go:generate moq -rm -out devicerepository_mock.go . DeviceRepository
type DeviceRepository interface {
	GetDevice(ctx context.Context, conditions ...storage.ConditionFunc) (types.Device, error)
	QueryDevices(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error)
	AddDevice(ctx context.Context, device types.Device) error
	UpdateDevice(ctx context.Context, device types.Device) error
	UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus types.DeviceStatus) error
	UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error
	GetTenants(ctx context.Context) ([]string, error)
}

type service struct {
	storage   DeviceRepository
	config    *DeviceManagementConfig
	messenger messaging.MsgContext
}

func New(storage DeviceRepository, messenger messaging.MsgContext, config *DeviceManagementConfig) DeviceManagement {
	s := service{
		storage:   storage,
		messenger: messenger,
		config:    config,
	}

	s.messenger.RegisterTopicMessageHandler("device-status", NewDeviceStatusHandler(s))
	s.messenger.RegisterTopicMessageHandler("alarms.alarmCreated", NewAlarmCreatedHandler(s))
	s.messenger.RegisterTopicMessageHandler("alarms.alarmClosed", NewAlarmClosedHandler(s))
	s.messenger.RegisterTopicMessageHandler("message.accepted", NewMessageAcceptedHandler(s))

	return s
}

func (s service) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (types.Device, error) {
	device, err := s.storage.GetDevice(ctx, storage.WithSensorID(sensorID), storage.WithTenants(tenants))
	if err != nil {
		if errors.Is(err, storage.ErrNoRows) {
			return types.Device{}, ErrDeviceNotFound
		}
		return types.Device{}, err
	}
	return device, nil
}

func (s service) GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (types.Device, error) {
	device, err := s.storage.GetDevice(ctx, storage.WithDeviceID(deviceID), storage.WithTenants(tenants))
	if err != nil {
		if errors.Is(err, storage.ErrNoRows) {
			return types.Device{}, ErrDeviceNotFound
		}
		return types.Device{}, err
	}
	return device, nil
}

func (s service) GetOnlineDevices(ctx context.Context, offset, limit int) (types.Collection[types.Device], error) {
	return s.storage.QueryDevices(ctx, storage.WithOnline(true), storage.WithOffset(offset), storage.WithLimit(limit))
}

func (s service) GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (types.Device, error) {
	device, err := s.storage.GetDevice(ctx, storage.WithDeviceAlarmID(alarmID), storage.WithTenants(tenants))
	if err != nil {
		if errors.Is(err, storage.ErrNoRows) {
			return types.Device{}, ErrDeviceNotFound
		}
		return types.Device{}, err
	}
	return device, nil
}

func (s service) GetWithinBounds(ctx context.Context, b types.Bounds) (types.Collection[types.Device], error) {
	return s.storage.QueryDevices(ctx, storage.WithBounds(b.MaxLat, b.MinLat, b.MaxLon, b.MinLon))
}

func (s service) Create(ctx context.Context, device types.Device) error {
	err := s.storage.AddDevice(ctx, device)
	if err != nil {
		return err
	}

	return s.messenger.PublishOnTopic(ctx, &types.DeviceCreated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant,
		Timestamp: time.Now().UTC(),
	})
}

func (s service) Update(ctx context.Context, device types.Device) error {
	return s.storage.UpdateDevice(ctx, device)
}

func (s service) UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus types.DeviceStatus) error {
	if deviceStatus.ObservedAt.IsZero() {
		deviceStatus.ObservedAt = time.Now().UTC()
	}

	err := s.storage.UpdateStatus(ctx, deviceID, tenant, deviceStatus)
	if err != nil {
		return err
	}

	return s.messenger.PublishOnTopic(ctx, &types.DeviceStatusUpdated{
		DeviceID:  deviceID,
		Tenant:    tenant,
		Timestamp: deviceStatus.ObservedAt.UTC(),
	})
}

func (s service) UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error {
	if deviceState.ObservedAt.IsZero() {
		deviceState.ObservedAt = time.Now().UTC()
	}

	err := s.storage.UpdateState(ctx, deviceID, tenant, deviceState)
	if err != nil {
		return err
	}

	return s.messenger.PublishOnTopic(ctx, &types.DeviceStateUpdated{
		DeviceID:  deviceID,
		Tenant:    tenant,
		State:     deviceState.State,
		Timestamp: deviceState.ObservedAt.UTC(),
	})
}

func (s service) GetTenants(ctx context.Context) (types.Collection[string], error) {
	tenants, err := s.storage.GetTenants(ctx)
	if err != nil {
		return types.Collection[string]{}, err
	}
	return types.Collection[string]{
		Data:       tenants,
		Count:      uint64(len(tenants)),
		Offset:     0,
		Limit:      uint64(len(tenants)),
		TotalCount: uint64(len(tenants)),
	}, nil
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

func extractCoordsFromQuery(bounds string) types.Bounds {
	trimmed := strings.Trim(bounds, "[]")

	pairs := strings.Split(trimmed, ";")

	coords1 := strings.Split(pairs[0], ",")
	coords2 := strings.Split(pairs[1], ",")

	seLat, _ := strconv.ParseFloat(coords1[0], 64)
	nwLon, _ := strconv.ParseFloat(coords1[1], 64)
	nwLat, _ := strconv.ParseFloat(coords2[0], 64)
	seLon, _ := strconv.ParseFloat(coords2[1], 64)

	coords := types.Bounds{
		MinLat: seLat,
		MinLon: nwLon,
		MaxLat: nwLat,
		MaxLon: seLon,
	}

	return coords
}

func (s service) Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Device], error) {
	conditions := make([]storage.ConditionFunc, 0)

	conditions = append(conditions, storage.WithTenants(tenants))

	for k, v := range params {
		switch strings.ToLower(k) {
		case "deveui":
			conditions = append(conditions, storage.WithSensorID(v[0]))
		case "device_id":
			conditions = append(conditions, storage.WithDeviceID(v[0]))
		case "sensor_id":
			conditions = append(conditions, storage.WithSensorID(v[0]))
		case "type":
			conditions = append(conditions, storage.WithTypes(v))
		case "types":
			conditions = append(conditions, storage.WithTypes(v))
		case "active":
			active, _ := strconv.ParseBool(v[0])
			conditions = append(conditions, storage.WithActive(active))
		case "online":
			online, _ := strconv.ParseBool(v[0])
			conditions = append(conditions, storage.WithOnline(online))
		case "limit":
			limit, _ := strconv.Atoi(v[0])
			conditions = append(conditions, storage.WithLimit(limit))
		case "offset":
			offset, _ := strconv.Atoi(v[0])
			conditions = append(conditions, storage.WithOffset(offset))
		case "sortby":
			conditions = append(conditions, storage.WithSortBy(v[0]))
		case "sortorder":
			conditions = append(conditions, storage.WithSortDesc(strings.EqualFold(v[0], "desc")))
		case "bounds":
			coords := extractCoordsFromQuery(v[0])
			conditions = append(conditions, storage.WithBounds(coords.MaxLat, coords.MinLat, coords.MaxLon, coords.MinLon))
		case "profilename":
			conditions = append(conditions, storage.WithProfileName(v))
		}
	}

	return s.storage.QueryDevices(ctx, conditions...)
}

func (s service) Merge(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
	log := logging.GetFromContext(ctx)

	device, err := s.storage.GetDevice(ctx, storage.WithDeviceID(deviceID), storage.WithTenants(tenants))
	if err != nil {
		return err
	}

	for k, v := range fields {
		switch k {
		case "deviceID":
			continue
		case "active":
			device.Active = v.(bool)
		case "description":
			device.Description = v.(string)
		case "latitude":
			lat := v.(float64)
			device.Location.Latitude = lat
		case "longitude":
			lon := v.(float64)
			device.Location.Longitude = lon
		case "name":
			device.Name = v.(string)
		case "tenant":
			device.Tenant = v.(string)
		case "types":
			typs := []string{}
			if anys, ok := v.([]any); ok {
				for _, a := range anys {
					if s, ok := a.(string); ok {
						typs = append(typs, s)
					}
				}
			}
			if types, err := s.GetLwm2mTypes(ctx, typs...); err == nil {
				device.Lwm2mTypes = types.Data
			}
		case "deviceProfile":
			if deviceProfile, err := s.GetDeviceProfiles(ctx, v.(string)); err == nil {
				if deviceProfile.Count == 1 {
					device.DeviceProfile = deviceProfile.Data[0]
				}
			}
		default:
			log.Debug("MERGE: key not found", slog.String("key", k), slog.Any("value", v))
		}
	}

	return s.storage.UpdateDevice(ctx, device)
}
