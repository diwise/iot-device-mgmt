package storage

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ConditionFunc func(*Condition) *Condition

type Condition struct {
	DeviceID string
	SensorID string

	Active      *bool
	Online      *bool
	Types       []string
	Tenant      string
	Tenants     []string
	ProfileName []string

	Urn      []string
	LastSeen time.Time

	Search string

	Bounds *Box

	IncludeDeleted bool

	sortBy    string
	sortOrder string

	offset *int
	limit  *int
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
	if c.Tenant != "" {
		args["tenant"] = c.Tenant
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
	if len(c.Urn) > 0 {
		args["urn"] = c.Urn
	}
	if !c.LastSeen.IsZero() {
		args["last_seen"] = c.LastSeen.UTC().Format(time.RFC3339)
	}
	if c.Search != "" {
		args["search"] = c.Search
	}

	return args
}

func (c Condition) Where() string {
	where := []string{}

	if c.DeviceID != "" {
		where = append(where, "d.device_id = @device_id")
	}

	if c.SensorID != "" {
		where = append(where, "d.sensor_id = @sensor_id")
	}

	if len(c.Tenant) > 0 && len(c.Tenants) > 0 && slices.Contains(c.Tenants, c.Tenant) {
		where = append(where, "d.tenant = @tenant")
	} else if len(c.Tenants) > 0 {
		where = append(where, "d.tenant = ANY(@tenants)")
	}

	if c.Active != nil {
		where = append(where, "d.active = @active")
	}

	if c.Online != nil {
		where = append(where, "dst.online = @online")
	}

	if len(c.Types) == 1 {
		where = append(where, "dp.decoder = @profile")
	}

	if len(c.Types) > 1 {
		where = append(where, "dp.decoder = ANY(@profiles)")
	}

	if len(c.ProfileName) > 0 {
		where = append(where, "dp.name = ANY(@profile_name)")
	}

	if c.Bounds != nil {
		where = append(where, fmt.Sprintf("location <@ BOX '((%f,%f),(%f,%f))'", c.Bounds.MinX, c.Bounds.MinY, c.Bounds.MaxX, c.Bounds.MaxY))
	}

	if len(c.Urn) > 0 {
		//TODO
	}

	if c.Search != "" {
		where = append(where, "(d.device_id ILIKE @search OR d.sensor_id ILIKE @search OR d.name ILIKE @search)")
	}

	if !c.LastSeen.IsZero() {
		where = append(where, "dst.observed_at >= @last_seen")
	}

	if !c.IncludeDeleted {
		where = append(where, "d.deleted=FALSE")
	}

	if len(where) == 0 {
		return ""
	}

	if len(where) == 1 {
		return "WHERE " + where[0]
	}

	return "WHERE " + strings.Join(where, " AND ")
}

func WithUrn(urn []string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Urn = urn
		return c
	}
}

var re = regexp.MustCompile(`[^a-zA-ZåäöÅÄÖ0-9 _,;().]+|[%]`)

func WithSearch(s string) ConditionFunc {
	return func(c *Condition) *Condition {
		s = re.ReplaceAllString(s, "")
		c.Search = strings.TrimSpace(s)
		return c
	}
}

func WithProfileName(profileName []string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.ProfileName = profileName
		return c
	}
}
/*
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
*/
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
		c.sortBy = "data-->>'name'"
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

func unique(s []string) []string {
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

func WithTenant(tenant string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Tenants = append(c.Tenants, tenant)
		c.Tenants = unique(c.Tenants)
		c.Tenant = tenant
		return c
	}
}

func WithTenants(tenants []string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Tenants = unique(tenants)
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

func WithLastSeen(ts time.Time) ConditionFunc {
	return func(c *Condition) *Condition {
		c.LastSeen = ts
		return c
	}
}
