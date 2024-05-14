package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	repository "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api/auth"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/matryer/is"
	"gopkg.in/yaml.v2"
)

func TestCreateDeviceHandler(t *testing.T) {
	is := is.New(t)

	filePath := "devices.csv"
	fieldName := "fileupload"
	body := new(bytes.Buffer)

	deviceMgmtRepoMock := &repository.DeviceRepositoryMock{
		SaveFunc: func(ctx context.Context, device types.Device) error {
			return nil
		},
		GetByDeviceIDFunc: func(ctx context.Context, deviceID string, tenants []string) (types.Device, error) {
			return types.Device{}, fmt.Errorf("device not found")
		},
	}

	msgCtx := messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
		PublishOnTopicFunc: func(ctx context.Context, message messaging.TopicMessage) error {
			return nil
		},
	}

	cfg := &devicemanagement.DeviceManagementConfig{}
	is.NoErr(yaml.Unmarshal([]byte(configYaml), cfg))

	deviceMgmt := devicemanagement.New(deviceMgmtRepoMock, &msgCtx, cfg)

	part := multipart.NewWriter(body)

	w, err := part.CreateFormFile(fieldName, filePath)
	is.NoErr(err)

	_, err = io.Copy(w, strings.NewReader(csvMock))
	is.NoErr(err)

	part.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v0/devices", body)
	ctx := auth.WithAllowedTenants(req.Context(), []string{"default", "_default"})
	req = req.WithContext(ctx)

	req.Header.Add("Content-Type", part.FormDataContentType())
	res := httptest.NewRecorder()

	createDeviceHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), deviceMgmt).ServeHTTP(res, req)

	is.Equal(2, len(deviceMgmtRepoMock.SaveCalls()))
}

const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;_default;60;source
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;_default;60;source
1234;intern-1234;0.0;0.0;air;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3304;enviot;name-1234;desc-1234;true;_test;60;källa
5678;intern-5678;0.0;0.0;soil;urn:oma:lwm2m:ext:3303;enviot;name-5678;desc-5678;true;_test;60; 
`

const configYaml string = `
deviceprofiles:
  - name: qalcosonic
    decoder: qalcosonic
    interval: 3600
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3424
      - urn:oma:lwm2m:ext:3303
  - name: axsensor
    decoder: axsensor
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3330
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3327
      - urn:oma:lwm2m:ext:3303
types:
  - urn : urn:oma:lwm2m:ext:3
    name: Device 
  - urn: urn:oma:lwm2m:ext:3303
    name: Temperature
`
