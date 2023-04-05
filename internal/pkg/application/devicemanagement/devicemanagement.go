package devicemanagement

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	r "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	t "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

//go:generate moq -rm -out devicemanagement_mock.go . DeviceManagement

type DeviceManagement interface {
	GetDevices(ctx context.Context, tenants ...string) ([]r.Device, error)
	GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (r.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (r.Device, error)

	UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus r.DeviceStatus) error
	UpdateDeviceState(ctx context.Context, deviceID string, deviceState r.DeviceState) error

	CreateDevice(ctx context.Context, device t.Device) error
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error
}

type deviceManagement struct {
	deviceRepository r.DeviceRepository
	messenger        messaging.MsgContext
}

func New(d r.DeviceRepository, m messaging.MsgContext) DeviceManagement {
	dm := &deviceManagement{
		deviceRepository: d,
		messenger:        m,
	}

	dm.messenger.RegisterTopicMessageHandler("device-status", DeviceStatusHandler(m, dm))
	dm.messenger.RegisterTopicMessageHandler("alarms.alarmCreated", AlarmsCreatedHandler(m, dm))

	return dm
}

func (d *deviceManagement) CreateDevice(ctx context.Context, device t.Device) error {
	dataModel, err := MapTo[r.Device](device)
	if err != nil {
		return err
	}

	err = d.deviceRepository.Save(ctx, &dataModel)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &t.DeviceCreated{
		DeviceID:  dataModel.DeviceID,
		Tenant:    dataModel.Tenant.Name,
		Timestamp: dataModel.CreatedAt,
	})
}

func (d *deviceManagement) UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error {
	return nil
}

func (d *deviceManagement) GetDevices(ctx context.Context, tenants ...string) ([]r.Device, error) {
	return d.deviceRepository.GetDevices(ctx, tenants...)
}

func (d *deviceManagement) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (r.Device, error) {
	return d.deviceRepository.GetDeviceBySensorID(ctx, sensorID, tenants...)
}

func (d *deviceManagement) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (r.Device, error) {
	return d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID, tenants...)
}

func (d *deviceManagement) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus r.DeviceStatus) error {
	logger := logging.GetFromContext(ctx)

	if deviceStatus.LastObserved.IsZero() {
		logger.Debug().Msgf("lastObserved is zero, set to Now")
		deviceStatus.LastObserved = time.Now().UTC()
	}

	err := d.deviceRepository.UpdateDeviceStatus(ctx, deviceID, deviceStatus)
	if err != nil {
		return err
	}

	device, err := d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &t.DeviceStatusUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant.Name,
		Timestamp: deviceStatus.LastObserved,
	})
}

func (d *deviceManagement) UpdateDeviceState(ctx context.Context, deviceID string, deviceState r.DeviceState) error {
	logger := logging.GetFromContext(ctx)

	device, err := d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	if deviceState.ObservedAt.IsZero() {
		logger.Debug().Msgf("observedAt is zero, set to Now")
		deviceState.ObservedAt = time.Now().UTC()
	}

	err = d.deviceRepository.UpdateDeviceState(ctx, deviceID, deviceState)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &t.DeviceStateUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant.Name,
		State:     deviceState.State,
		Timestamp: deviceState.ObservedAt,
	})
}

func DeviceStatusHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		status := struct {
			DeviceID     string   `json:"deviceID"`
			BatteryLevel int      `json:"batteryLevel"`
			Code         int      `json:"statusCode"`
			Messages     []string `json:"statusMessages,omitempty"`
			Tenant       string   `json:"tenant,omitempty"`
			Timestamp    string   `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &status)
		if err != nil {
			logger.Error().Err(err).Msgf("failed to unmarshal message from %s", msg.RoutingKey)
			return
		}

		logger = logger.With().Str("deviceID", status.DeviceID).Logger()

		lastObserved, err := time.Parse(time.RFC3339Nano, status.Timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("device-status contains no valid timestamp")
			lastObserved = time.Now().UTC()
		}

		err = dm.UpdateDeviceStatus(ctx, status.DeviceID, r.DeviceStatus{
			BatteryLevel: status.BatteryLevel,
			LastObserved: lastObserved,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not update status on device")
			return
		}

		err = dm.UpdateDeviceState(ctx, status.DeviceID, r.DeviceState{
			Online:     true,
			State:      r.DeviceStateOK,
			ObservedAt: lastObserved,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not update state on device")
			return
		}

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}

func AlarmsCreatedHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			Alarm struct {
				RefID struct {
					DeviceID string `json:"deviceID,omitempty"`
				} `json:"refID"`
				Type        string    `json:"type"`
				Severity    int       `json:"severity"`
				Description string    `json:"description"`
				Active      bool      `json:"active"`
				ObservedAt  time.Time `json:"observedAt"`
			} `json:"alarm"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		if len(message.Alarm.RefID.DeviceID) == 0 {
			return
		}

		deviceID := message.Alarm.RefID.DeviceID

		logger = logger.With().Str("deviceID", deviceID).Logger()

		d, err := dm.GetDeviceByDeviceID(ctx, deviceID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to retrieve device")
			return
		}

		state := r.DeviceStateUnknown
		switch message.Alarm.Severity {
		case 1:
			state = r.DeviceStateOK
		case 2:
			state = r.DeviceStateWarning
		case 3:
			state = r.DeviceStateError
		default:
			state = r.DeviceStateOK
		}

		dm.UpdateDeviceState(ctx, deviceID, r.DeviceState{
			Online:     d.DeviceState.Online,
			State:      state,
			ObservedAt: message.Timestamp,
		})

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}
