package application

import (
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"time"
)

type Watchdog interface {
	Start()
}

type watchdogImpl struct {
	db database.Datastore
}

func New(db data) Watchdog {
	w := &watchdogImpl{
		db:
	}
}

func (w *watchdogImpl) Start() {
	go backgroundWorker()
}

func backgroundWorker() {
	for {
		time.Sleep(time.Second * 60)
	}
}
