package query

import (
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

type Filters struct {
	Active         *bool
	AllowedTenants []string
	Bounds         *types.Bounds
	DeviceID       string
	LastSeen       *time.Time
	Metadata       map[string]string
	Name           string
	Online         *bool
	ProfileNames   []string
	Search         string
	SensorID       string
	Tenant         string
	Types          []string
	Urn            string
	Status         string

	Export bool

	Limit    *int
	Offset   *int
	SortBy   string
	SortDesc bool
}

type DeviceFilters struct {
	Filters
	Urns []string
}

type StatusFilters struct {
	Filters
}

type MeasurementFilters struct {
	Filters
}
