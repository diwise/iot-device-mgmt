package types

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

type ConditionFunc func(*Condition) *Condition

func NewCondition(funcs ...ConditionFunc) *Condition {
	c := &Condition{}
	for _, f := range funcs {
		c = f(c)
	}
	return c
}

type Condition struct {
	DeviceID string
	SensorID string

	Active      *bool
	Online      *bool
	Types       []string
	Tenant      string
	Tenants     []string
	ProfileName []string
	Metadata    map[string]string

	AlarmType string

	LastSeen time.Time

	Search string

	Bounds *Box

	Name string
	Urn  string

	IncludeDeleted bool

	Export bool

	SortBy    string
	SortOrder string

	Offset *int
	Limit  *int
}

type Box struct {
	MinX float64 // west
	MaxX float64 // east
	MinY float64 // south
	MaxY float64 // north
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
			c.SortBy = "d.device_id"
		case "deveui":
			fallthrough
		case "sensor_id":
			c.SortBy = "d.sensor_id"
		case "name":
			c.SortBy = "d.name"
		case "decoder":
			fallthrough
		case "profile":
			fallthrough
		case "device_profile_id":
			fallthrough
		case "sensor_profile_id":
			c.SortBy = "sp.sensor_profile_id"
		}

		return c
	}
}

func WithSortDesc(desc bool) ConditionFunc {
	return func(c *Condition) *Condition {
		if desc {
			c.SortOrder = "DESC"
		} else {
			c.SortOrder = "ASC"
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
		c.Offset = &offset
		return c
	}
}

func WithLimit(limit int) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Limit = &limit
		return c
	}
}

func WithAlarmType(alarmtype string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.AlarmType = alarmtype
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

func WithExport() ConditionFunc {
	return func(c *Condition) *Condition {
		c.Export = true
		return c
	}
}

func WithMetadata(key, value string) ConditionFunc {
	return func(c *Condition) *Condition {
		c.Metadata = map[string]string{key: value}
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

func Parse(ctx context.Context, params map[string][]string) []ConditionFunc {
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
		case "export":
			if v[0] == "true" {
				conditions = append(conditions, WithExport())
			}
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

			if strings.HasPrefix(k, "metadata[") && strings.HasSuffix(k, "]") {
				metadataKey := strings.TrimPrefix(strings.TrimSuffix(k, "]"), "metadata[")
				conditions = append(conditions, WithMetadata(metadataKey, v[0]))
			} else {
				log.Debug("unknown query parameter", "param", k, "value", v[0])
			}
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
