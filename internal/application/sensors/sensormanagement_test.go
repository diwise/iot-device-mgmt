package sensors

import (
	"context"
	"errors"
	"testing"

	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/matryer/is"
)

type readerStub struct {
	queryFunc func(context.Context, sensorquery.Sensors) (types.Collection[Sensor], error)
	getFunc   func(context.Context, string) (Sensor, bool, error)
}

func (s readerStub) QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[Sensor], error) {
	return s.queryFunc(ctx, query)
}

func (s readerStub) GetSensor(ctx context.Context, sensorID string) (Sensor, bool, error) {
	return s.getFunc(ctx, sensorID)
}

type writerStub struct {
	createFunc func(context.Context, Sensor) error
	updateFunc func(context.Context, Sensor) error
}

func (s writerStub) CreateSensor(ctx context.Context, sensor Sensor) error {
	return s.createFunc(ctx, sensor)
}

func (s writerStub) UpdateSensor(ctx context.Context, sensor Sensor) error {
	return s.updateFunc(ctx, sensor)
}

func TestSensor(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (Sensor, bool, error) {
			return Sensor{SensorID: "sensor-1"}, true, nil
		},
	}, writerStub{})

	sensor, err := svc.Sensor(context.Background(), "sensor-1")
	is.NoErr(err)
	is.Equal(sensor.SensorID, "sensor-1")
}

func TestSensorNotFound(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (Sensor, bool, error) {
			return Sensor{}, false, nil
		},
	}, writerStub{})

	_, err := svc.Sensor(context.Background(), "missing")
	is.True(errors.Is(err, ErrSensorNotFound))
}

func TestCreateSensorAlreadyExists(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (Sensor, bool, error) {
			return Sensor{SensorID: "sensor-1"}, true, nil
		},
	}, writerStub{})

	err := svc.Create(context.Background(), Sensor{SensorID: "sensor-1"})
	is.True(errors.Is(err, ErrSensorAlreadyExists))
}

func TestUpdateSensorNotFound(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (Sensor, bool, error) {
			return Sensor{}, false, nil
		},
	}, writerStub{})

	err := svc.Update(context.Background(), Sensor{SensorID: "missing"})
	is.True(errors.Is(err, ErrSensorNotFound))
}

func TestQuerySensors(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		queryFunc: func(context.Context, sensorquery.Sensors) (types.Collection[Sensor], error) {
			return types.Collection[Sensor]{
				Data:  []Sensor{{SensorID: "sensor-1", SensorProfile: &types.SensorProfile{Decoder: "elsys"}}},
				Count: 1,
			}, nil
		},
		getFunc: func(context.Context, string) (Sensor, bool, error) {
			return Sensor{}, false, nil
		},
	}, writerStub{})

	result, err := svc.Query(context.Background(), sensorquery.Sensors{})
	is.NoErr(err)
	is.Equal(len(result.Data), 1)
	is.Equal(result.Data[0].SensorID, "sensor-1")
}
