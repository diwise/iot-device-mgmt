package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/pgtype"
	"github.com/jackc/pgx/v5"
)

func (s *Storage) UpdateDevice(ctx context.Context, device types.Device) error {
	data, _ := json.Marshal(device)
	profile, _ := json.Marshal(device.DeviceProfile)
	state, _ := json.Marshal(device.DeviceState)
	status, _ := json.Marshal(device.DeviceStatus)
	tags, _ := json.Marshal(device.Tags)

	var m map[string]any
	json.Unmarshal(data, &m)

	delete(m, "deviceID")
	delete(m, "sensorID")
	delete(m, "active")
	delete(m, "deviceProfile")
	delete(m, "deviceState")
	delete(m, "deviceStatus")
	delete(m, "tenant")
	delete(m, "tags")

	data, _ = json.Marshal(m)

	args := pgx.NamedArgs{
		"device_id": device.DeviceID,
		"sensor_id": device.SensorID,
		"active":    device.Active,
		"data":      string(data),
		"profile":   string(profile),
		"state":     string(state),
		"status":    string(status),
		"lat":       device.Location.Latitude,
		"lon":       device.Location.Longitude,
		"tags":      string(tags),
		"tenant":    device.Tenant,
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET active = @active, data = @data, profile = @profile, state = @state, status = @status, tags = @tags, location = point(@lon,@lat), tenant = @tenant, modified_on = CURRENT_TIMESTAMP
		WHERE device_id = @device_id AND deleted = FALSE
	`, args)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) AddDevice(ctx context.Context, device types.Device) error {
	data, _ := json.Marshal(device)
	profile, _ := json.Marshal(device.DeviceProfile)
	state, _ := json.Marshal(device.DeviceState)
	status, _ := json.Marshal(device.DeviceStatus)
	tags, _ := json.Marshal(device.Tags)

	var m map[string]any
	json.Unmarshal(data, &m)

	delete(m, "deviceID")
	delete(m, "sensorID")
	delete(m, "active")
	delete(m, "deviceProfile")
	delete(m, "deviceState")
	delete(m, "deviceStatus")
	delete(m, "tenant")
	delete(m, "tags")

	data, _ = json.Marshal(m)

	args := pgx.NamedArgs{
		"device_id": device.DeviceID,
		"sensor_id": device.SensorID,
		"active":    device.Active,
		"data":      string(data),
		"profile":   string(profile),
		"state":     string(state),
		"status":    string(status),
		"lat":       device.Location.Latitude,
		"lon":       device.Location.Longitude,
		"tags":      string(tags),
		"tenant":    device.Tenant,
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (device_id, sensor_id, active, data, profile, state, status, tags, location, tenant)
		VALUES (@device_id, @sensor_id, @active, @data, @profile, @state, @status, @tags, point(@lon,@lat), @tenant)
	`, args)
	if err != nil {
		return err
	}

	return nil
}

type ConditionFunc func(*Condition) *Condition

func (s *Storage) GetDevice(ctx context.Context, conditions ...ConditionFunc) (types.Device, error) {
	condition := &Condition{}
	for _, f := range conditions {
		f(condition)
	}

	args := condition.NamedArgs()
	where := condition.Where()

	var deviceID, sensorID, tenant string
	var location pgtype.Point
	var active bool
	var data, profile, state, status, tags json.RawMessage

	query := fmt.Sprintf(`
		SELECT device_id, sensor_id, active, data, profile, state, status, tags, location, tenant
		FROM devices
		WHERE %s AND deleted = FALSE
	`, where)

	err := s.pool.QueryRow(ctx, query, args).Scan(&deviceID, &sensorID, &active, &data, &profile, &state, &status, &tags, &location, &tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Device{}, ErrNoRows
		}
		return types.Device{}, err
	}

	var errs []error

	var device types.Device
	errs = append(errs, json.Unmarshal(data, &device))
	errs = append(errs, json.Unmarshal(profile, &device.DeviceProfile))
	errs = append(errs, json.Unmarshal(state, &device.DeviceState))
	errs = append(errs, json.Unmarshal(status, &device.DeviceStatus))
	errs = append(errs, json.Unmarshal(tags, &device.Tags))

	device.DeviceID = deviceID
	device.SensorID = sensorID
	device.Active = active
	device.Tenant = tenant
	device.Location = types.Location{
		Latitude:  location.P.Y,
		Longitude: location.P.X,
	}

	return device, errors.Join(errs...)
}

func (s *Storage) QueryDevices(ctx context.Context, conditions ...ConditionFunc) (types.Collection[types.Device], error) {
	condition := &Condition{}
	for _, f := range conditions {
		f(condition)
	}

	args := condition.NamedArgs()
	where := condition.Where()

	var deviceID, sensorID, tenant string
	var location pgtype.Point
	var active bool
	var data, profile, state, status, tags json.RawMessage
	var count int64

	var offsetLimit string

	if condition.offset != nil {
		offsetLimit += fmt.Sprintf("OFFSET %d ", condition.Offset())
	}

	if condition.limit != nil {
		offsetLimit += fmt.Sprintf("LIMIT %d ", condition.Limit())
	}

	query := fmt.Sprintf(`
		SELECT device_id, sensor_id, active, data, profile, state, status, tags, location, tenant, count(*) OVER () AS count
		FROM devices
		WHERE %s AND deleted = FALSE		
		ORDER BY %s %s		
		%s
	`, where, condition.SortBy(), condition.SortOrder(), offsetLimit)

	rows, err := s.pool.Query(ctx, query, args)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}

	devices := make([]types.Device, 0)

	_, err = pgx.ForEachRow(rows, []any{&deviceID, &sensorID, &active, &data, &profile, &state, &status, &tags, &location, &tenant, &count}, func() error {
		var errs []error
		var device types.Device

		errs = append(errs, json.Unmarshal(data, &device))
		errs = append(errs, json.Unmarshal(profile, &device.DeviceProfile))
		errs = append(errs, json.Unmarshal(state, &device.DeviceState))
		errs = append(errs, json.Unmarshal(status, &device.DeviceStatus))
		errs = append(errs, json.Unmarshal(tags, &device.Tags))

		device.DeviceID = deviceID
		device.SensorID = sensorID
		device.Active = active
		device.Tenant = tenant
		device.Location = types.Location{
			Latitude:  location.P.Y,
			Longitude: location.P.X,
		}
		devices = append(devices, device)

		return errors.Join(errs...)
	})
	if err != nil {
		return types.Collection[types.Device]{}, err
	}

	return types.Collection[types.Device]{
		Data:       devices,
		Count:      uint64(len(devices)),
		Limit:      uint64(condition.Limit()),
		Offset:     uint64(condition.Offset()),
		TotalCount: uint64(count),
	}, nil
}

func (s *Storage) UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus types.DeviceStatus) error {
	status, _ := json.Marshal(deviceStatus)

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET status = @status
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"status":    string(status),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error {
	state, _ := json.Marshal(deviceState)

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET state = @state
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"state":     string(state),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetTags(ctx context.Context, deviceID, tenant string) ([]types.Tag, error) {
	var tags json.RawMessage

	err := s.pool.QueryRow(ctx, `
		SELECT tags
		FROM devices
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
	}).Scan(&tags)
	if err != nil {
		return nil, err
	}

	var t []types.Tag
	err = json.Unmarshal(tags, &t)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (s *Storage) UpdateTags(ctx context.Context, deviceID, tenant string, tags []types.Tag) error {
	t, _ := json.Marshal(tags)

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET tags = @tags
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"tags":      string(t),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) SetTenant(ctx context.Context, deviceID, tenant string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET tenant = @tenant
		WHERE device_id = @device_id AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) SetActive(ctx context.Context, deviceID, tenant string, active bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET active = @active
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"active":    active,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetTenants(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT tenant
		FROM devices
		WHERE deleted = FALSE
	`)
	if err != nil {
		return []string{}, err
	}

	var tenants []string
	for rows.Next() {
		var tenant string
		rows.Scan(&tenant)
		tenants = append(tenants, tenant)
	}

	return tenants, nil
}
