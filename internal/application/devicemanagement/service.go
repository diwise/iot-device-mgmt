package devicemanagement

import (
	"context"
	"io"

	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devicemanagement/query"
	"github.com/diwise/iot-device-mgmt/internal/application/sensormanagement"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
)

var ErrDeviceNotFound = errDeviceNotFound
var ErrDeviceAlreadyExist = errDeviceAlreadyExist
var ErrDeviceProfileNotFound = errDeviceProfileNotFound
var ErrMissingTenant = errMissingTenant
var ErrInvalidPatch = errInvalidPatch
var ErrSensorNotFound = errSensorNotFound
var ErrSensorAlreadyAssigned = errSensorAlreadyAssigned
var ErrSensorProfileRequired = errSensorProfileRequired

type DeviceReader interface {
	Query(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error)
	GetDeviceBySensorID(ctx context.Context, sensorID string) (types.Device, bool, error)
	GetSensor(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error)
	GetTenants(ctx context.Context) (types.Collection[string], error)
	GetDeviceAlarms(ctx context.Context, deviceID string) (types.Collection[types.AlarmDetails], error)
	GetDeviceMeasurements(ctx context.Context, deviceID string, query dmquery.Measurements) (types.Collection[types.Measurement], error)
	GetDeviceStatus(ctx context.Context, deviceID string, query dmquery.Status) (types.Collection[types.SensorStatus], error)
}

type DeviceWriter interface {
	CreateOrUpdateDevice(ctx context.Context, d types.Device) error
	UpdateDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error
	SetDeviceProfileTypes(ctx context.Context, deviceID string, types []types.Lwm2mType) error
	AssignSensor(ctx context.Context, deviceID, sensorID string) error
	UnassignSensor(ctx context.Context, deviceID string) error
}

type DeviceStatusWriter interface {
	SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error
	AddDeviceStatus(ctx context.Context, status types.StatusMessage) error
}

type DeviceProfileStore interface {
	CreateSensorProfile(ctx context.Context, p types.SensorProfile) error
	CreateSensorProfileType(ctx context.Context, t types.Lwm2mType) error
}

/* ----- SERVICE INTERFACES ----- */

type DeviceQueryService interface {
	DeviceBySensor(ctx context.Context, sensorID string, tenants []string) (types.Device, error)
	Device(ctx context.Context, deviceID string, tenants []string) (types.Device, error)
	Status(ctx context.Context, deviceID string, query dmquery.Status) (types.Collection[types.SensorStatus], error)
	Alarms(ctx context.Context, deviceID string, tenants []string) (types.Collection[types.AlarmDetails], error)
	Measurements(ctx context.Context, deviceID string, query dmquery.Measurements) (types.Collection[types.Measurement], error)
	Query(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error)
	Tenants(ctx context.Context) (types.Collection[string], error)
	Lwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error)
	Profiles(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error)
}

type DeviceCommandService interface {
	Create(ctx context.Context, device types.Device) error
	CreateMany(ctx context.Context, devices io.ReadCloser, validTenants []string) error
	Update(ctx context.Context, device types.Device) error
	Merge(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error
	AttachSensor(ctx context.Context, deviceID, sensorID string, tenants []string) error
	DetachSensor(ctx context.Context, deviceID string, tenants []string) error
	UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error
}

type DeviceBootstrapService interface {
	Seed(ctx context.Context, devices io.ReadCloser, validTenants []string) error
	SeedLwm2mTypes(ctx context.Context, lwm2m []types.Lwm2mType) error
	SeedSensorProfiles(ctx context.Context, profiles []types.SensorProfile) error
}

type DeviceStatusHandler interface {
	Handle(ctx context.Context, status types.StatusMessage) error
}

type DeviceAPIService interface {
	DeviceQueryService
	DeviceCommandService
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
