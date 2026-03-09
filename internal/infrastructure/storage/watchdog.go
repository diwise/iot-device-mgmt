package storage

import (
	"context"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func (s *Storage) GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error) {
	sql := `
		WITH last_status AS (
			SELECT sensor_id, MAX(observed_at) AS last_observed
			FROM sensor_status
			GROUP BY sensor_id
		)

		SELECT
			d.device_id,
			d.sensor_id,
			d.active,
			d.tenant,
			s.sensor_profile,
			d.interval      AS device_interval,
			sp.interval     AS profile_interval,
			ls.last_observed,
			CASE WHEN d.interval = 0 THEN sp.interval ELSE d.interval END AS effective_interval_seconds
		FROM devices d
			LEFT JOIN sensors s ON s.sensor_id = d.sensor_id
			LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
			LEFT JOIN last_status ls ON ls.sensor_id = d.sensor_id
		WHERE ls.last_observed IS NOT NULL AND ls.last_observed < NOW() - (COALESCE(NULLIF(d.interval, 0), sp.interval) * INTERVAL '1 second');
	`

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, sql)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}
	defer rows.Close()

	devices := []types.Device{}

	var deviceID, tenant, profile string
	var device_interval, profile_interval, effective_interval int
	var sensorID *string
	var active bool
	var lastObserved *time.Time

	for rows.Next() {
		err := rows.Scan(&deviceID, &sensorID, &active, &tenant, &profile, &device_interval, &profile_interval, &lastObserved, &effective_interval)
		if err != nil {
			return types.Collection[types.Device]{}, err
		}

		_a := active
		_sid := ""
		if sensorID != nil {
			_sid = *sensorID
		}
		_did := deviceID
		_tid := tenant
		_ei := effective_interval
		_l := lastObserved

		d := types.Device{
			Active:   _a,
			SensorID: _sid,
			DeviceID: _did,
			Tenant:   _tid,
			Interval: _ei,
			DeviceState: types.DeviceState{
				ObservedAt: time.Time{},
			},
		}

		if _l != nil {
			d.DeviceState.ObservedAt = *_l
		}

		devices = append(devices, d)
	}

	if err := rows.Err(); err != nil {
		return types.Collection[types.Device]{}, err
	}

	return types.Collection[types.Device]{
		Data:       devices,
		Count:      uint64(len(devices)),
		TotalCount: uint64(len(devices)),
		Offset:     0,
		Limit:      uint64(len(devices)),
	}, nil
}
