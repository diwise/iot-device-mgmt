package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func isDuplicateKeyErr(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // duplicate key value violates unique constraint
	}
	return false
}

func (s *Storage) AddDevice(ctx context.Context, device types.Device) error {
	if device.DeviceID == "" {
		return ErrNoID
	}
	if device.SensorID == "" {
		return ErrNoID
	}
	if device.Tenant == "" {
		return ErrMissingTenant
	}

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
		INSERT INTO devices (device_id, sensor_id, active, data, profile,status, state, tags, location, tenant)
		VALUES (@device_id, @sensor_id, @active, @data, @profile,@status, @state, @tags, point(@lon,@lat), @tenant)
	`, args)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return ErrAlreadyExist
		}
		return err
	}

	return nil
}

func (s *Storage) AddDeviceStatus(ctx context.Context, status types.StatusMessage) error {
	args := pgx.NamedArgs{
		"time":          status.Timestamp,
		"device_id":     status.DeviceID,
		"battery_level": status.BatteryLevel,
		"rssi":          status.RSSI,
		"snr":           status.LoRaSNR,
		"fq":            status.Frequency,
		"sf":            status.SpreadingFactor,
		"dr":            status.DR,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	device, err := getDeviceTx(ctx, tx, WithDeviceID(status.DeviceID))
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO device_status (time, device_id, battery_level, rssi, snr, fq, sf, dr)
		VALUES (@time, @device_id, @battery_level, @rssi, @snr, @fq, @sf, @dr);`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_status WHERE device_id=@device_id AND time < NOW() - INTERVAL '3 weeks'`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	ds := device.DeviceStatus
	if status.BatteryLevel != nil {
		b := *status.BatteryLevel
		ds.BatteryLevel = int(b)
	}

	if status.RSSI != nil {
		ds.RSSI = status.RSSI
	}

	if status.LoRaSNR != nil {
		ds.LoRaSNR = status.LoRaSNR
	}

	if status.Frequency != nil {
		ds.Frequency = status.Frequency
	}

	if status.SpreadingFactor != nil {
		ds.SpreadingFactor = status.SpreadingFactor
	}

	if status.DR != nil {
		ds.DR = status.DR
	}

	var ts time.Time
	if status.Timestamp.IsZero() {
		ts = time.Now()
	} else {
		ts = status.Timestamp
	}

	if ts.After(device.DeviceStatus.ObservedAt) {
		ds.ObservedAt = ts
	}

	err = updateDeviceStatusTx(ctx, tx, device.DeviceID, device.Tenant, ds)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) UpdateDevice(ctx context.Context, device types.Device) error {
	if device.DeviceID == "" {
		return ErrNoID
	}
	if device.SensorID == "" {
		return ErrNoID
	}
	if device.Tenant == "" {
		return ErrMissingTenant
	}

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

func getDeviceTx(ctx context.Context, tx pgx.Tx, conditions ...ConditionFunc) (types.Device, error) {
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
	var deleted bool

	query := `
		SELECT device_id, sensor_id, active, data, profile, state, status, tags, location, tenant, deleted
		FROM devices		
	`

	if len(where) > 0 {
		query = fmt.Sprintf("%s WHERE %s ORDER BY device_id ASC, deleted_on DESC", query, where)
	}

	err := tx.QueryRow(ctx, query, args).Scan(&deviceID, &sensorID, &active, &data, &profile, &state, &status, &tags, &location, &tenant, &deleted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Device{}, ErrNoRows
		}
		return types.Device{}, err
	}

	if deleted {
		return types.Device{}, ErrDeleted
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

func (s *Storage) GetDevice(ctx context.Context, conditions ...ConditionFunc) (types.Device, error) {
	log := logging.GetFromContext(ctx)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		log.Error("failed to begin transaction", "err", err)
		tx.Rollback(ctx)
		return types.Device{}, err
	}

	d, err := getDeviceTx(ctx, tx, conditions...)
	if err != nil {
		log.Error("failed to get device", "err", err)
		tx.Rollback(ctx)
		return types.Device{}, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Error("failed to commit transaction", "err", err)
		return types.Device{}, err
	}

	return d, nil
}

func (s *Storage) QueryDevices(ctx context.Context, conditions ...ConditionFunc) (types.Collection[types.Device], error) {
	log := logging.GetFromContext(ctx)

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
		WHERE %s 		
		ORDER BY %s %s		
		%s
	`, where, condition.SortBy(), condition.SortOrder(), offsetLimit)

	log.Debug("query devices", "sql", query, "args", args)

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

func updateDeviceStatusTx(ctx context.Context, tx pgx.Tx, deviceID, tenant string, deviceStatus types.DeviceStatus) error {
	log := logging.GetFromContext(ctx)
	log.Debug("update device status", "deviceID", deviceID, "tenant", tenant, "status", deviceStatus)

	status, _ := json.Marshal(deviceStatus)

	_, err := tx.Exec(ctx, `
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

func (s *Storage) UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus types.DeviceStatus) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	err = updateDeviceStatusTx(ctx, tx, deviceID, tenant, deviceStatus)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
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
