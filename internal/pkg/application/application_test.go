package application

import (
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestConfig(t *testing.T) {
	is, _ := setupTest(t)
	config := strings.NewReader(`
notifications:
  - id: qalcosonic
    name: Qalcosonic W1 StatusCodes
    type: diwise.statusmessage
    subscribers:
    - endpoint: http://api-notification:8990
      information:
      - entities:
        - idPattern: ^urn:ngsi-ld:Device:.+
`)
	cfg, err := LoadConfiguration(config)

	is.NoErr(err)
	is.Equal(len(cfg.Notifications), 1)
	is.Equal(cfg.Notifications[0].ID, "qalcosonic")
}

func setupTest(t *testing.T) (*is.I, DeviceManagement) {
	is := is.New(t)
	log := zerolog.Logger{}
	db, _ := database.NewDatabaseConnection(database.NewSQLiteConnector(log))
	app := New(db, Config{})

	return is, app
}
