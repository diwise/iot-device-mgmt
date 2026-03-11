package storage

import (
	"fmt"
	"strings"
	"time"

	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	"github.com/diwise/iot-device-mgmt/internal/pkg/types"
	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/jackc/pgx/v5"
)

func Where(c *types.Condition) string {
	where := []string{}

	if c.DeviceID != "" {
		where = append(where, "d.device_id = @device_id")
	}

	if c.SensorID != "" {
		where = append(where, "d.sensor_id = @sensor_id")
	}

	if len(c.Tenants) > 0 {
		where = append(where, "d.tenant = ANY(@tenants)")
	} else if len(c.Tenant) > 0 {
		where = append(where, "d.tenant = @tenant")
	}

	if c.Active != nil {
		where = append(where, "d.active = @active")
	}

	if c.Online != nil {
		if !*c.Online {
			where = append(where, "(dst.online = @online OR dst.online IS NULL)")
		} else {
			where = append(where, "dst.online = @online")
		}
	}

	if len(c.Types) == 1 {
		where = append(where, "sp.decoder = @profile")
	}

	if len(c.Types) > 1 {
		where = append(where, "sp.decoder = ANY(@profiles)")
	}

	if len(c.ProfileName) > 0 {
		where = append(where, "sp.name = ANY(@profile_name)")
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

	if c.AlarmType != "" {
		where = append(where, "a.type=@alarmtype")
	}

	if len(c.Metadata) > 0 {
		for k := range c.Metadata {
			metadataWhere := fmt.Sprintf("EXISTS (SELECT 1 FROM device_metadata dm WHERE dm.device_id = d.device_id AND dm.key = @meta_key_%s", k)
			if c.Metadata[k] != "" {
				metadataWhere += fmt.Sprintf(" AND dm.vs = @meta_value_%s", k)
			}
			metadataWhere += ")"
			where = append(where, metadataWhere)
		}
	}

	if len(where) == 0 {
		return ""
	}

	if len(where) == 1 {
		return "WHERE " + where[0]
	}

	return "WHERE " + strings.Join(where, " AND ")
}

func NamedArgs(c *types.Condition) pgx.NamedArgs {
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
	if c.Online != nil {
		args["online"] = *c.Online
	}
	if c.AlarmType != "" {
		args["alarmtype"] = c.AlarmType
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
		term := c.Search

		if !strings.Contains(term, "%") {
			term = "%" + strings.TrimSpace(term) + "%"
		}

		args["search"] = term
	}
	if c.Offset != nil {
		args["offset"] = *c.Offset
	}
	if c.Limit != nil {
		args["limit"] = *c.Limit
	}
	if c.Name != "" {
		args["name"] = c.Name
	}
	if c.Urn != "" {
		args["urn"] = c.Urn
	}
	if len(c.Metadata) > 0 {
		for k, v := range c.Metadata {
			args[fmt.Sprintf("meta_key_%s", k)] = k
			args[fmt.Sprintf("meta_value_%s", k)] = v
		}
	}

	return args
}

func OffsetLimit(c *types.Condition, i ...int) (string, int, int) {
	offsetLimit := ""
	offset := 0
	limit := 10

	if len(i) > 0 {
		offset = i[0]
		if len(i) > 1 {
			limit = i[1]
		}
	}

	if c.Offset != nil {
		offsetLimit += "OFFSET @offset "
		offset = *c.Offset
	} else {
		offsetLimit += fmt.Sprintf("OFFSET %d ", offset)
	}

	if c.Limit != nil {
		offsetLimit += "LIMIT @limit "
		limit = *c.Limit
	} else {
		offsetLimit += fmt.Sprintf("LIMIT %d ", limit)
	}

	return offsetLimit, offset, limit
}

func OrderByWithFallback(c *types.Condition, fallback string) string {
	orderBy := ""

	if c.SortBy != "" {
		orderBy += fmt.Sprintf("ORDER BY %s ", c.SortBy)
		if c.SortOrder != "" {
			orderBy += c.SortOrder
		} else {
			orderBy += "ASC "
		}
	}

	if orderBy == "" && fallback != "" {
		return fallback
	}

	return orderBy
}

func deviceConditionFromQuery(filters dmquery.Filters) *conditions.Condition {
	condition := &conditions.Condition{
		DeviceID:    filters.DeviceID,
		SensorID:    filters.SensorID,
		Active:      filters.Active,
		Online:      filters.Online,
		Types:       filters.Types,
		Tenants:     filters.AllowedTenants,
		Tenant:      filters.Tenant,
		ProfileName: filters.ProfileNames,
		Metadata:    filters.Metadata,
		Search:      filters.Search,
		Name:        filters.Name,
		Urn:         filters.Urn,
		Export:      filters.Export,
		Offset:      filters.Offset,
		Limit:       filters.Limit,
	}

	if filters.Bounds != nil {
		condition.Bounds = &conditions.Box{
			MinX: filters.Bounds.MinLon,
			MaxX: filters.Bounds.MaxLon,
			MinY: filters.Bounds.MinLat,
			MaxY: filters.Bounds.MaxLat,
		}
	}

	if filters.LastSeen != nil {
		condition.LastSeen = *filters.LastSeen
	}

	if filters.SortBy != "" {
		condition = conditions.WithSortBy(filters.SortBy)(condition)
		condition = conditions.WithSortDesc(filters.SortDesc)(condition)
	}

	return condition
}

func statusConditionFromQuery(deviceID string, query dmquery.Status) *conditions.Condition {
	condition := deviceConditionFromQuery(query.Filters)
	condition.DeviceID = deviceID
	return condition
}

func measurementConditionFromQuery(deviceID string, query dmquery.Measurements) *conditions.Condition {
	condition := deviceConditionFromQuery(query.Filters)
	condition.DeviceID = deviceID
	condition.IncludeDeleted = true
	return condition
}

func alarmConditionFromQuery(query alarmquery.Alarms) *conditions.Condition {
	condition := &conditions.Condition{
		AlarmType: query.AlarmType,
		Tenants:   query.AllowedTenants,
		Offset:    query.Offset,
		Limit:     query.Limit,
	}

	if query.ActiveOnly {
		active := true
		condition.Active = &active
	}

	return condition
}
