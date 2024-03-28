package alarms

import (
	"bytes"
	"context"
	"testing"

	"log/slog"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog/events"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/goccy/go-json"
	"github.com/matryer/is"
)

func TestBatteryLevelChangedHandler(t *testing.T) {
	is, ctx, log := testSetup(t)

	var blc events.BatteryLevelChanged
	json.Unmarshal([]byte(batteryLevelChangedJson), &blc)

	alarmList := []alarms.Alarm{}

	m := &messaging.MsgContextMock{}
	a := &AlarmServiceMock{
		GetConfigurationFunc: func() Configuration {
			return *parseConfigFile(bytes.NewBufferString(configFileJson))
		},

		AddAlarmFunc: func(ctx context.Context, alarm alarms.Alarm) error {
			alarmList = append(alarmList, alarm)
			return nil
		},
	}

	BatteryLevelChangedHandler(m, a)(ctx, blc, log)

	is.Equal(1, len(alarmList))
}

func TestFunctionUpdatedHandler_CounterOverflow(t *testing.T) {
	is, ctx, log := testSetup(t)

	var f functionUpdated
	json.Unmarshal([]byte(counterOverflow1Json), &f)

	alarmList := []alarms.Alarm{}

	m := &messaging.MsgContextMock{}
	a := &AlarmServiceMock{
		GetConfigurationFunc: func() Configuration {
			return *parseConfigFile(bytes.NewBufferString(configFileJson))
		},
		AddAlarmFunc: func(ctx context.Context, alarm alarms.Alarm) error {
			alarmList = append(alarmList, alarm)
			return nil
		},
	}

	FunctionUpdatedHandler(m, a)(ctx, f, log)

	is.Equal(1, len(alarmList))
}

func TestFunctionUpdatedHandler_CounterOverflow_Between(t *testing.T) {
	is, ctx, log := testSetup(t)

	var f functionUpdated
	json.Unmarshal([]byte(counterOverflow2Json), &f)

	alarmList := []alarms.Alarm{}

	m := &messaging.MsgContextMock{}
	a := &AlarmServiceMock{
		GetConfigurationFunc: func() Configuration {
			return *parseConfigFile(bytes.NewBufferString(configFileJson))
		},
		AddAlarmFunc: func(ctx context.Context, alarm alarms.Alarm) error {
			alarmList = append(alarmList, alarm)
			return nil
		},
	}

	FunctionUpdatedHandler(m, a)(ctx, f, log)

	is.Equal(1, len(alarmList))
}

func TestFunctionUpdatedHandler_LevelSand(t *testing.T) {
	is, ctx, log := testSetup(t)

	var f functionUpdated
	json.Unmarshal([]byte(levelSandJson), &f)

	alarmList := []alarms.Alarm{}

	m := &messaging.MsgContextMock{}
	a := &AlarmServiceMock{
		GetConfigurationFunc: func() Configuration {
			return *parseConfigFile(bytes.NewBufferString(configFileJson))
		},
		AddAlarmFunc: func(ctx context.Context, alarm alarms.Alarm) error {
			alarmList = append(alarmList, alarm)
			return nil
		},
	}

	FunctionUpdatedHandler(m, a)(ctx, f, log)

	is.Equal(1, len(alarmList))
}

func TestDeviceStatusHandler(t *testing.T) {
	is, ctx, log := testSetup(t)

	var f deviceStatus
	json.Unmarshal([]byte(uplinkFcntRetransmissionJson), &f)

	alarmList := []alarms.Alarm{}

	m := &messaging.MsgContextMock{}
	a := &AlarmServiceMock{
		GetConfigurationFunc: func() Configuration {
			return *parseConfigFile(bytes.NewBufferString(configFileJson))
		},
		AddAlarmFunc: func(ctx context.Context, alarm alarms.Alarm) error {
			alarmList = append(alarmList, alarm)
			return nil
		},
	}

	DeviceStatusHandler(m, a)(ctx, f, log)

	is.Equal(1, len(alarmList))
}

func TestParseConfigFile(t *testing.T) {
	is := is.New(t)
	config := parseConfigFile(bytes.NewBufferString(configFileJson))
	is.Equal(8, len(config.AlarmConfigurations))

	is.Equal("net:test:iot:a81757", config.AlarmConfigurations[0].ID)
	is.Equal("", config.AlarmConfigurations[6].ID)
	is.Equal("", config.AlarmConfigurations[7].ID)
}

func TestWithNoConfigFile(t *testing.T) {
	is := is.New(t)
	config := LoadConfiguration("")
	is.Equal(nil, config)

	msgCtx := messaging.MsgContextMock{}
	msgCtx.RegisterTopicMessageHandlerFunc = func(routingKey string, handler messaging.TopicMessageHandler) error {
		return nil
	}

	svc := New(&alarms.AlarmRepositoryMock{}, &msgCtx, config)
	is.True(nil != svc.GetConfiguration().AlarmConfigurations)
}

func testSetup(t *testing.T) (*is.I, context.Context, *slog.Logger) {
	is := is.New(t)
	ctx := context.Background()
	logger := logging.GetFromContext(ctx)
	return is, ctx, logger
}

const batteryLevelChangedJson = `{"deviceID":"net:test:iot:a81757","batteryLevel":10,"tenant":"default","observedAt":"2023-04-12T06:51:25.389495559Z"}`
const counterOverflow1Json string = `{"id":"a817bf9e","type":"counter","subtype":"overflow","counter":{"count":11,"state":true},"tenant":"default"}`
const counterOverflow2Json string = `{"id":"fbf9f","type":"counter","subtype":"overflow","counter":{"count":11,"state":true},"tenant":"default"}`
const levelSandJson string = `{"id":"323c6","type":"level","subtype":"sand","level":{"current":1.4,"percent":19},"tenant":"default"}`
const uplinkFcntRetransmissionJson string = `{"deviceID":"01","batteryLevel":10,"tenant":"default","statusCode":1,"statusMessages":["UPLINK_FCNT_RETRANSMISSION"],"timestamp":"2023-04-12T06:51:25.389495559Z"}`

const configFileJson string = `
deviceID;functionID;alarmName;alarmType;min;max;severity;description
;;deviceNotObserved;-;;;1;
net:test:iot:a81757;;batteryLevel;MIN;20;0;1;
;a817bf9e;counter;MAX;;10;1;
;;payload error;-;;;-1;
;fbf9f;counter;BETWEEN;1;10;1;Count ska vara mellan {MIN} och {MAX} men är {VALUE}
;323c6;level;MAX;;4;1;
;70t589;waterquality;BETWEEN;4;35;1;Temp ska vara mellan {MIN} och {MAX} men är {VALUE}
;;UPLINK_FCNT_RETRANSMISSION;-;;;1;
`
