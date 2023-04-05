package devicemanagement

import (
	"time"

	"gorm.io/gorm"
)

type Device struct {
	gorm.Model `json:"-"`

	Active      bool     `json:"active"`
	SensorID    string   `gorm:"uniqueIndex" json:"sensorID"`
	DeviceID    string   `gorm:"uniqueIndex" json:"deviceID"`
	TenantID    uint     `gorm:"foreignKey:TenantID" json:"-"`
	Tenant      Tenant   `json:"tenant"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Location    Location `json:"location"`
	Environment string   `json:"environment"`

	Lwm2mTypes []Lwm2mType `gorm:"many2many:device_lwm2mType;" json:"types"`
	Tags       []Tag       `gorm:"many2many:device_tags;" json:"tags"`

	DeviceProfileID uint          `gorm:"foreignKey:DeviceProfileID" json:"-"`
	DeviceProfile   DeviceProfile `json:"deviceProfile"`

	DeviceStatus DeviceStatus `json:"deviceStatus"`
	DeviceState  DeviceState  `json:"deviceState"`
}

func (d *Device) BeforeSave(tx *gorm.DB) (err error) {
	if d.TenantID == 0 && d.Tenant.ID == 0 {
		var t = Tenant{}
		result := tx.Where(Tenant{Name: d.Tenant.Name}).First(&t)
		if result.RowsAffected > 0 {
			d.TenantID = t.ID
			d.Tenant = t
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

	if d.DeviceProfileID == 0 && d.DeviceProfile.ID == 0 {
		existing := DeviceProfile{}

		result := tx.Where(&DeviceProfile{Name: d.DeviceProfile.Name}).First(&existing)
		if result.RowsAffected > 0 {
			d.DeviceProfile = existing
			d.DeviceProfileID = existing.ID
		}
	}

	return nil
}

/*
func (d *Device) HasActiveAlarms() (bool, int, []Alarm) {
	alarms := []Alarm{}
	highestSeverityLevel := 0

	if len(d.Alarms) == 0 {
		return false, 0, nil
	}

	for _, a := range d.Alarms {
		if a.Active {
			alarms = append(alarms, a)
			if highestSeverityLevel < a.Severity {
				highestSeverityLevel = a.Severity
			}
		}
	}

	if len(alarms) == 0 {
		return false, 0, nil
	}

	return true, highestSeverityLevel, alarms
}
*/

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

type Tag struct {
	gorm.Model `json:"-"`

	Name string `json:"name"`
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
	gorm.Model `json:"-"`
	DeviceID   uint `json:"-"`

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
	gorm.Model `json:"-"`
	DeviceID   uint `json:"-"`

	Online     bool      `json:"online"`
	State      int       `json:"state"`
	ObservedAt time.Time `json:"observedAt"`
}
