package database

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

type Datastore interface {
	GetDeviceFromDevEUI(eui string) (Device, error)
}

type db struct {
	log     zerolog.Logger
	devices [][]string
}

func SetUpNewDatabase(log zerolog.Logger, devicesFile io.Reader) (Datastore, error) {
	r := csv.NewReader(devicesFile)
	r.Comma = ';'

	knownDevices, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	return &db{
		log:     log,
		devices: knownDevices,
	}, nil
}

func (db *db) GetDeviceFromDevEUI(eui string) (Device, error) {
	for _, d := range db.devices {
		if eui == d[0] {
			lat, err := strconv.ParseFloat(d[2], 64)
			if err != nil {
				return nil, err
			}
			lon, err := strconv.ParseFloat(d[3], 64)
			if err != nil {
				return nil, err
			}

			types := strings.Split(d[5], ",")

			dev := device{
				Identity:  d[1],
				Latitude:  lat,
				Longitude: lon,
				Where:     d[4],
				Types:     types,
			}

			return dev, nil
		}
	}

	return nil, fmt.Errorf("no matching devices found with devEUI %s", eui)
}

type Device interface {
	ID() string
}

type device struct {
	Identity  string   `json:"id"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Where     string   `json:"where"`
	Types     []string `json:"types"`
}

func (d device) ID() string {
	return d.Identity
}
