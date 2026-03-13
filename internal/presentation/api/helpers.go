package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func createLinks(u *url.URL, m *meta) *links {
	if m == nil || m.TotalRecords == 0 {
		return nil
	}

	query := u.Query()

	newURL := func(offset uint64) *string {
		query.Set("offset", strconv.Itoa(int(offset)))
		u.RawQuery = query.Encode()
		urlValue := u.String()
		return &urlValue
	}

	first := uint64(0)
	last := ((m.TotalRecords - 1) / *m.Limit) * *m.Limit
	next := *m.Offset + *m.Limit
	prev := int64(*m.Offset) - int64(*m.Limit)

	links := &links{
		Self:  newURL(*m.Offset),
		First: newURL(first),
		Last:  newURL(last),
	}

	if next < m.TotalRecords {
		links.Next = newURL(next)
	}

	if prev >= 0 {
		links.Prev = newURL(uint64(prev))
	}

	return links
}

func isMultipartFormData(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "multipart/form-data")
}

func isApplicationJson(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

func wantsGeoJSON(r *http.Request) bool {
	contentType := r.Header.Get("Accept")
	return strings.Contains(contentType, "application/geo+json")
}

func wantsTextCSV(r *http.Request) bool {
	contentType := r.Header.Get("Accept")
	return strings.Contains(contentType, "text/csv")
}

func sensorQueryFromValues(values url.Values) (sensorquery.Sensors, error) {
	query := sensorquery.Sensors{}

	for key, value := range values {
		if len(value) == 0 {
			continue
		}

		switch strings.ToLower(key) {
		case "limit":
			parsed, err := strconv.Atoi(value[0])
			if err != nil {
				return sensorquery.Sensors{}, fmt.Errorf("invalid limit value: %w", err)
			}
			query.Limit = &parsed
		case "offset":
			parsed, err := strconv.Atoi(value[0])
			if err != nil {
				return sensorquery.Sensors{}, fmt.Errorf("invalid offset value: %w", err)
			}
			query.Offset = &parsed
		case "assigned":
			parsed, err := strconv.ParseBool(value[0])
			if err != nil {
				return sensorquery.Sensors{}, fmt.Errorf("invalid assigned value: %w", err)
			}
			query.Assigned = &parsed
		case "hasprofile":
			parsed, err := strconv.ParseBool(value[0])
			if err != nil {
				return sensorquery.Sensors{}, fmt.Errorf("invalid hasProfile value: %w", err)
			}
			query.HasProfile = &parsed
		case "profilename", "sensortype":
			query.ProfileName = value[0]
		case "type", "types":
			query.Types = append([]string(nil), value...)
		}
	}

	return query, nil
}

func alarmQueryFromValues(values url.Values, allowedTenants []string) (alarmquery.Alarms, error) {
	query := alarmquery.Alarms{
		AllowedTenants: allowedTenants,
		ActiveOnly:     true,
	}

	for key, value := range values {
		if len(value) == 0 {
			continue
		}

		switch strings.ToLower(key) {
		case "limit":
			parsed, err := strconv.Atoi(value[0])
			if err != nil {
				return alarmquery.Alarms{}, fmt.Errorf("invalid limit value: %w", err)
			}
			query.Limit = &parsed
		case "offset":
			parsed, err := strconv.Atoi(value[0])
			if err != nil {
				return alarmquery.Alarms{}, fmt.Errorf("invalid offset value: %w", err)
			}
			query.Offset = &parsed
		case "alarmtype":
			query.AlarmType = value[0]
		}
	}

	return query, nil
}

func deviceQueryFromValues(values url.Values, allowedTenants []string) (dmquery.Devices, error) {
	filters, err := filtersFromValues(values, allowedTenants)
	if err != nil {
		return dmquery.Devices{}, err
	}

	return dmquery.Devices{Filters: filters}, nil
}

func deviceStatusQueryFromValues(values url.Values, allowedTenants []string) (dmquery.Status, error) {
	filters, err := filtersFromValues(values, allowedTenants)
	if err != nil {
		return dmquery.Status{}, err
	}

	return dmquery.Status{Filters: filters}, nil
}

func deviceMeasurementsQueryFromValues(values url.Values, allowedTenants []string) (dmquery.Measurements, error) {
	filters, err := filtersFromValues(values, allowedTenants)
	if err != nil {
		return dmquery.Measurements{}, err
	}

	return dmquery.Measurements{Filters: filters}, nil
}

func filtersFromValues(values url.Values, allowedTenants []string) (dmquery.Filters, error) {
	filters := dmquery.Filters{AllowedTenants: allowedTenants}
	metadata := map[string]string{}

	for key, value := range values {
		if len(value) == 0 {
			continue
		}

		switch strings.ToLower(key) {
		case "deveui", "sensor_id":
			filters.SensorID = value[0]
		case "device_id":
			filters.DeviceID = value[0]
		case "type", "types":
			filters.Types = append([]string(nil), value...)
		case "active":
			parsed, err := strconv.ParseBool(value[0])
			if err != nil {
				return dmquery.Filters{}, fmt.Errorf("invalid active value: %w", err)
			}
			filters.Active = &parsed
		case "online":
			parsed, err := strconv.ParseBool(value[0])
			if err != nil {
				return dmquery.Filters{}, fmt.Errorf("invalid online value: %w", err)
			}
			filters.Online = &parsed
		case "limit":
			parsed, err := strconv.Atoi(value[0])
			if err != nil {
				return dmquery.Filters{}, fmt.Errorf("invalid limit value: %w", err)
			}
			filters.Limit = &parsed
		case "offset":
			parsed, err := strconv.Atoi(value[0])
			if err != nil {
				return dmquery.Filters{}, fmt.Errorf("invalid offset value: %w", err)
			}
			filters.Offset = &parsed
		case "sortby":
			filters.SortBy = value[0]
		case "sortorder":
			filters.SortDesc = strings.EqualFold(value[0], "desc")
		case "bounds":
			bounds, err := boundsFromValue(value[0])
			if err != nil {
				return dmquery.Filters{}, err
			}
			filters.Bounds = bounds
		case "profilename":
			filters.ProfileNames = append([]string(nil), value...)
		case "search":
			filters.Search = value[0]
		case "tenant":
			filters.Tenant = value[0]
		case "name":
			filters.Name = value[0]
		case "urn":
			filters.Urn = value[0]
		case "export":
			filters.Export = strings.EqualFold(value[0], "true")
		case "lastseen":
			parsed, err := parseLastSeen(value[0])
			if err != nil {
				return dmquery.Filters{}, err
			}
			filters.LastSeen = parsed
		default:
			if strings.HasPrefix(key, "metadata[") && strings.HasSuffix(key, "]") {
				metadataKey := strings.TrimPrefix(strings.TrimSuffix(key, "]"), "metadata[")
				metadata[metadataKey] = value[0]
			}
		}
	}

	if len(metadata) > 0 {
		filters.Metadata = metadata
	}

	return filters, nil
}

func boundsFromValue(value string) (*types.Bounds, error) {
	trimmed := strings.Trim(value, "[]")
	pairs := strings.Split(trimmed, ";")
	if len(pairs) != 2 {
		return nil, fmt.Errorf("invalid bounds value")
	}

	coords1 := strings.Split(pairs[0], ",")
	coords2 := strings.Split(pairs[1], ",")
	if len(coords1) != 2 || len(coords2) != 2 {
		return nil, fmt.Errorf("invalid bounds value")
	}

	minLat, err := strconv.ParseFloat(coords1[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bounds value: %w", err)
	}
	minLon, err := strconv.ParseFloat(coords1[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bounds value: %w", err)
	}
	maxLat, err := strconv.ParseFloat(coords2[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bounds value: %w", err)
	}
	maxLon, err := strconv.ParseFloat(coords2[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bounds value: %w", err)
	}

	return &types.Bounds{MinLat: minLat, MinLon: minLon, MaxLat: maxLat, MaxLon: maxLon}, nil
}

func parseLastSeen(value string) (*time.Time, error) {
	layouts := []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02T15:04Z"}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("invalid lastseen value")
}
