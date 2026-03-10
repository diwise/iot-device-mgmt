package storage

import (
	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devicemanagement/query"
	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
)

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
