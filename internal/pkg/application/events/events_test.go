package events

import (
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestConfig(t *testing.T) {
	is := setupTest(t)
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

func setupTest(t *testing.T) *is.I {
	is := is.New(t)

	return is
}
