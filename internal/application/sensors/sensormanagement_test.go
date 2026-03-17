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
	queryFunc func(context.Context, sensorquery.Sensors) (types.Collection[types.Sensor], error)
	getFunc   func(context.Context, string) (types.Sensor, bool, error)
	getSensorProfileFunc func(context.Context, string) (types.SensorProfile, bool, error)
}

func (s readerStub) QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
	return s.queryFunc(ctx, query)
}

func (s readerStub) GetSensor(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
	return s.getFunc(ctx, sensorID)
}

func (s readerStub) GetSensorProfile(ctx context.Context, profileID string) (types.SensorProfile, bool, error) {
	return s.getSensorProfileFunc(ctx, profileID)
}

type writerStub struct {
	createFunc func(context.Context, types.Sensor) error
	updateFunc func(context.Context, types.Sensor) error
}

func (s writerStub) CreateSensor(ctx context.Context, sensor types.Sensor) error {
	return s.createFunc(ctx, sensor)
}

func (s writerStub) UpdateSensor(ctx context.Context, sensor types.Sensor) error {
	return s.updateFunc(ctx, sensor)
}

func TestSensor(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (types.Sensor, bool, error) {
			return types.Sensor{SensorID: "sensor-1"}, true, nil
		},
	}, writerStub{})

	sensor, err := svc.Sensor(context.Background(), "sensor-1")
	is.NoErr(err)
	is.Equal(sensor.SensorID, "sensor-1")
}

func TestSensorNotFound(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (types.Sensor, bool, error) {
			return types.Sensor{}, false, nil
		},
	}, writerStub{})

	_, err := svc.Sensor(context.Background(), "missing")
	is.True(errors.Is(err, ErrSensorNotFound))
}

func TestCreateSensorAlreadyExists(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (types.Sensor, bool, error) {
			return types.Sensor{SensorID: "sensor-1"}, true, nil
		},
	}, writerStub{})

	err := svc.Create(context.Background(), types.Sensor{SensorID: "sensor-1"})
	is.True(errors.Is(err, ErrSensorAlreadyExists))
}

func TestUpdateSensorNotFound(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		getFunc: func(context.Context, string) (types.Sensor, bool, error) {
			return types.Sensor{}, false, nil
		},
	}, writerStub{})

	err := svc.Update(context.Background(), types.Sensor{SensorID: "missing"})
	is.True(errors.Is(err, ErrSensorNotFound))
}

func TestQuerySensors(t *testing.T) {
	is := is.New(t)
	svc := New(readerStub{
		queryFunc: func(context.Context, sensorquery.Sensors) (types.Collection[types.Sensor], error) {
			return types.Collection[types.Sensor]{
				Data:  []types.Sensor{{SensorID: "sensor-1", SensorProfile: &types.SensorProfile{Decoder: "elsys"}}},
				Count: 1,
			}, nil
		},
		getFunc: func(context.Context, string) (types.Sensor, bool, error) {
			return types.Sensor{}, false, nil
		},
	}, writerStub{})

	result, err := svc.Query(context.Background(), sensorquery.Sensors{})
	is.NoErr(err)
	is.Equal(len(result.Data), 1)
	is.Equal(result.Data[0].SensorID, "sensor-1")
}
