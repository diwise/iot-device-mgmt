package storage

import (
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

type ConditionFunc func(*Condition) *Condition

type Condition struct {
	DeviceID          string
	SensorID          string
	DeviceWithAlarmID string
	Types             []string
	Tenants           []string
	ProfileName       []string
	offset            *int
	limit             *int
	Active            *bool
	Online            *bool
	Bounds            *Box
	sortBy            string
	sortOrder         string
	AlarmID           string
	RefID             string
	IncludeDeleted    bool
}

type Box struct {
	MinX float64 // west
	MaxX float64 // east
	MinY float64 // south
	MaxY float64 // north
}

func (c Condition) NamedArgs() pgx.NamedArgs {
	args := pgx.NamedArgs{}

	if c.DeviceID != "" {
		args["device_id"] = c.DeviceID
	}
	if c.SensorID != "" {
		args["sensor_id"] = c.SensorID
	}
	if c.Tenants != nil {
		args["tenants"] = c.Tenants
	}
	if c.Active != nil {
		args["active"] = *c.Active
	}
	if len(c.Types) == 1 {
		args["profile"] = c.Types[0]
	}
	if len(c.Types) > 1 {
		args["profiles"] = c.Types
	}
	if len(c.ProfileName) > 0 {
		args["profile_name"] = c.ProfileName
	}
	if c.AlarmID != "" {
		args["alarm_id"] = c.AlarmID
	}
	if c.RefID != "" {
		args["ref_id"] = c.RefID
	}

	return args
}

func (c Condition) Where() string {
	var where string

	if c.DeviceID != "" {
		where += "AND device_id = @device_id "
	}
	if c.SensorID != "" {
		where += "AND sensor_id = @sensor_id "
	}
	if len(c.Tenants) > 0 {
		where += "AND tenant = ANY(@tenants) "
	}
	if c.Active != nil {
		where += "AND active = @active "
	}
	if c.Online != nil {
		where += fmt.Sprintf("AND state->'online' = '%t' ", *c.Online)
	}
	if len(c.Types) == 1 {
		where += "AND profile->>'decoder' = @profile "
	}
	if len(c.Types) > 1 {
		where += "AND profile->>'decoder' = ANY(@profiles) "
	}
	if len(c.ProfileName) > 0 {
		where += "AND profile->>'name' = ANY(@profile_name) "
	}
	if c.Bounds != nil {
		where += fmt.Sprintf("AND location <@ BOX '((%f,%f),(%f,%f))' ", c.Bounds.MinX, c.Bounds.MinY, c.Bounds.MaxX, c.Bounds.MaxY)
	}
	if c.AlarmID != "" {
		where += "AND alarm_id = @alarm_id "
	}
	if c.RefID != "" {
		where += "AND ref_id = @ref_id "
	}
	if c.DeviceWithAlarmID != "" {
		where += fmt.Sprintf("AND data @> '{\"alarms\": [\"%s\"]}' ", c.DeviceWithAlarmID)
	}

	where = strings.TrimPrefix(where, "AND")

	if !c.IncludeDeleted {
		if where != "" {
			where += "AND "
		}
		where += "deleted = FALSE "
	}

	return where
}

func WithProfileName(profileName []string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.ProfileName = profileName
		return c
	}
}

func WithDeviceAlarmID(alarmID string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.DeviceWithAlarmID = alarmID
		return c
	}
}

func WithAlarmID(alarmID string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.AlarmID = alarmID
		return c
	}
}

func WithRefID(refID string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.RefID = refID
		return c
	}
}

func WithSortBy(sortBy string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.sortBy = sortBy
		return c
	}
}

func WithSortDesc(desc bool) ConditionFunc {
	return func(c *Condition) *Condition {
		if desc {
			c.sortOrder = "DESC"
		} else {
			c.sortOrder = "ASC"
		}
		return c
	}
}

func WithTypes(types []string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Types = types
		return c
	}
}

func (c Condition) SortBy() string {
	if c.sortBy == "" {
		c.sortBy = "device_id"
	}
	return c.sortBy
}

func (c Condition) SortOrder() string {
	if c.sortOrder == "" {
		c.sortOrder = "ASC"
	}
	return c.sortOrder
}

func (c Condition) Offset() int {
	if c.offset == nil {
		return 0
	}
	return *c.offset
}

func (c Condition) Limit() int {
	if c.limit == nil {
		return math.MaxInt64
	}
	return *c.limit
}

func WithOffset(offset int) ConditionFunc {
	return func(c *Condition) *Condition {
		c.offset = &offset
		return c
	}
}

func WithLimit(limit int) ConditionFunc {
	return func(c *Condition) *Condition {
		c.limit = &limit
		return c
	}
}

func WithDeviceID(deviceID string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.DeviceID = deviceID
		return c
	}
}

func WithSensorID(sensorID string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.SensorID = sensorID
		return c
	}
}

func WithTenant(tenant string) ConditionFunc {
	unique := func(s []string) []string {
		keys := make(map[string]bool)
		list := []string{}
		for _, entry := range s {
			if _, value := keys[entry]; !value {
				keys[entry] = true
				list = append(list, entry)
			}
		}
		return list
	}

	return func(c *Condition) *Condition {
		c.Tenants = append(c.Tenants, tenant)
		c.Tenants = unique(c.Tenants)
		return c
	}
}

func WithTenants(tenants []string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Tenants = tenants
		return c
	}
}

func WithActive(active bool) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Active = &active
		return c
	}
}

func WithOnline(online bool) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Online = &online
		return c
	}
}

func WithBounds(north, south, east, west float64) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Bounds = &Box{MinX: west, MaxX: east, MinY: south, MaxY: north}
		return c
	}
}

func WithDeleted() ConditionFunc {
	return func(c *Condition) *Condition {
		c.IncludeDeleted = true
		return c
	}
}
