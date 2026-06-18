package storage

import (
	"context"
	"maps"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Storage) Stale(ctx context.Context) (types.Collection[types.Device], error) {
	return s.stale(ctx, "", nil, 0, 0)
}

func (s *Storage) StaleDevices(ctx context.Context, offset, limit int, tenants []string) (types.Collection[types.Device], error) {
	return s.stale(ctx, " AND d.active=@active AND d.tenant = ANY(@tenants)", pgx.NamedArgs{"tenants": tenants, "active": true}, offset, limit)
}

func (s *Storage) stale(ctx context.Context, tenantFilter string, args pgx.NamedArgs, offset, limit int) (types.Collection[types.Device], error) {
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
			d.location,
			d.interval      		AS device_interval,
			s.sensor_profile,
			sp.sensor_profile_id,
			sp.name          		AS profile_name,
			sp.decoder,
			sp.description   		AS profile_description,
			sp.interval     		AS profile_interval,
			ls.last_observed,
			CASE WHEN d.interval = 0 THEN sp.interval ELSE d.interval END AS effective_interval_seconds,
			dst.online      AS state_online,
			dst.state       AS state_value,
			dst.observed_at AS state_observed_at
		FROM devices d
		LEFT JOIN sensors s ON s.sensor_id = d.sensor_id
		LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
		LEFT JOIN last_status ls ON ls.sensor_id = d.sensor_id
		LEFT JOIN device_state dst ON dst.device_id = d.device_id
		WHERE ls.last_observed IS NOT NULL AND ls.last_observed < NOW() - (COALESCE(NULLIF(d.interval, 0), sp.interval) * INTERVAL '1 second')` + tenantFilter + `
		ORDER BY d.device_id`

	if offset > 0 {
		sql += ` OFFSET @offset`
	}
	if limit > 0 {
		sql += ` LIMIT @limit`
	}
	sql += `;`

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}
	defer c.Release()

	log.Debug("fetch stale devices", "sql", sql, "args", args, "offset", offset, "limit", limit)

	allArgs := pgx.NamedArgs{}
	if offset > 0 {
		allArgs["offset"] = offset
	}
	if limit > 0 {
		allArgs["limit"] = limit
	}

	maps.Copy(allArgs, args)

	rows, err := c.Query(ctx, sql, allArgs)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}
	defer rows.Close()

	devices := []types.Device{}

	var (
		deviceID          string
		sensorID          *string
		active            bool
		tenant            string
		location          pgtype.Point
		deviceInterval    int
		sensorProfile     string
		sensorProfileID   string
		name              string
		decoder           string
		description       *string
		profileInterval   int
		lastObserved      *time.Time
		effectiveInterval int
		stateOnline       *bool
		stateValue        int
		stateObservedAt   *time.Time
	)

	for rows.Next() {
		err := rows.Scan(&deviceID, &sensorID, &active, &tenant, &location, &deviceInterval, &sensorProfile, &sensorProfileID, &name, &decoder, &description, &profileInterval, &lastObserved, &effectiveInterval, &stateOnline, &stateValue, &stateObservedAt)
		if err != nil {
			return types.Collection[types.Device]{}, err
		}

		device := types.Device{
			Active:   active,
			DeviceID: deviceID,
			Tenant:   tenant,
			Interval: effectiveInterval,
			SensorProfile: types.SensorProfile{
				Name:     decoder,
				Decoder:  decoder,
				Interval: profileInterval,
			},
		}

		if stateObservedAt != nil {
			device.DeviceState = types.DeviceState{
				Online:     *stateOnline,
				State:      stateValue,
				ObservedAt: *stateObservedAt,
			}
		}

		if sensorID != nil {
			device.SensorID = *sensorID
		}

		if location.Valid {
			device.Location = types.Location{
				Latitude:  location.P.Y,
				Longitude: location.P.X,
			}
		}

		if lastObserved != nil {
			device.SensorStatus = types.SensorStatus{
				ObservedAt: *lastObserved,
			}
		}

		devices = append(devices, device)
	}

	if err := rows.Err(); err != nil {
		return types.Collection[types.Device]{}, err
	}

	return types.Collection[types.Device]{
		Data:       devices,
		Count:      uint64(len(devices)),
		TotalCount: uint64(len(devices)),
		Offset:     uint64(offset),
		Limit:      uint64(limit),
	}, nil
}
