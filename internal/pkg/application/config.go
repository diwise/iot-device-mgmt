package application

import (
	"io"

	yaml "gopkg.in/yaml.v2"
)

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
