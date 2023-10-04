package devicemanagement

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	dm "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	"github.com/matryer/is"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestCreateDevice(t *testing.T) {
	is := is.New(t)

	var model dm.Device

	m := messaging.MsgContextMock{}
	r := dm.DeviceRepositoryMock{
		SaveFunc: func(ctx context.Context, device *dm.Device) error {
			model = *device
			return nil
		},
	}

	d := types.Device{
		SensorID:      "sensorID",
		DeviceID:      "deviceID",
		Active:        true,
		Tenant:        types.Tenant{Name: "default"},
		Name:          "name",
		Description:   "description",
		Location:      types.Location{Latitude: 0.0, Longitude: 0.0, Altitude: 0.0},
		Lwm2mTypes:    []types.Lwm2mType{{Urn: "urn"}},
		Tags:          []types.Tag{{Name: "tag"}},
		DeviceProfile: types.DeviceProfile{Name: "profile", Decoder: "decoder"},
		DeviceStatus:  types.DeviceStatus{BatteryLevel: 100, LastObserved: time.Now()},
		DeviceState:   types.DeviceState{State: 0, ObservedAt: time.Now()},
	}

	svc := New(&r, &m)
	err := svc.CreateDevice(context.Background(), d)
	is.NoErr(err)

	is.Equal(d.DeviceID, model.DeviceID)
}

func TestLock(t *testing.T) {

	ctx := context.Background()
	log := logging.GetFromContext(ctx)

	m := &messaging.MsgContextMock{}
	conn := database.NewSQLiteConnector(ctx)
	//	conn := database.NewPostgreSQLConnector(log, database.ConnectorConfig{/
	//		Host:     "localhost",
	//		Username: "diwise",
	//		DbName:   "diwise",
	//		Password: "diwise",
	//		SslMode:  "disable",
	//	})
	r, _ := dm.NewDeviceRepository(conn)
	svc := New(r, m)

	device := types.Device{
		SensorID:      "sensorid",
		DeviceID:      "deviceid",
		Active:        true,
		Tenant:        types.Tenant{Name: "default"},
		Name:          "name",
		Description:   "description",
		Location:      types.Location{Latitude: 0.0, Longitude: 0.0, Altitude: 0.0},
		Lwm2mTypes:    []types.Lwm2mType{{Urn: "urn"}},
		Tags:          []types.Tag{{Name: "tag"}},
		DeviceProfile: types.DeviceProfile{Name: "profile", Decoder: "decoder"},
		DeviceStatus:  types.DeviceStatus{BatteryLevel: 100, LastObserved: time.Now()},
		DeviceState:   types.DeviceState{State: 0, ObservedAt: time.Now()},
	}

	err := svc.CreateDevice(context.Background(), device)
	if err != nil {
		t.Error(err)
	}

	alarmsCreatedHandler := AlarmsCreatedHandler(m, svc)
	deviceStatusHandler := DeviceStatusHandler(m, svc)

	alarm := struct {
		ID         uint      `json:"id"`
		RefID      string    `json:"refID"`
		Severity   int       `json:"severity"`
		ObservedAt time.Time `json:"observedAt"`
	}{
		ID:         1,
		RefID:      device.DeviceID,
		Severity:   3,
		ObservedAt: time.Now().Add(-5 * time.Second),
	}

	alarmCreated := struct {
		Alarm struct {
			ID         uint      `json:"id"`
			RefID      string    `json:"refID"`
			Severity   int       `json:"severity"`
			ObservedAt time.Time `json:"observedAt"`
		} `json:"alarm"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Alarm:     alarm,
		Timestamp: time.Now(),
	}

	status := struct {
		DeviceID     string   `json:"deviceID"`
		BatteryLevel int      `json:"batteryLevel"`
		Code         int      `json:"statusCode"`
		Messages     []string `json:"statusMessages,omitempty"`
		Tenant       string   `json:"tenant,omitempty"`
		Timestamp    string   `json:"timestamp"`
	}{
		DeviceID:     device.DeviceID,
		BatteryLevel: 45,
		Code:         0,
		Messages:     []string{},
		Tenant:       "default",
		Timestamp:    time.Now().Add(5 * time.Second).Format(time.RFC3339Nano),
	}

	delivery := func(v any) amqp.Delivery {
		b, _ := json.Marshal(v)
		return amqp.Delivery{Body: b}
	}

	go alarmsCreatedHandler(context.Background(), delivery(alarmCreated), log)
	go deviceStatusHandler(context.Background(), delivery(status), log)

	time.Sleep(2 * time.Second)

	fromDb, _ := svc.GetDeviceByDeviceID(context.Background(), "deviceid")
	if fromDb.DeviceStatus.BatteryLevel != 45 {
		t.Error("batteryLevel != 45")
	}
}
