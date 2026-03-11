package storage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestWhere(t *testing.T) {
	is := is.New(t)

	t.Run("default includes non-deleted", func(t *testing.T) {
		where := Where(&Condition{})
		is.Equal(where, "WHERE d.deleted=FALSE")
	})

	t.Run("include deleted removes default filter", func(t *testing.T) {
		where := Where(&Condition{IncludeDeleted: true})
		is.Equal(where, "")
	})

	t.Run("all major filters", func(t *testing.T) {
		active := true
		online := true
		lastSeen := time.Date(2024, 4, 2, 10, 11, 12, 0, time.UTC)

		where := Where(&Condition{
			DeviceID:    "dev-1",
			SensorID:    "sen-1",
			Tenant:      "tenant-1",
			Active:      &active,
			Online:      &online,
			Types:       []string{"decoder-1"},
			ProfileName: []string{"profile-a", "profile-b"},
			Bounds:      &Box{MinX: 1.1, MinY: 2.2, MaxX: 3.3, MaxY: 4.4},
			Search:      "thing",
			LastSeen:    lastSeen,
			Name:        "name-1",
			Urn:         "urn:1",
			AlarmType:   "battery",
		})

		is.True(strings.Contains(where, "d.device_id = @device_id"))
		is.True(strings.Contains(where, "d.sensor_id = @sensor_id"))
		is.True(strings.Contains(where, "d.tenant = @tenant"))
		is.True(strings.Contains(where, "d.active = @active"))
		is.True(strings.Contains(where, "dst.online = @online"))
		is.True(strings.Contains(where, "sp.decoder = @profile"))
		is.True(strings.Contains(where, "sp.name = ANY(@profile_name)"))
		is.True(strings.Contains(where, "location <@ BOX '((1.100000,2.200000),(3.300000,4.400000))'"))
		is.True(strings.Contains(where, "(d.device_id ILIKE @search OR d.sensor_id ILIKE @search OR d.name ILIKE @search)"))
		is.True(strings.Contains(where, "dst.observed_at >= @last_seen"))
		is.True(strings.Contains(where, "d.deleted=FALSE"))
		is.True(strings.Contains(where, "d.name=@name"))
		is.True(strings.Contains(where, "d.urn=@urn"))
		is.True(strings.Contains(where, "a.type=@alarmtype"))
	})

	t.Run("tenants take precedence over tenant", func(t *testing.T) {
		where := Where(&Condition{Tenant: "tenant-1", Tenants: []string{"tenant-1", "tenant-2"}})
		is.True(strings.Contains(where, "d.tenant = ANY(@tenants)"))
		is.True(!strings.Contains(where, "d.tenant = @tenant"))
	})

	t.Run("online false includes null status", func(t *testing.T) {
		online := false
		where := Where(&Condition{Online: &online})
		is.True(strings.Contains(where, "(dst.online = @online OR dst.online IS NULL)"))
	})

	t.Run("multiple types use profiles arg", func(t *testing.T) {
		where := Where(&Condition{Types: []string{"a", "b"}})
		is.True(strings.Contains(where, "sp.decoder = ANY(@profiles)"))
	})

	t.Run("metadata with explicit value", func(t *testing.T) {
		where := Where(&Condition{Metadata: map[string]string{"key": "value"}})
		is.True(strings.Contains(where, "EXISTS (SELECT 1 FROM device_metadata dm WHERE dm.device_id = d.device_id AND dm.key = @meta_key_key AND dm.vs = @meta_value_key)"))
	})

	t.Run("metadata with empty value", func(t *testing.T) {
		where := Where(&Condition{Metadata: map[string]string{"key": ""}})
		is.True(strings.Contains(where, "EXISTS (SELECT 1 FROM device_metadata dm WHERE dm.device_id = d.device_id AND dm.key = @meta_key_key)"))
		is.True(!strings.Contains(where, "@meta_value_key"))
	})
}

func TestNamedArgs(t *testing.T) {
	is := is.New(t)

	t.Run("empty condition returns no args", func(t *testing.T) {
		args := NamedArgs(&Condition{})
		is.Equal(len(args), 0)
	})

	t.Run("all supported args", func(t *testing.T) {
		active := true
		online := false
		offset := 7
		limit := 15
		lastSeen := time.Date(2024, 1, 10, 9, 8, 7, 0, time.FixedZone("UTC+2", 2*3600))

		args := NamedArgs(&Condition{
			DeviceID:    "dev-1",
			SensorID:    "sen-1",
			Tenants:     []string{"t1", "t2"},
			Tenant:      "tenant-1",
			Active:      &active,
			Online:      &online,
			AlarmType:   "battery",
			Types:       []string{"decoder-1", "decoder-2"},
			ProfileName: []string{"profile-a"},
			LastSeen:    lastSeen,
			Search:      " status ",
			Offset:      &offset,
			Limit:       &limit,
			Name:        "name-1",
			Urn:         "urn:1",
			Metadata: map[string]string{
				"key": "value",
			},
		})

		is.Equal(args["device_id"], "dev-1")
		is.Equal(args["sensor_id"], "sen-1")
		is.Equal(args["tenant"], "tenant-1")
		is.Equal(args["active"], true)
		is.Equal(args["online"], false)
		is.Equal(args["alarmtype"], "battery")
		is.Equal(args["search"], "%status%")
		is.Equal(args["offset"], 7)
		is.Equal(args["limit"], 15)
		is.Equal(args["name"], "name-1")
		is.Equal(args["urn"], "urn:1")
		is.Equal(args["meta_key_key"], "key")
		is.Equal(args["meta_value_key"], "value")
		is.Equal(args["last_seen"], lastSeen.UTC().Format(time.RFC3339))

		// Stored as []string in pgx.NamedArgs for array params.
		is.Equal(fmt.Sprintf("%v", args["tenants"]), "[t1 t2]")
		is.Equal(fmt.Sprintf("%v", args["profiles"]), "[decoder-1 decoder-2]")
		is.Equal(fmt.Sprintf("%v", args["profile_name"]), "[profile-a]")
	})

	t.Run("single type uses profile key", func(t *testing.T) {
		args := NamedArgs(&Condition{Types: []string{"decoder-1"}})
		_, hasProfile := args["profile"]
		_, hasProfiles := args["profiles"]
		is.True(hasProfile)
		is.True(!hasProfiles)
	})

	t.Run("tenants key set when non-nil empty slice", func(t *testing.T) {
		args := NamedArgs(&Condition{Tenants: []string{}})
		_, hasTenants := args["tenants"]
		is.True(hasTenants)
	})

	t.Run("search term with wildcard is not wrapped", func(t *testing.T) {
		args := NamedArgs(&Condition{Search: "%abc%"})
		is.Equal(args["search"], "%abc%")
	})
}

func TestOffsetLimit(t *testing.T) {
	is := is.New(t)

	t.Run("uses built-in defaults", func(t *testing.T) {
		query, offset, limit := OffsetLimit(&Condition{})
		is.Equal(query, "OFFSET 0 LIMIT 10 ")
		is.Equal(offset, 0)
		is.Equal(limit, 10)
	})

	t.Run("uses provided fallback values", func(t *testing.T) {
		query, offset, limit := OffsetLimit(&Condition{}, 5, 20)
		is.Equal(query, "OFFSET 5 LIMIT 20 ")
		is.Equal(offset, 5)
		is.Equal(limit, 20)
	})

	t.Run("condition offset and limit override fallback", func(t *testing.T) {
		offsetValue := 3
		limitValue := 9
		query, offset, limit := OffsetLimit(&Condition{Offset: &offsetValue, Limit: &limitValue}, 5, 20)
		is.Equal(query, "OFFSET @offset LIMIT @limit ")
		is.Equal(offset, 3)
		is.Equal(limit, 9)
	})

	t.Run("offset only", func(t *testing.T) {
		offsetValue := 11
		query, offset, limit := OffsetLimit(&Condition{Offset: &offsetValue})
		is.Equal(query, "OFFSET @offset LIMIT 10 ")
		is.Equal(offset, 11)
		is.Equal(limit, 10)
	})

	t.Run("limit only", func(t *testing.T) {
		limitValue := 4
		query, offset, limit := OffsetLimit(&Condition{Limit: &limitValue})
		is.Equal(query, "OFFSET 0 LIMIT @limit ")
		is.Equal(offset, 0)
		is.Equal(limit, 4)
	})
}

func TestOrderByWithFallback(t *testing.T) {
	is := is.New(t)

	t.Run("explicit sort with explicit order", func(t *testing.T) {
		order := OrderByWithFallback(&Condition{SortBy: "d.device_id", SortOrder: "DESC "}, "ORDER BY d.sensor_id ASC")
		is.Equal(order, "ORDER BY d.device_id DESC ")
	})

	t.Run("explicit sort defaults to ascending", func(t *testing.T) {
		order := OrderByWithFallback(&Condition{SortBy: "d.device_id"}, "ORDER BY d.sensor_id ASC")
		is.Equal(order, "ORDER BY d.device_id ASC ")
	})

	t.Run("fallback used when sort missing", func(t *testing.T) {
		order := OrderByWithFallback(&Condition{}, "ORDER BY d.device_id DESC")
		is.Equal(order, "ORDER BY d.device_id DESC")
	})

	t.Run("empty when no sort and no fallback", func(t *testing.T) {
		order := OrderByWithFallback(&Condition{}, "")
		is.Equal(order, "")
	})
}
