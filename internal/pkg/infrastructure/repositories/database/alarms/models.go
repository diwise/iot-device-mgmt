package alarms

import (
	"time"

	"gorm.io/gorm"
)

const (
	AlarmSeverityLow    = 1
	AlarmSeverityMedium = 2
	AlarmSeverityHigh   = 3
)

type Alarm struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	RefID string `json:"refID"`

	Type        string    `json:"type"`
	Severity    int       `json:"severity"`
	Description string    `json:"description"`	
	Tenant      string    `json:"tenant"`
	ObservedAt  time.Time `json:"observedAt"`
}
