package api

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestCreateLinks(t *testing.T) {
	requestURL, err := url.Parse("http://example.com/api/v0/devices?limit=10&offset=10")
	if err != nil {
		t.Fatalf("expected valid url, got %v", err)
	}

	offset := uint64(10)
	limit := uint64(10)
	meta := &meta{
		TotalRecords: 35,
		Offset:       &offset,
		Limit:        &limit,
		Count:        10,
	}

	links := createLinks(requestURL, meta)
	if links == nil {
		t.Fatal("expected links to be created")
	}
	if links.Self == nil || *links.Self != "http://example.com/api/v0/devices?limit=10&offset=10" {
		t.Fatalf("expected self link, got %+v", links.Self)
	}
	if links.First == nil || *links.First != "http://example.com/api/v0/devices?limit=10&offset=0" {
		t.Fatalf("expected first link, got %+v", links.First)
	}
	if links.Next == nil || *links.Next != "http://example.com/api/v0/devices?limit=10&offset=20" {
		t.Fatalf("expected next link, got %+v", links.Next)
	}
	if links.Prev == nil || *links.Prev != "http://example.com/api/v0/devices?limit=10&offset=0" {
		t.Fatalf("expected prev link, got %+v", links.Prev)
	}
	if links.Last == nil || *links.Last != "http://example.com/api/v0/devices?limit=10&offset=30" {
		t.Fatalf("expected last link, got %+v", links.Last)
	}
}

func TestCreateLinksReturnsNilWithoutRecords(t *testing.T) {
	requestURL, err := url.Parse("http://example.com/api/v0/devices")
	if err != nil {
		t.Fatalf("expected valid url, got %v", err)
	}

	offset := uint64(0)
	limit := uint64(10)
	if links := createLinks(requestURL, &meta{TotalRecords: 0, Offset: &offset, Limit: &limit}); links != nil {
		t.Fatalf("expected nil links, got %+v", links)
	}
}

func TestContentTypeHelpers(t *testing.T) {
	t.Run("multipart form data", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
		if !isMultipartFormData(req) {
			t.Fatal("expected multipart form data to be detected")
		}
	})

	t.Run("application json", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		if !isApplicationJson(req) {
			t.Fatal("expected application/json to be detected")
		}
	})

	t.Run("geo json accept", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "application/geo+json")
		if !wantsGeoJSON(req) {
			t.Fatal("expected geojson accept to be detected")
		}
	})

	t.Run("csv accept", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/csv")
		if !wantsTextCSV(req) {
			t.Fatal("expected csv accept to be detected")
		}
	})

	t.Run("negative cases", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		if isApplicationJson(req) || isMultipartFormData(req) || wantsGeoJSON(req) || wantsTextCSV(req) {
			t.Fatal("expected helpers to return false when headers are absent")
		}
	})
}

func TestSensorQueryFromValues(t *testing.T) {
	query, err := sensorQueryFromValues(url.Values{
		"limit":  {"5"},
		"offset": {"10"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if query.Limit == nil || *query.Limit != 5 {
		t.Fatalf("expected limit 5, got %v", query.Limit)
	}
	if query.Offset == nil || *query.Offset != 10 {
		t.Fatalf("expected offset 10, got %v", query.Offset)
	}
}

func TestSensorQueryFromValuesRejectsInvalidLimit(t *testing.T) {
	_, err := sensorQueryFromValues(url.Values{"limit": {"not-a-number"}})
	if err == nil {
		t.Fatal("expected error for invalid limit")
	}
}

func TestAlarmQueryFromValues(t *testing.T) {
	query, err := alarmQueryFromValues(url.Values{
		"alarmtype": {"battery_low"},
		"limit":     {"5"},
		"offset":    {"10"},
	}, []string{"tenant-a"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if query.AlarmType != "battery_low" {
		t.Fatalf("expected alarm type, got %q", query.AlarmType)
	}
	if query.Limit == nil || *query.Limit != 5 {
		t.Fatalf("expected limit 5, got %v", query.Limit)
	}
	if query.Offset == nil || *query.Offset != 10 {
		t.Fatalf("expected offset 10, got %v", query.Offset)
	}
	if len(query.AllowedTenants) != 1 || query.AllowedTenants[0] != "tenant-a" {
		t.Fatalf("expected allowed tenants, got %+v", query.AllowedTenants)
	}
	if !query.ActiveOnly {
		t.Fatal("expected activeOnly to default to true")
	}
}

func TestAlarmQueryFromValuesRejectsInvalidLimit(t *testing.T) {
	_, err := alarmQueryFromValues(url.Values{"limit": {"not-a-number"}}, []string{"tenant-a"})
	if err == nil {
		t.Fatal("expected error for invalid limit")
	}
}

func TestDeviceQueryFromValues(t *testing.T) {
	values := url.Values{
		"device_id":          {"device-1"},
		"sensor_id":          {"sensor-1"},
		"type":               {"profile-a", "profile-b"},
		"active":             {"true"},
		"online":             {"false"},
		"limit":              {"25"},
		"offset":             {"10"},
		"sortby":             {"name"},
		"sortorder":          {"desc"},
		"bounds":             {"[55.1,12.1;56.2,13.2]"},
		"profilename":        {"profile-name"},
		"search":             {"kitchen"},
		"tenant":             {"tenant-a"},
		"name":               {"sensor name"},
		"urn":                {"3303/0/5700"},
		"export":             {"true"},
		"lastseen":           {"2024-01-02T03:04:05"},
		"metadata[building]": {"alpha"},
	}

	query, err := deviceQueryFromValues(values, []string{"tenant-a", "tenant-b"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if query.DeviceID != "device-1" {
		t.Fatalf("expected device id, got %q", query.DeviceID)
	}
	if query.SensorID != "sensor-1" {
		t.Fatalf("expected sensor id, got %q", query.SensorID)
	}
	if query.Active == nil || !*query.Active {
		t.Fatalf("expected active=true, got %v", query.Active)
	}
	if query.Online == nil || *query.Online {
		t.Fatalf("expected online=false, got %v", query.Online)
	}
	if query.Limit == nil || *query.Limit != 25 {
		t.Fatalf("expected limit 25, got %v", query.Limit)
	}
	if query.Offset == nil || *query.Offset != 10 {
		t.Fatalf("expected offset 10, got %v", query.Offset)
	}
	if query.SortBy != "name" || !query.SortDesc {
		t.Fatalf("expected descending name sort, got %+v", query.Filters)
	}
	if query.Bounds == nil || query.Bounds.MinLat != 55.1 || query.Bounds.MaxLon != 13.2 {
		t.Fatalf("expected parsed bounds, got %+v", query.Bounds)
	}
	if len(query.ProfileNames) != 1 || query.ProfileNames[0] != "profile-name" {
		t.Fatalf("expected profile names, got %+v", query.ProfileNames)
	}
	if query.Search != "kitchen" || query.Tenant != "tenant-a" || query.Name != "sensor name" || query.Urn != "3303/0/5700" {
		t.Fatalf("expected scalar filters to be preserved, got %+v", query.Filters)
	}
	if !query.Export {
		t.Fatalf("expected export flag to be set")
	}
	if len(query.AllowedTenants) != 2 || query.AllowedTenants[0] != "tenant-a" {
		t.Fatalf("expected allowed tenants, got %+v", query.AllowedTenants)
	}
	if query.Metadata["building"] != "alpha" {
		t.Fatalf("expected metadata filter, got %+v", query.Metadata)
	}

	expectedLastSeen := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	if query.LastSeen == nil || !query.LastSeen.Equal(expectedLastSeen) {
		t.Fatalf("expected lastseen %v, got %v", expectedLastSeen, query.LastSeen)
	}
}

func TestDeviceQueryFromValuesRejectsInvalidLimit(t *testing.T) {
	_, err := deviceQueryFromValues(url.Values{"limit": {"not-a-number"}}, []string{"tenant-a"})
	if err == nil {
		t.Fatal("expected error for invalid limit")
	}
}

func TestDeviceStatusQueryFromValuesRejectsInvalidLastSeen(t *testing.T) {
	_, err := deviceStatusQueryFromValues(url.Values{"lastseen": {"bad-timestamp"}}, []string{"tenant-a"})
	if err == nil {
		t.Fatal("expected error for invalid lastseen")
	}
}
