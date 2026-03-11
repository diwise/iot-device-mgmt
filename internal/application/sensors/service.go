package sensors

import (
	"context"
	"errors"

	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

var ErrSensorNotFound = errors.New("sensor not found")
var ErrSensorAlreadyExists = errors.New("sensor already exists")

type Sensor struct {
	SensorID      string               `json:"sensorID"`
	DeviceID      *string              `json:"deviceID,omitempty"`
	SensorProfile *types.SensorProfile `json:"sensorProfile,omitempty"`
}

type SensorReader interface {
	QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[Sensor], error)
	GetSensor(ctx context.Context, sensorID string) (Sensor, bool, error)
}

type SensorWriter interface {
	CreateSensor(ctx context.Context, sensor Sensor) error
	UpdateSensor(ctx context.Context, sensor Sensor) error
}

type SensorQueryService interface {
	Query(ctx context.Context, query sensorquery.Sensors) (types.Collection[Sensor], error)
	Sensor(ctx context.Context, sensorID string) (Sensor, error)
}

type SensorCommandService interface {
	Create(ctx context.Context, sensor Sensor) error
	Update(ctx context.Context, sensor Sensor) error
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
