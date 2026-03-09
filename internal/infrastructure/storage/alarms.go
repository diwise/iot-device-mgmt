package storage

import (
	"context"
	"fmt"
	"time"

	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5"
)

func (s *Storage) AddAlarm(ctx context.Context, deviceID string, a types.AlarmDetails) error {
	if deviceID == "" {
		return ErrNoID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	args := pgx.NamedArgs{
		"device_id":   deviceID,
		"type":        a.AlarmType,
		"description": a.Description,
		"observed_at": a.ObservedAt,
		"severity":    a.Severity,
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO device_alarms (device_id, type, description, observed_at, severity)
		VALUES (@device_id, @type, @description, @observed_at, @severity)
		ON CONFLICT (device_id, type) DO UPDATE
			SET
				description=EXCLUDED.description,
				observed_at=EXCLUDED.observed_at,
				severity=EXCLUDED.severity,
				count = device_alarms.count + 1
				`, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) GetDeviceAlarms(ctx context.Context, deviceID string) (types.Collection[types.AlarmDetails], error) {
	if deviceID == "" {
		return types.Collection[types.AlarmDetails]{}, ErrNoID
	}

	args := pgx.NamedArgs{
		"device_id": deviceID,
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[types.AlarmDetails]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT device_id, type, description, observed_at, severity
		FROM device_alarms
		WHERE device_id=@device_id
		ORDER BY observed_at ASC`, args)
	if err != nil {
		return types.Collection[types.AlarmDetails]{}, err
	}
	defer rows.Close()

	alarms := []types.AlarmDetails{}

	for rows.Next() {
		var device_id, alarmtype, description string
		var observed_at time.Time
		var severity int

		err := rows.Scan(&device_id, &alarmtype, &description, &observed_at, &severity)
		if err != nil {
			return types.Collection[types.AlarmDetails]{}, err
		}

		alarms = append(alarms, types.AlarmDetails{
			AlarmType:   alarmtype,
			Description: description,
			ObservedAt:  observed_at.UTC(),
			Severity:    severity,
		})
	}

	if err := rows.Err(); err != nil {
		return types.Collection[types.AlarmDetails]{}, err
	}

	return types.Collection[types.AlarmDetails]{
		Data:       alarms,
		Count:      uint64(len(alarms)),
		Offset:     0,
		Limit:      uint64(len(alarms)),
		TotalCount: uint64(len(alarms)),
	}, nil
}

func (s *Storage) RemoveAlarm(ctx context.Context, deviceID string, alarmType string) error {
	args := pgx.NamedArgs{
		"device_id":  deviceID,
		"alarm_type": alarmType,
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM device_alarms WHERE device_id=@device_id AND type=@alarm_type`, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) GetAlarms(ctx context.Context, conds ...conditions.ConditionFunc) (types.Collection[types.Alarms], error) {
	condition := conditions.NewCondition(conds...)

	args := NamedArgs(condition)
	offsetLimit, offset, limit := OffsetLimit(condition, 0, 5)

	sql := fmt.Sprintf(`
		SELECT a.device_id, array_agg(type) as type, MAX(severity) as severity, MAX(observed_at) as observed_at, count(*) OVER () AS count
		FROM device_alarms a
		JOIN devices d ON a.device_id = d.device_id
		%s
		GROUP BY a.device_id
		ORDER BY observed_at DESC
		%s
	`, Where(condition), offsetLimit)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[types.Alarms]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, sql, args)
	if err != nil {
		return types.Collection[types.Alarms]{}, err
	}
	defer rows.Close()

	var totalCount uint64
	alarms := []types.Alarms{}

	for rows.Next() {
		var observedAt time.Time
		var deviceID string
		var typs []string
		var severity int

		err := rows.Scan(&deviceID, &typs, &severity, &observedAt, &totalCount)
		if err != nil {
			return types.Collection[types.Alarms]{}, err
		}
		alarms = append(alarms, types.Alarms{
			DeviceID:   deviceID,
			AlarmTypes: typs,
			ObservedAt: observedAt.UTC(),
		})
	}

	if err := rows.Err(); err != nil {
		return types.Collection[types.Alarms]{}, err
	}

	return types.Collection[types.Alarms]{
		Data:       alarms,
		Count:      uint64(len(alarms)),
		Offset:     uint64(offset),
		Limit:      uint64(limit),
		TotalCount: totalCount,
	}, nil
}
