package events

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"golang.org/x/sys/unix"
	yaml "gopkg.in/yaml.v2"
)

type EventSender interface {
	Send(ctx context.Context, deviceID string, message types.DeviceStatus) error
}

type eventSender struct {
	subscribers map[string][]SubscriberConfig
}

func New(cfg *Config) EventSender {
	e := &eventSender{
		subscribers: make(map[string][]SubscriberConfig),
	}

	if cfg != nil {
		for _, s := range cfg.Notifications {
			e.subscribers[s.Type] = s.Subscribers
		}
	}

	return e
}

func (e *eventSender) Send(ctx context.Context, deviceID string, message types.DeviceStatus) error {
	if s, ok := e.subscribers["diwise.statusmessage"]; !ok || len(s) == 0 {
		return nil
	}

	var err error

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		return err
	}

	event := cloudevents.NewEvent()
	if timestamp, err := time.Parse(time.RFC3339Nano, message.Timestamp); err == nil {
		event.SetID(fmt.Sprintf("%s:%d", deviceID, timestamp.Unix()))
		event.SetTime(timestamp)
	} else {
		return err
	}

	eventData := struct {
		DeviceID     string   `json:"deviceID"`
		BatteryLevel int      `json:"batteryLevel"`
		Status       int      `json:"statusCode"`
		Messages     []string `json:"statusMessages"`
		Timestamp    string   `json:"timestamp"`
	}{
		DeviceID:     deviceID,
		BatteryLevel: message.BatteryLevel,
		Status:       message.Code,
		Messages:     message.Messages,
		Timestamp:    message.Timestamp,
	}

	event.SetSource("github.com/diwise/iot-device-mgmt")
	event.SetType("diwise.statusmessage")
	err = event.SetData(cloudevents.ApplicationJSON, eventData)
	if err != nil {
		return err
	}

	logger := logging.GetFromContext(ctx)

	for _, s := range e.subscribers["diwise.statusmessage"] {
		ctxWithTarget := cloudevents.ContextWithTarget(ctx, s.Endpoint)

		result := c.Send(ctxWithTarget, event)
		if cloudevents.IsUndelivered(result) || errors.Is(result, unix.ECONNREFUSED) {
			logger.Error().Err(result).Msgf("faild to send event to %s", s.Endpoint)
			err = fmt.Errorf("%w", result)
		}
	}

	return err
}

type EntityInfo struct {
	IDPattern string `yaml:"idPattern"`
}

type RegistrationInfo struct {
	Entities []EntityInfo `yaml:"entities"`
}

type SubscriberConfig struct {
	Endpoint    string             `yaml:"endpoint"`
	Information []RegistrationInfo `yaml:"information"`
}

type Notification struct {
	ID          string             `yaml:"id"`
	Name        string             `yaml:"name"`
	Type        string             `yaml:"type"`
	Subscribers []SubscriberConfig `yaml:"subscribers"`
}

type Config struct {
	Notifications []Notification `yaml:"notifications"`
}

func LoadConfiguration(data io.Reader) (*Config, error) {
	buf, err := io.ReadAll(data)
	if err != nil {
		return nil, err
	}

	cfg := Config{}
	if err := yaml.Unmarshal(buf, &cfg); err == nil {
		return &cfg, nil
	} else {
		return nil, err
	}
}
