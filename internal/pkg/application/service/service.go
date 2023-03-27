package service

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/models"
	m "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/models"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	t "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
)

//go:generate moq -rm -out devicemanagement_mock.go . DeviceManagement

type DeviceManagement interface {
	GetDevices(ctx context.Context, tenants ...string) ([]m.Device, error)
	GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (m.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (m.Device, error)
	// GetStatistics(ctx context.Context) (DeviceStatistics, error)
	UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus m.DeviceStatus) error
	UpdateDeviceState(ctx context.Context, deviceID string, deviceState m.DeviceState) error

	GetAlarms(ctx context.Context, onlyActive bool) ([]m.Alarm, error)
	AddAlarm(ctx context.Context, deviceID string, alarm m.Alarm) error

	CreateDevice(ctx context.Context, device types.Device) error
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error
}

type deviceManagement struct {
	deviceRepository db.DeviceRepository
	messaging        messaging.MsgContext
}

func New(d db.DeviceRepository, m messaging.MsgContext) DeviceManagement {
	dm := &deviceManagement{
		deviceRepository: d,
		messaging:        m,
	}

	m.RegisterTopicMessageHandler("device-status", DeviceStatusTopicHandler(m, dm))
	m.RegisterTopicMessageHandler("alarms.batteryLevelWarning", WatchdogBatteryLevelWarningHandler(m, dm))
	m.RegisterTopicMessageHandler("alarms.lastObservedWarning", WatchdogLastObservedWarningHandler(m, dm))

	return dm
}

func (d *deviceManagement) CreateDevice(ctx context.Context, device types.Device) error {
	dataModel, err := MapTo[models.Device](device)
	if err != nil {
		return err
	}

	err = d.deviceRepository.Save(ctx, &dataModel)
	if err != nil {
		return err
	}

	return d.messaging.PublishOnTopic(ctx, &t.DeviceCreated{
		DeviceID:  dataModel.DeviceID,
		Tenant:    dataModel.Tenant.Name,
		Timestamp: dataModel.CreatedAt,
	})
}

func (d *deviceManagement) UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error {
	return nil
}

func (d *deviceManagement) AddAlarm(ctx context.Context, deviceID string, alarm m.Alarm) error {
	err := d.deviceRepository.AddAlarm(ctx, deviceID, alarm)
	if err != nil {
		return err
	}

	return d.UpdateDeviceState(ctx, deviceID, m.DeviceState{
		State:      m.DeviceStateOK,
		ObservedAt: time.Now().UTC(),
	})
}

func (d *deviceManagement) GetAlarms(ctx context.Context, onlyActive bool) ([]m.Alarm, error) {
	return d.deviceRepository.GetAlarms(ctx, onlyActive)
}

func (d *deviceManagement) GetDevices(ctx context.Context, tenants ...string) ([]m.Device, error) {
	return d.deviceRepository.GetDevices(ctx, tenants...)
}

func (d *deviceManagement) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (m.Device, error) {
	return d.deviceRepository.GetDeviceBySensorID(ctx, sensorID, tenants...)
}

func (d *deviceManagement) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (m.Device, error) {
	return d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID, tenants...)
}

func (d *deviceManagement) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus m.DeviceStatus) error {
	err := d.deviceRepository.UpdateDeviceStatus(ctx, deviceID, deviceStatus)
	if err != nil {
		return err
	}

	device, err := d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	return d.messaging.PublishOnTopic(ctx, &t.DeviceStatusUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant.Name,
		Timestamp: deviceStatus.LastObserved,
	})
}

func (d *deviceManagement) UpdateDeviceState(ctx context.Context, deviceID string, deviceState m.DeviceState) error {
	device, err := d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	if hasAlarm, highestSeverityLevel, _ := device.HasActiveAlarms(); hasAlarm {
		switch highestSeverityLevel {
		case m.AlarmSeverityLow:
			break
		case m.AlarmSeverityMedium:
			deviceState.State = m.DeviceStateWarning
		case m.AlarmSeverityHigh:
			deviceState.State = m.DeviceStateError
		}
	}

	err = d.deviceRepository.UpdateDeviceState(ctx, deviceID, deviceState)
	if err != nil {
		return err
	}

	return d.messaging.PublishOnTopic(ctx, &t.DeviceStateUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant.Name,
		State:     deviceState.State,
		Timestamp: deviceState.ObservedAt,
	})
}

func DeviceStatusTopicHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		status := t.DeviceStatus{}

		err := json.Unmarshal(msg.Body, &status)
		if err != nil {
			logger.Error().Err(err).Msgf("failed to unmarshal message from %s", msg.RoutingKey)
			return
		}

		logger = logger.With().Str("deviceID", status.DeviceID).Logger()

		err = dm.UpdateDeviceStatus(ctx, status.DeviceID, m.DeviceStatus{
			BatteryLevel: status.BatteryLevel,
			LastObserved: status.LastObserved,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not update status on device")
			return
		}

		err = dm.UpdateDeviceState(ctx, status.DeviceID, m.DeviceState{
			State:      m.DeviceStateOK,
			ObservedAt: status.LastObserved,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not update state on device")
		}
	}
}

func WatchdogBatteryLevelWarningHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID   string    `json:"deviceID"`
			ObservedAt time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		dm.AddAlarm(ctx, message.DeviceID, m.Alarm{
			Type:        msg.RoutingKey,
			Severity:    m.AlarmSeverityLow,
			Active:      true,
			Description: "",
			ObservedAt:  message.ObservedAt,
		})
	}
}

func WatchdogLastObservedWarningHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID   string    `json:"deviceID"`
			ObservedAt time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		dm.AddAlarm(ctx, message.DeviceID, m.Alarm{
			Type:        msg.RoutingKey,
			Severity:    m.AlarmSeverityMedium,
			Active:      true,
			Description: "",
			ObservedAt:  message.ObservedAt,
		})
	}
}
