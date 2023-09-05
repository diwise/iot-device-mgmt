package devicemanagement

import (
	"time"

	"gorm.io/gorm"
)

type Device struct {
	gorm.Model `json:"-"`

	SensorID string `gorm:"uniqueIndex" json:"sensorID"`
	DeviceID string `gorm:"uniqueIndex" json:"deviceID"`

	Active       bool         `json:"active"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Location     Location     `json:"location"`
	Environment  string       `json:"environment"`
	Source       string       `json:"source"`
	DeviceStatus DeviceStatus `json:"deviceStatus"`
	DeviceState  DeviceState  `json:"deviceState"`

	TenantID uint   `gorm:"foreignKey:TenantID" json:"-"`
	Tenant   Tenant `json:"tenant"`

	DeviceProfileID uint          `gorm:"foreignKey:DeviceProfileID" json:"-"`
	DeviceProfile   DeviceProfile `json:"deviceProfile"`

	Lwm2mTypes []Lwm2mType `gorm:"many2many:device_lwm2mType;" json:"types"`
	Tags       []Tag       `gorm:"many2many:device_tags;" json:"tags"`

	Alarms []Alarm `json:"-"`
}

func (d *Device) BeforeSave(tx *gorm.DB) (err error) {
	if d.TenantID == 0 {
		var t = Tenant{}
		result := tx.Where(Tenant{Name: d.Tenant.Name}).First(&t)
		if result.RowsAffected > 0 {
			d.TenantID = t.ID
			d.Tenant = t
		}
	}

	if d.DeviceProfileID == 0 {
		existing := DeviceProfile{}
		result := tx.Where(&DeviceProfile{Name: d.DeviceProfile.Name}).First(&existing)
		if result.RowsAffected > 0 {
			d.DeviceProfileID = existing.ID
			d.DeviceProfile = existing
		}
	}

	for i, l := range d.Lwm2mTypes {
		if l.ID == 0 {
			t := Lwm2mType{}
			result := tx.Where(&Lwm2mType{Urn: l.Urn}).First(&t)
			if result.RowsAffected > 0 {
				d.Lwm2mTypes[i] = t
			}
		}
	}

	for i, t := range d.Tags {
		if t.ID == 0 {
			tag := Tag{}
			result := tx.Where(&Tag{Name: t.Name}).First(&tag)
			if result.RowsAffected > 0 {
				d.Tags[i] = tag
			}
		}
	}

	return nil
}

type Location struct {
	gorm.Model `json:"-"`
	DeviceID   uint `json:"-"`

	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

type Tenant struct {
	gorm.Model `json:"-"`

	Name string `gorm:"uniqueIndex" json:"name"`
}

type DeviceProfile struct {
	gorm.Model `json:"-"`

	Name     string `gorm:"uniqueIndex" json:"name"`
	Decoder  string `json:"decoder"`
	Interval int    `json:"interval"`
}

type Lwm2mType struct {
	gorm.Model `json:"-"`

	Urn string `gorm:"uniqueIndex" json:"urn"`
}

type DeviceStatus struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	DeviceID uint `gorm:"primarykey" json:"-"`

	BatteryLevel int       `json:"batteryLevel"`
	LastObserved time.Time `json:"lastObservedAt"`
}

const (
	DeviceStateUnknown = -1
	DeviceStateOK      = 1
	DeviceStateWarning = 2
	DeviceStateError   = 3
)

type DeviceState struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	DeviceID uint `gorm:"primarykey" json:"-"`

	Online     bool      `json:"online"`
	State      int       `json:"state"`
	ObservedAt time.Time `json:"observedAt"`
}

type Tag struct {
	gorm.Model `json:"-"`

	Name string `json:"name"`
}

type Alarm struct {
	gorm.Model `json:"-"`
	DeviceID   uint `json:"-"`

	AlarmID    int       `json:"-"`
	Severity   int       `json:"-"`
	ObservedAt time.Time `json:"-"`
}

func (Alarm) TableName() string {
	return "device_alarms"
}

func (d *Device) HasActiveAlarms() (bool, int, []Alarm) {
	highestSeverityLevel := 0

	if len(d.Alarms) == 0 {
		return false, 0, nil
	}

	for _, a := range d.Alarms {
		if highestSeverityLevel < a.Severity {
			highestSeverityLevel = a.Severity
		}
	}

	return true, highestSeverityLevel, d.Alarms
}

func (d *Device) HasAlarm(id int) bool {
	for _, a := range d.Alarms {
		if a.AlarmID == id {
			return true
		}
	}

	return false
}
