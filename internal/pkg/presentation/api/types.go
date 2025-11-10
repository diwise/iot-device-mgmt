package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

type meta struct {
	TotalRecords uint64  `json:"totalRecords"`
	Offset       *uint64 `json:"offset,omitempty"`
	Limit        *uint64 `json:"limit,omitempty"`
	Count        uint64  `json:"count"`
}

type links struct {
	Self  *string `json:"self,omitempty"`
	First *string `json:"first,omitempty"`
	Prev  *string `json:"prev,omitempty"`
	Next  *string `json:"next,omitempty"`
	Last  *string `json:"last,omitempty"`
}

type ApiResponse struct {
	Meta  *meta  `json:"meta,omitempty"`
	Data  any    `json:"data"`
	Links *links `json:"links,omitempty"`
}

func (r ApiResponse) Byte() []byte {
	b, _ := json.Marshal(r)
	return b
}

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
	Meta     *meta            `json:"meta,omitempty"`
	Links    *links           `json:"links,omitempty"`
}

func NewFeatureCollection() *GeoJSONFeatureCollection {
	fc := &GeoJSONFeatureCollection{Type: "FeatureCollection"}
	return fc
}

type GeoJSONFeature struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Geometry   any            `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

func NewFeatureCollectionWithDevices(devices []types.Device) (*GeoJSONFeatureCollection, error) {
	fc := NewFeatureCollection()

	for _, d := range devices {
		f, err := ConvertDevice(d)
		if err != nil {
			return nil, err
		}
		fc.Features = append(fc.Features, *f)
	}

	return fc, nil
}

func ConvertDevice(d types.Device) (*GeoJSONFeature, error) {
	feature := &GeoJSONFeature{
		ID:         d.DeviceID,
		Type:       "Feature",
		Geometry:   CreateGeoJSONPropertyFromWGS84(d.Location.Longitude, d.Location.Latitude),
		Properties: map[string]any{},
	}

	b, err := json.Marshal(d)
	if err != nil {
		return feature, nil
	}

	m := make(map[string]any)
	err = json.Unmarshal(b, &m)
	if err != nil {
		return feature, nil
	}

	feature.Properties = m

	return feature, nil
}

type GeoJSONGeometry interface {
	GeoPropertyType() string
	GeoPropertyValue() GeoJSONGeometry
	GetAsPoint() GeoJSONPropertyPoint
}

type PropertyImpl struct {
	Type string `json:"type"`
}

// GeoJSONProperty is used to encapsulate different GeoJSONGeometry types
type GeoJSONProperty struct {
	PropertyImpl
	Val GeoJSONGeometry `json:"value"`
}

func (gjp *GeoJSONProperty) GeoPropertyType() string {
	return gjp.Val.GeoPropertyType()
}

func (gjp *GeoJSONProperty) GeoPropertyValue() GeoJSONGeometry {
	return gjp.Val
}

func (gjp *GeoJSONProperty) GetAsPoint() GeoJSONPropertyPoint {
	return gjp.Val.GetAsPoint()
}

func (gjp *GeoJSONProperty) Type() string {
	return gjp.PropertyImpl.Type
}

func (gjp *GeoJSONProperty) Value() any {
	return gjp.GeoPropertyValue()
}

// GeoJSONPropertyPoint is used as the value object for a GeoJSONPropertyPoint
type GeoJSONPropertyPoint struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

func (gjpp *GeoJSONPropertyPoint) GeoPropertyType() string {
	return gjpp.Type
}

func (gjpp *GeoJSONPropertyPoint) GeoPropertyValue() GeoJSONGeometry {
	return gjpp
}

func (gjpp *GeoJSONPropertyPoint) GetAsPoint() GeoJSONPropertyPoint {
	// Return a copy of this point to prevent mutation
	return GeoJSONPropertyPoint{
		Type:        gjpp.Type,
		Coordinates: [2]float64{gjpp.Coordinates[0], gjpp.Coordinates[1]},
	}
}

func (gjpp GeoJSONPropertyPoint) Latitude() float64 {
	return gjpp.Coordinates[1]
}

func (gjpp GeoJSONPropertyPoint) Longitude() float64 {
	return gjpp.Coordinates[0]
}

// GeoJSONPropertyLineString is used as the value object for a GeoJSONPropertyLineString
type GeoJSONPropertyLineString struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

func (gjpls *GeoJSONPropertyLineString) GeoPropertyType() string {
	return gjpls.Type
}

func (gjpls *GeoJSONPropertyLineString) GeoPropertyValue() GeoJSONGeometry {
	return gjpls
}

func (gjpls *GeoJSONPropertyLineString) GetAsPoint() GeoJSONPropertyPoint {
	return GeoJSONPropertyPoint{
		Type:        "Point",
		Coordinates: [2]float64{gjpls.Coordinates[0][0], gjpls.Coordinates[0][1]},
	}
}

// GeoJSONPropertyMultiPolygon is used as the value object for a GeoJSONPropertyMultiPolygon
type GeoJSONPropertyMultiPolygon struct {
	Type        string          `json:"type"`
	Coordinates [][][][]float64 `json:"coordinates"`
}

func (gjpmp *GeoJSONPropertyMultiPolygon) GeoPropertyType() string {
	return gjpmp.Type
}

func (gjpmp *GeoJSONPropertyMultiPolygon) GeoPropertyValue() GeoJSONGeometry {
	return gjpmp
}

func (gjpmp *GeoJSONPropertyMultiPolygon) GetAsPoint() GeoJSONPropertyPoint {
	return GeoJSONPropertyPoint{
		Type:        "Point",
		Coordinates: [2]float64{gjpmp.Coordinates[0][0][0][0], gjpmp.Coordinates[0][0][0][1]},
	}
}

// CreateGeoJSONPropertyFromWGS84 creates a GeoJSONProperty from a WGS84 coordinate
func CreateGeoJSONPropertyFromWGS84(longitude, latitude float64) *GeoJSONProperty {
	p := &GeoJSONProperty{
		PropertyImpl: PropertyImpl{Type: "GeoProperty"},
		Val: &GeoJSONPropertyPoint{
			Type:        "Point",
			Coordinates: [2]float64{longitude, latitude},
		},
	}

	return p
}

// CreateGeoJSONPropertyFromLineString creates a GeoJSONProperty from an array of line coordinate arrays
func CreateGeoJSONPropertyFromLineString(coordinates [][]float64) *GeoJSONProperty {
	p := &GeoJSONProperty{
		PropertyImpl: PropertyImpl{Type: "GeoProperty"},
		Val: &GeoJSONPropertyLineString{
			Type:        "LineString",
			Coordinates: coordinates,
		},
	}

	return p
}

// CreateGeoJSONPropertyFromMultiPolygon creates a GeoJSONProperty from an array of polygon coordinate arrays
func CreateGeoJSONPropertyFromMultiPolygon(coordinates [][][][]float64) *GeoJSONProperty {
	p := &GeoJSONProperty{
		PropertyImpl: PropertyImpl{Type: "GeoProperty"},
		Val: &GeoJSONPropertyMultiPolygon{
			Type:        "MultiPolygon",
			Coordinates: coordinates,
		},
	}

	return p
}

func writeCsvWithDevices(w io.Writer, devices []types.Device) error {
	header := []string{"devEUI", "internalID", "lat", "lon", "where", "types", "sensorType", "name", "description", "active", "tenant", "interval", "source", "metadata"}
	rows := [][]string{header}

	meta := func(d types.Device) string {
		result := []string{}

		for _, m := range d.Metadata {
			result = append(result, fmt.Sprintf("%s=%s", m.Key, m.Value))
		}

		return strings.Join(result, ",")
	}

	lwm2mTypes := func(d types.Device) string {
		urn := []string{}

		for _, t := range d.Lwm2mTypes {
			urn = append(urn, t.Urn)
		}

		return strings.Join(urn, ",")
	}

	for _, d := range devices {
		row := []string{
			d.SensorID,
			d.DeviceID,
			fmt.Sprintf("%f", d.Location.Latitude),
			fmt.Sprintf("%f", d.Location.Longitude),
			d.Environment,
			lwm2mTypes(d),
			d.DeviceProfile.Decoder,
			d.Name,
			d.Description,
			fmt.Sprintf("%t", d.Active),
			d.Tenant,
			fmt.Sprintf("%d", d.DeviceProfile.Interval),
			d.Source,
			meta(d),
		}
		rows = append(rows, row)
	}

	for _, row := range rows {
		_, err := fmt.Fprintln(w, strings.Join(row, ";"))
		if err != nil {
			return err
		}
	}

	return nil
}
