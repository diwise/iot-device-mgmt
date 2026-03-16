package sensors

import (
	"context"
	"errors"

	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

var ErrSensorNotFound = errors.New("sensor not found")
var ErrSensorAlreadyExists = errors.New("sensor already exists")
var ErrSensorProfileNotFound = errors.New("sensor profile not found")

type SensorReader interface {
	QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error)
	GetSensor(ctx context.Context, sensorID string) (types.Sensor, bool, error)
	GetSensorProfile(ctx context.Context, profileID string) (types.SensorProfile, bool, error)
}

type SensorWriter interface {
	CreateSensor(ctx context.Context, sensor types.Sensor) error
	UpdateSensor(ctx context.Context, sensor types.Sensor) error
}

type SensorQueryService interface {
	Query(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error)
	Sensor(ctx context.Context, sensorID string) (types.Sensor, error)
	SensorProfile(ctx context.Context, profileID string) (types.SensorProfile, error)
}

type SensorCommandService interface {
	Create(ctx context.Context, sensor types.Sensor) error
	Update(ctx context.Context, sensor types.Sensor) error
}

type SensorAPIService interface {
	SensorQueryService
	SensorCommandService
}

type service struct {
	reader SensorReader
	writer SensorWriter
}

func New(reader SensorReader, writer SensorWriter) *service {
	return &service{
		reader: reader,
		writer: writer,
	}
}
