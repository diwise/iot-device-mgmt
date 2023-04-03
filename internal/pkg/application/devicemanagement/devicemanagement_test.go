package devicemanagement

import (
	"context"
	"testing"
	"time"
	
	dm "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/matryer/is"
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
		Alarms:        []types.Alarm{{Type: "type", Severity: 0, Description: "description", Active: true, ObservedAt: time.Now()}},
	}

	svc := New(&r, &m)
	err := svc.CreateDevice(context.Background(), d)
	is.NoErr(err)

	is.Equal(d.DeviceID, model.DeviceID)
}
