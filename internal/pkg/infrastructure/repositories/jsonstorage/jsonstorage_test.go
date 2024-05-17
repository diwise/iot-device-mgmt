package jsonstore

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
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
