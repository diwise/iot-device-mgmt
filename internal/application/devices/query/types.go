package query

import (
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

type Filters struct {
	DeviceID       string
	SensorID       string
	Active         *bool
	Online         *bool
	Types          []string
	AllowedTenants []string
	Tenant         string
	ProfileNames   []string
	Metadata       map[string]string
	LastSeen       *time.Time
	Search         string
	Bounds         *types.Bounds
	Name           string
	Urn            string
	Export         bool
	SortBy         string
	SortDesc       bool
	Offset         *int
	Limit          *int
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
