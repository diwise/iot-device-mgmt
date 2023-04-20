// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package alarms

import (
	"context"
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	"sync"
)

// Ensure, that AlarmServiceMock does implement AlarmService.
// If this is not the case, regenerate this file with moq.
var _ AlarmService = &AlarmServiceMock{}

// AlarmServiceMock is a mock implementation of AlarmService.
//
//	func TestSomethingThatUsesAlarmService(t *testing.T) {
//
//		// make and configure a mocked AlarmService
//		mockedAlarmService := &AlarmServiceMock{
//			AddAlarmFunc: func(ctx context.Context, alarm db.Alarm) error {
//				panic("mock out the AddAlarm method")
//			},
//			CloseAlarmFunc: func(ctx context.Context, alarmID int) error {
//				panic("mock out the CloseAlarm method")
//			},
//			GetAlarmsFunc: func(ctx context.Context, tenants ...string) ([]db.Alarm, error) {
//				panic("mock out the GetAlarms method")
//			},
//			GetAlarmsByIDFunc: func(ctx context.Context, id int) (db.Alarm, error) {
//				panic("mock out the GetAlarmsByID method")
//			},
//			GetAlarmsByRefIDFunc: func(ctx context.Context, refID string, tenants ...string) ([]db.Alarm, error) {
//				panic("mock out the GetAlarmsByRefID method")
//			},
//			GetConfigurationFunc: func() Configuration {
//				panic("mock out the GetConfiguration method")
//			},
//			StartFunc: func()  {
//				panic("mock out the Start method")
//			},
//			StopFunc: func()  {
//				panic("mock out the Stop method")
//			},
//		}
//
//		// use mockedAlarmService in code that requires AlarmService
//		// and then make assertions.
//
//	}
type AlarmServiceMock struct {
	// AddAlarmFunc mocks the AddAlarm method.
	AddAlarmFunc func(ctx context.Context, alarm db.Alarm) error

	// CloseAlarmFunc mocks the CloseAlarm method.
	CloseAlarmFunc func(ctx context.Context, alarmID int) error

	// GetAlarmsFunc mocks the GetAlarms method.
	GetAlarmsFunc func(ctx context.Context, tenants ...string) ([]db.Alarm, error)

	// GetAlarmsByIDFunc mocks the GetAlarmsByID method.
	GetAlarmsByIDFunc func(ctx context.Context, id int) (db.Alarm, error)

	// GetAlarmsByRefIDFunc mocks the GetAlarmsByRefID method.
	GetAlarmsByRefIDFunc func(ctx context.Context, refID string, tenants ...string) ([]db.Alarm, error)

	// GetConfigurationFunc mocks the GetConfiguration method.
	GetConfigurationFunc func() Configuration

	// StartFunc mocks the Start method.
	StartFunc func()

	// StopFunc mocks the Stop method.
	StopFunc func()

	// calls tracks calls to the methods.
	calls struct {
		// AddAlarm holds details about calls to the AddAlarm method.
		AddAlarm []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Alarm is the alarm argument value.
			Alarm db.Alarm
		}
		// CloseAlarm holds details about calls to the CloseAlarm method.
		CloseAlarm []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// AlarmID is the alarmID argument value.
			AlarmID int
		}
		// GetAlarms holds details about calls to the GetAlarms method.
		GetAlarms []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetAlarmsByID holds details about calls to the GetAlarmsByID method.
		GetAlarmsByID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ID is the id argument value.
			ID int
		}
		// GetAlarmsByRefID holds details about calls to the GetAlarmsByRefID method.
		GetAlarmsByRefID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// RefID is the refID argument value.
			RefID string
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetConfiguration holds details about calls to the GetConfiguration method.
		GetConfiguration []struct {
		}
		// Start holds details about calls to the Start method.
		Start []struct {
		}
		// Stop holds details about calls to the Stop method.
		Stop []struct {
		}
	}
	lockAddAlarm         sync.RWMutex
	lockCloseAlarm       sync.RWMutex
	lockGetAlarms        sync.RWMutex
	lockGetAlarmsByID    sync.RWMutex
	lockGetAlarmsByRefID sync.RWMutex
	lockGetConfiguration sync.RWMutex
	lockStart            sync.RWMutex
	lockStop             sync.RWMutex
}

// AddAlarm calls AddAlarmFunc.
func (mock *AlarmServiceMock) AddAlarm(ctx context.Context, alarm db.Alarm) error {
	if mock.AddAlarmFunc == nil {
		panic("AlarmServiceMock.AddAlarmFunc: method is nil but AlarmService.AddAlarm was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Alarm db.Alarm
	}{
		Ctx:   ctx,
		Alarm: alarm,
	}
	mock.lockAddAlarm.Lock()
	mock.calls.AddAlarm = append(mock.calls.AddAlarm, callInfo)
	mock.lockAddAlarm.Unlock()
	return mock.AddAlarmFunc(ctx, alarm)
}

// AddAlarmCalls gets all the calls that were made to AddAlarm.
// Check the length with:
//
//	len(mockedAlarmService.AddAlarmCalls())
func (mock *AlarmServiceMock) AddAlarmCalls() []struct {
	Ctx   context.Context
	Alarm db.Alarm
} {
	var calls []struct {
		Ctx   context.Context
		Alarm db.Alarm
	}
	mock.lockAddAlarm.RLock()
	calls = mock.calls.AddAlarm
	mock.lockAddAlarm.RUnlock()
	return calls
}

// CloseAlarm calls CloseAlarmFunc.
func (mock *AlarmServiceMock) CloseAlarm(ctx context.Context, alarmID int) error {
	if mock.CloseAlarmFunc == nil {
		panic("AlarmServiceMock.CloseAlarmFunc: method is nil but AlarmService.CloseAlarm was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		AlarmID int
	}{
		Ctx:     ctx,
		AlarmID: alarmID,
	}
	mock.lockCloseAlarm.Lock()
	mock.calls.CloseAlarm = append(mock.calls.CloseAlarm, callInfo)
	mock.lockCloseAlarm.Unlock()
	return mock.CloseAlarmFunc(ctx, alarmID)
}

// CloseAlarmCalls gets all the calls that were made to CloseAlarm.
// Check the length with:
//
//	len(mockedAlarmService.CloseAlarmCalls())
func (mock *AlarmServiceMock) CloseAlarmCalls() []struct {
	Ctx     context.Context
	AlarmID int
} {
	var calls []struct {
		Ctx     context.Context
		AlarmID int
	}
	mock.lockCloseAlarm.RLock()
	calls = mock.calls.CloseAlarm
	mock.lockCloseAlarm.RUnlock()
	return calls
}

// GetAlarms calls GetAlarmsFunc.
func (mock *AlarmServiceMock) GetAlarms(ctx context.Context, tenants ...string) ([]db.Alarm, error) {
	if mock.GetAlarmsFunc == nil {
		panic("AlarmServiceMock.GetAlarmsFunc: method is nil but AlarmService.GetAlarms was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Tenants []string
	}{
		Ctx:     ctx,
		Tenants: tenants,
	}
	mock.lockGetAlarms.Lock()
	mock.calls.GetAlarms = append(mock.calls.GetAlarms, callInfo)
	mock.lockGetAlarms.Unlock()
	return mock.GetAlarmsFunc(ctx, tenants...)
}

// GetAlarmsCalls gets all the calls that were made to GetAlarms.
// Check the length with:
//
//	len(mockedAlarmService.GetAlarmsCalls())
func (mock *AlarmServiceMock) GetAlarmsCalls() []struct {
	Ctx     context.Context
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		Tenants []string
	}
	mock.lockGetAlarms.RLock()
	calls = mock.calls.GetAlarms
	mock.lockGetAlarms.RUnlock()
	return calls
}

// GetAlarmsByID calls GetAlarmsByIDFunc.
func (mock *AlarmServiceMock) GetAlarmsByID(ctx context.Context, id int) (db.Alarm, error) {
	if mock.GetAlarmsByIDFunc == nil {
		panic("AlarmServiceMock.GetAlarmsByIDFunc: method is nil but AlarmService.GetAlarmsByID was just called")
	}
	callInfo := struct {
		Ctx context.Context
		ID  int
	}{
		Ctx: ctx,
		ID:  id,
	}
	mock.lockGetAlarmsByID.Lock()
	mock.calls.GetAlarmsByID = append(mock.calls.GetAlarmsByID, callInfo)
	mock.lockGetAlarmsByID.Unlock()
	return mock.GetAlarmsByIDFunc(ctx, id)
}

// GetAlarmsByIDCalls gets all the calls that were made to GetAlarmsByID.
// Check the length with:
//
//	len(mockedAlarmService.GetAlarmsByIDCalls())
func (mock *AlarmServiceMock) GetAlarmsByIDCalls() []struct {
	Ctx context.Context
	ID  int
} {
	var calls []struct {
		Ctx context.Context
		ID  int
	}
	mock.lockGetAlarmsByID.RLock()
	calls = mock.calls.GetAlarmsByID
	mock.lockGetAlarmsByID.RUnlock()
	return calls
}

// GetAlarmsByRefID calls GetAlarmsByRefIDFunc.
func (mock *AlarmServiceMock) GetAlarmsByRefID(ctx context.Context, refID string, tenants ...string) ([]db.Alarm, error) {
	if mock.GetAlarmsByRefIDFunc == nil {
		panic("AlarmServiceMock.GetAlarmsByRefIDFunc: method is nil but AlarmService.GetAlarmsByRefID was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		RefID   string
		Tenants []string
	}{
		Ctx:     ctx,
		RefID:   refID,
		Tenants: tenants,
	}
	mock.lockGetAlarmsByRefID.Lock()
	mock.calls.GetAlarmsByRefID = append(mock.calls.GetAlarmsByRefID, callInfo)
	mock.lockGetAlarmsByRefID.Unlock()
	return mock.GetAlarmsByRefIDFunc(ctx, refID, tenants...)
}

// GetAlarmsByRefIDCalls gets all the calls that were made to GetAlarmsByRefID.
// Check the length with:
//
//	len(mockedAlarmService.GetAlarmsByRefIDCalls())
func (mock *AlarmServiceMock) GetAlarmsByRefIDCalls() []struct {
	Ctx     context.Context
	RefID   string
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		RefID   string
		Tenants []string
	}
	mock.lockGetAlarmsByRefID.RLock()
	calls = mock.calls.GetAlarmsByRefID
	mock.lockGetAlarmsByRefID.RUnlock()
	return calls
}

// GetConfiguration calls GetConfigurationFunc.
func (mock *AlarmServiceMock) GetConfiguration() Configuration {
	if mock.GetConfigurationFunc == nil {
		panic("AlarmServiceMock.GetConfigurationFunc: method is nil but AlarmService.GetConfiguration was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetConfiguration.Lock()
	mock.calls.GetConfiguration = append(mock.calls.GetConfiguration, callInfo)
	mock.lockGetConfiguration.Unlock()
	return mock.GetConfigurationFunc()
}

// GetConfigurationCalls gets all the calls that were made to GetConfiguration.
// Check the length with:
//
//	len(mockedAlarmService.GetConfigurationCalls())
func (mock *AlarmServiceMock) GetConfigurationCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetConfiguration.RLock()
	calls = mock.calls.GetConfiguration
	mock.lockGetConfiguration.RUnlock()
	return calls
}

// Start calls StartFunc.
func (mock *AlarmServiceMock) Start() {
	if mock.StartFunc == nil {
		panic("AlarmServiceMock.StartFunc: method is nil but AlarmService.Start was just called")
	}
	callInfo := struct {
	}{}
	mock.lockStart.Lock()
	mock.calls.Start = append(mock.calls.Start, callInfo)
	mock.lockStart.Unlock()
	mock.StartFunc()
}

// StartCalls gets all the calls that were made to Start.
// Check the length with:
//
//	len(mockedAlarmService.StartCalls())
func (mock *AlarmServiceMock) StartCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockStart.RLock()
	calls = mock.calls.Start
	mock.lockStart.RUnlock()
	return calls
}

// Stop calls StopFunc.
func (mock *AlarmServiceMock) Stop() {
	if mock.StopFunc == nil {
		panic("AlarmServiceMock.StopFunc: method is nil but AlarmService.Stop was just called")
	}
	callInfo := struct {
	}{}
	mock.lockStop.Lock()
	mock.calls.Stop = append(mock.calls.Stop, callInfo)
	mock.lockStop.Unlock()
	mock.StopFunc()
}

// StopCalls gets all the calls that were made to Stop.
// Check the length with:
//
//	len(mockedAlarmService.StopCalls())
func (mock *AlarmServiceMock) StopCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockStop.RLock()
	calls = mock.calls.Stop
	mock.lockStop.RUnlock()
	return calls
}
