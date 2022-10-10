package database

import (
	"time"

	"gorm.io/gorm"
)

type Device struct {
	gorm.Model
	DevEUI        string `gorm:"unique;column:dev_eui"`
	DeviceId      string `gorm:"unique;column:device_id;<-:create"`
	Name          string
	Description   string
	Latitude      float64
	Longitude     float64
	EnvironmentID int `gorm:"foreignKey:EnvironmentID"`
	Environment   Environment
	Types         []Lwm2mType `gorm:"foreignKey:device_id"`
	SensorType    string
	LastObserved  time.Time
	Active        bool
	TenantID      int `gorm:"foreignKey:TenantID"`
	Tenant        Tenant
}

type Lwm2mType struct {
	Type     string `gorm:"primaryKey"`
	DeviceID uint   `gorm:"primaryKey;column:device_id"`
}

type Environment struct {
	gorm.Model
	Name string `gorm:"unique"`
}

type Tenant struct {
	gorm.Model
	Name string `gorm:"unique"`
}

type Status struct {
	DeviceID     string `gorm:"primaryKey;column:device_id"`
	BatteryLevel int
	Status       int
	Messages     string
	Timestamp    string `gorm:"primaryKey"`
}
