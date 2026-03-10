package storage

import (
	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
)

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
