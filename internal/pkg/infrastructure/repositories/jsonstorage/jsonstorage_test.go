package jsonstore

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	types "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matryer/is"
)

type Thing struct {
	Id        string    `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

func TestStoreNewObject(t *testing.T) {
	ctx, storage := testSetup(t)

	thing := Thing{
		Id:        uuid.NewString(),
		Type:      "Thing",
		Timestamp: time.Now(),
	}

	b, _ := json.Marshal(thing)
	err := storage.Store(ctx, thing.Id, thing.Type, b, "default")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

func TestQuery(t *testing.T) {
	ctx, storage := testSetup(t)

	res, err := storage.Query(ctx, "data ->> 'type' = 'Thing'", []string{"default"}, Limit(5))
	if err != nil {
		t.FailNow()
	}

	if res.Count == 0 {
		t.Fail()
	}
}

func TestGetTenants(t *testing.T) {
	ctx, storage := testSetup(t)
	tenants := storage.GetTenants(ctx)

	if len(tenants) == 0 {
		t.Fail()
	}
}

func TestDelete(t *testing.T) {
	ctx, storage := testSetup(t)
	var (
		typeName = "Thing"
		tenants  = []string{"default"}
	)

	thing := Thing{
		Id:        uuid.NewString(),
		Type:      typeName,
		Timestamp: time.Now(),
	}

	err := Store(ctx, storage, thing, tenants[0])
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	res, err := storage.FetchType(ctx, typeName, tenants)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	countBeforeDelete := res.Count

	err = storage.Delete(ctx, thing.Id, typeName, tenants)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	res, err = storage.FetchType(ctx, typeName, tenants)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	countAfterDelete := res.Count

	if countAfterDelete == countBeforeDelete {
		t.Log("found equal number of things after delete. Should be one less")
		t.Fail()
	}
}

func TestQueryWithinBounds(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	insertTestData(ctx, t, s.db)

	bounds := types.Bounds{
		MinLat: 40.0,
		MinLon: -75.0,
		MaxLat: 41.0,
		MaxLon: -73.0,
	}

	result, err := s.QueryWithinBounds(ctx, bounds)
	is.NoErr(err)
	is.Equal(2, result.Count)

	var data []map[string]interface{}
	for _, d := range result.Data {
		var device map[string]interface{}
		err := json.Unmarshal(d, &device)
		is.NoErr(err)
		data = append(data, device)
	}

	expectedDevices := []map[string]interface{}{
		{"id": 1, "latitude": 40.712776, "longitude": -74.005974, "tenant": "default"},
		{"id": 2, "latitude": 40.730610, "longitude": -73.935242, "tenant": "default"},
	}

	is.Equal(expectedDevices, data)
}

func insertTestData(ctx context.Context, t *testing.T, db *pgxpool.Pool) {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS devices (
			id SERIAL PRIMARY KEY,
			latitude FLOAT NOT NULL,
			longitude FLOAT NOT NULL,
			tenant TEXT NOT NULL,
			data JSONB
		);
		INSERT INTO devices (latitude, longitude, tenant, data) VALUES
		(40.712776, -74.005974, 'default', '{"id": 1, "latitude": 40.712776, "longitude": -74.005974, "tenant": "default"}'),
		(40.730610, -73.935242, 'default', '{"id": 2, "latitude": 40.730610, "longitude": -73.935242, "tenant": "default"}'),
		(34.052235, -118.243683, 'default', '{"id": 3, "latitude": 34.052235, "longitude": -118.243683, "tenant": "default"}');
	`)
	if err != nil {
		t.Fatalf("Unable to insert test data: %v", err)
	}
}

func testSetup(t *testing.T) (context.Context, JsonStorage) {
	ctx := context.Background()
	r := bytes.NewBuffer([]byte(configFile))
	config := Config{
		host:     "localhost",
		user:     "postgres",
		password: "password",
		port:     "5432",
		dbname:   "postgres",
		sslmode:  "disable",
	}
	s, err := New(ctx, config, r)
	if err != nil {
		t.SkipNow()
	}
	err = s.Initialize(ctx)
	if err != nil {
		t.SkipNow()
	}
	return ctx, s
}

var configFile string = `
serviceName: testSvc
entities:
  - idPattern: ^
    type: Device
    tableName: devices
  - idPattern: ^
    type: DeviceModel
    tableName: deviceModels
  - idPattern: ^
    type: Thing
    tableName: things
`
