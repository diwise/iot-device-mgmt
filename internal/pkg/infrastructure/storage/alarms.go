package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5"
)

func (s *Storage) QueryAlarms(ctx context.Context, conditions ...ConditionFunc) (types.Collection[types.Alarm], error) {
	condition := &Condition{}
	for _, f := range conditions {
		f(condition)
	}

	if condition.sortBy == "" {
		condition.sortBy = "observed_at"
	}

	args := condition.NamedArgs()
	where := condition.Where()

	var offsetLimit string

	if condition.offset != nil {
		offsetLimit += fmt.Sprintf("OFFSET %d ", condition.Offset())
	}

	if condition.limit != nil {
		offsetLimit += fmt.Sprintf("LIMIT %d ", condition.Limit())
	}

	var alarm_id, alarm_type, description, ref_id, tenant string
	var observed_at time.Time
	var severity int
	var count int64

	query := fmt.Sprintf(`
		SELECT alarm_id, alarm_type, description, observed_at, ref_id, severity, tenant, count(*) OVER () AS count
		FROM alarms
		WHERE %s 
		ORDER BY %s %s		
		%s
	`, where, condition.SortBy(), condition.SortOrder(), offsetLimit)

	rows, err := s.pool.Query(ctx, query, args)
	if err != nil {
		return types.Collection[types.Alarm]{}, err
	}

	alarms := make([]types.Alarm, 0)

	_, err = pgx.ForEachRow(rows, []any{&alarm_id, &alarm_type, &description, &observed_at, &ref_id, &severity, &tenant, &count}, func() error {
		alarm := types.Alarm{}

		alarm.ID = alarm_id
		alarm.AlarmType = alarm_type
		alarm.Description = description
		alarm.ObservedAt = observed_at
		alarm.RefID = ref_id
		alarm.Severity = severity
		alarm.Tenant = tenant

		alarms = append(alarms, alarm)

		return nil
	})
	if err != nil {
		return types.Collection[types.Alarm]{}, err
	}

	return types.Collection[types.Alarm]{
		Data:       alarms,
		Count:      uint64(len(alarms)),
		Limit:      uint64(condition.Limit()),
		Offset:     uint64(condition.Offset()),
		TotalCount: uint64(count),
	}, nil

}

func (s *Storage) GetAlarm(ctx context.Context, conditions ...ConditionFunc) (types.Alarm, error) {
	condition := &Condition{}
	for _, f := range conditions {
		f(condition)
	}

	args := condition.NamedArgs()
	where := condition.Where()

	var alarm_id, alarm_type, description, ref_id, tenant string
	var observed_at time.Time
	var severity int

	query := fmt.Sprintf(`
		SELECT alarm_id, alarm_type, description, observed_at, ref_id, severity, tenant
		FROM alarms
		WHERE %s 
	`, where)

	err := s.pool.QueryRow(ctx, query, args).Scan(&alarm_id, &alarm_type, &description, &observed_at, &ref_id, &severity, &tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Alarm{}, ErrNoRows
		}
		return types.Alarm{}, err
	}

	var alarm types.Alarm
	alarm.ID = alarm_id
	alarm.AlarmType = alarm_type
	alarm.Description = description
	alarm.ObservedAt = observed_at
	alarm.RefID = ref_id
	alarm.Severity = severity
	alarm.Tenant = tenant

	return alarm, nil
}

func (s *Storage) AddAlarm(ctx context.Context, alarm types.Alarm) error {
	args := pgx.NamedArgs{
		"alarm_id":    alarm.ID,
		"alarm_type":  alarm.AlarmType,
		"description": alarm.Description,
		"observed_at": alarm.ObservedAt,
		"ref_id":      alarm.RefID,
		"severity":    alarm.Severity,
		"tenant":      alarm.Tenant,
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO alarms (alarm_id, alarm_type, description, observed_at, ref_id, severity, tenant)
		VALUES (@alarm_id, @alarm_type, @description, @observed_at, @ref_id, @severity, @tenant)
	`, args)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) CloseAlarm(ctx context.Context, alarmID, tenant string) error {
	args := pgx.NamedArgs{
		"alarm_id":    alarmID,
		"tenant":      tenant,
	}
	
	_, err := s.pool.Exec(ctx, `
		UPDATE alarms
		SET deleted = TRUE, deleted_on = CURRENT_TIMESTAMP
		WHERE alarm_id = @alarm_id AND tenant = @tenant AND deleted = FALSE
	`, args)
	if err != nil {
		return err
	}

	return nil
}
