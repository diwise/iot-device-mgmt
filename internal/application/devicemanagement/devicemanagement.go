package devicemanagement

import (
	"context"
	"io"

	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
)

var ErrDeviceNotFound = errDeviceNotFound
var ErrDeviceAlreadyExist = errDeviceAlreadyExist
var ErrDeviceProfileNotFound = errDeviceProfileNotFound
var ErrMissingTenant = errMissingTenant

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

func New(reader DeviceReader, writer DeviceWriter, statusWriter DeviceStatusWriter, profiles DeviceProfileStore, messenger messaging.MsgContext, config *Config) *service {
	return &service{
		reader:       reader,
		writer:       writer,
		statusWriter: statusWriter,
		profiles:     profiles,
		messenger:    messenger,
		config:       config,
	}
}
