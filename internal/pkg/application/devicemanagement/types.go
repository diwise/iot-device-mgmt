package devicemanagement

import models "github.com/diwise/iot-device-mgmt/pkg/types"

type DeviceManagementConfig struct {
	DeviceProfiles []models.DeviceProfile `yaml:"deviceprofiles"`
	Types          []models.Lwm2mType     `yaml:"types"`
}
