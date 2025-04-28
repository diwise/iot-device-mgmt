package storage

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
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

	LastSeen time.Time

	Search string

	Bounds *Box

	Name string
	Urn  string

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

func (c Condition) OrderBy() string {
	orderBy := ""

	if c.sortBy != "" {
		orderBy += fmt.Sprintf("ORDER BY %s ", c.sortBy)
		if c.sortOrder != "" {
			orderBy += c.sortOrder
		} else {
			orderBy += "ASC "
		}
	}

	return orderBy
}

func (c Condition) OffsetLimit(i ...int) (string, int, int) {
	offsetLimit := ""
	offset := 0
	limit := 0

	if len(i) > 0 {
		offset = i[0]
		if len(i) > 1 {
			limit = i[1]
		}
	}

	if c.offset != nil {
		offsetLimit += "OFFSET @offset "
		offset = *c.offset
	}
	if c.limit != nil {
		offsetLimit += "LIMIT @limit "
		limit = *c.limit
	}
	return offsetLimit, offset, limit
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
	if !c.LastSeen.IsZero() {
		args["last_seen"] = c.LastSeen.UTC().Format(time.RFC3339)
	}
	if c.Search != "" {
		args["search"] = c.Search
	}
	if c.offset != nil {
		args["offset"] = *c.offset
	}
	if c.limit != nil {
		args["limit"] = *c.limit
	}
	if c.Name != "" {
		args["name"] = c.Name
	}
	if c.Urn != "" {
		args["urn"] = c.Urn
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

	if c.Search != "" {
		where = append(where, "(d.device_id ILIKE @search OR d.sensor_id ILIKE @search OR d.name ILIKE @search)")
	}

	if !c.LastSeen.IsZero() {
		where = append(where, "dst.observed_at >= @last_seen")
	}

	if !c.IncludeDeleted {
		where = append(where, "d.deleted=FALSE")
	}

	if c.Name != "" {
		where = append(where, "d.name=@name")
	}

	if c.Urn != "" {
		where = append(where, "d.urn=@urn")
	}

	if len(where) == 0 {
		return ""
	}

	if len(where) == 1 {
		return "WHERE " + where[0]
	}

	return "WHERE " + strings.Join(where, " AND ")
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

func WithSortBy(sortBy string) ConditionFunc {
	return func(c *Condition) *Condition {

		switch strings.ToLower(sortBy) {
		case "device_id":
			c.sortBy = "d.device_id"
		case "deveui":
			fallthrough
		case "sensor_id":
			c.sortBy = "d.sensor_id"
		case "name":
			c.sortBy = "d.name"
		case "decoder":
			fallthrough
		case "profile":
			fallthrough
		case "device_profile_id":
			c.sortBy = "dp.device_profile_id"
		}

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

func WithName(n string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Name = n
		return c
	}
}

func WithUrn(urn string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Urn = urn
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

func ParseConditions(ctx context.Context, params map[string][]string) []ConditionFunc {
	log := logging.GetFromContext(ctx)

	conditions := make([]ConditionFunc, 0)

	for k, v := range params {
		switch strings.ToLower(k) {
		case "deveui":
			conditions = append(conditions, WithSensorID(v[0]))
		case "device_id":
			conditions = append(conditions, WithDeviceID(v[0]))
		case "sensor_id":
			conditions = append(conditions, WithSensorID(v[0]))
		case "type":
			conditions = append(conditions, WithTypes(v))
		case "types":
			conditions = append(conditions, WithTypes(v))
		case "active":
			active, _ := strconv.ParseBool(v[0])
			conditions = append(conditions, WithActive(active))
		case "online":
			online, _ := strconv.ParseBool(v[0])
			conditions = append(conditions, WithOnline(online))
		case "limit":
			limit, _ := strconv.Atoi(v[0])
			conditions = append(conditions, WithLimit(limit))
		case "offset":
			offset, _ := strconv.Atoi(v[0])
			conditions = append(conditions, WithOffset(offset))
		case "sortby":
			conditions = append(conditions, WithSortBy(v[0]))
		case "sortorder":
			conditions = append(conditions, WithSortDesc(strings.EqualFold(v[0], "desc")))
		case "bounds":
			coords := extractCoordsFromQuery(v[0])
			conditions = append(conditions, WithBounds(coords.MaxLat, coords.MinLat, coords.MaxLon, coords.MinLon))
		case "profilename":
			conditions = append(conditions, WithProfileName(v))
		case "search":
			conditions = append(conditions, WithSearch(v[0]))
		case "tenant":
			conditions = append(conditions, WithTenant(v[0]))
		case "name":
			conditions = append(conditions, WithName(v[0]))
		case "urn":
			conditions = append(conditions, WithUrn(v[0]))
		case "lastseen":
			log.Debug("last seen", "value", v[0])

			switch len(v[0]) {
			case len("2006-01-02T15:04"):
				t, err := time.Parse("2006-01-02T15:04", v[0])
				if err == nil {
					conditions = append(conditions, WithLastSeen(t))
				}
			case len("2006-01-02T15:04:05"):
				t, err := time.Parse("2006-01-02T15:04:05", v[0])
				if err == nil {
					conditions = append(conditions, WithLastSeen(t))
				}
			case len("2006-01-02T15:04Z"):
				t, err := time.Parse("2006-01-02T15:04Z", v[0])
				if err == nil {
					conditions = append(conditions, WithLastSeen(t))
				}
			}
		default:
			log.Debug("unknown query parameter", "param", k, "value", v[0])
		}
	}
	return conditions
}

func extractCoordsFromQuery(bounds string) types.Bounds {
	trimmed := strings.Trim(bounds, "[]")

	pairs := strings.Split(trimmed, ";")

	coords1 := strings.Split(pairs[0], ",")
	coords2 := strings.Split(pairs[1], ",")

	seLat, _ := strconv.ParseFloat(coords1[0], 64)
	nwLon, _ := strconv.ParseFloat(coords1[1], 64)
	nwLat, _ := strconv.ParseFloat(coords2[0], 64)
	seLon, _ := strconv.ParseFloat(coords2[1], 64)

	coords := types.Bounds{
		MinLat: seLat,
		MinLon: nwLon,
		MaxLat: nwLat,
		MaxLon: seLon,
	}

	return coords
}
