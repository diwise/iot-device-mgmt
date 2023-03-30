package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	AlarmSeverityLow    = 1
	AlarmSeverityMedium = 2
	AlarmSeverityHigh   = 3
)

type AlarmIdentifier struct {
	DeviceID   string `json:"deviceID,omitempty"`
	FunctionID string `json:"functionID,omitempty"`
}

type Alarm struct {
	gorm.Model `json:"-"`

	RefID AlarmIdentifier `gorm:"embedded;embeddedPrefix:refID_" json:"refID"`

	Type        string    `json:"type"`
	Severity    int       `json:"severity"`
	Description string    `json:"description"`
	Active      bool      `json:"active"`
	ObservedAt  time.Time `json:"observedAt"`
}
