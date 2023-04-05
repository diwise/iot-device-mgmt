// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package devicemanagement

import (
	"context"
	r "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	t "github.com/diwise/iot-device-mgmt/pkg/types"
	"sync"
)

// Ensure, that DeviceManagementMock does implement DeviceManagement.
// If this is not the case, regenerate this file with moq.
var _ DeviceManagement = &DeviceManagementMock{}

// DeviceManagementMock is a mock implementation of DeviceManagement.
//
//	func TestSomethingThatUsesDeviceManagement(t *testing.T) {
//
//		// make and configure a mocked DeviceManagement
//		mockedDeviceManagement := &DeviceManagementMock{
//			CreateDeviceFunc: func(ctx context.Context, device t.Device) error {
//				panic("mock out the CreateDevice method")
//			},
//			GetDeviceByDeviceIDFunc: func(ctx context.Context, deviceID string, tenants ...string) (r.Device, error) {
//				panic("mock out the GetDeviceByDeviceID method")
//			},
//			GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string, tenants ...string) (r.Device, error) {
//				panic("mock out the GetDeviceBySensorID method")
//			},
//			GetDevicesFunc: func(ctx context.Context, tenants ...string) ([]r.Device, error) {
//				panic("mock out the GetDevices method")
//			},
//			UpdateDeviceFunc: func(ctx context.Context, deviceID string, fields map[string]any) error {
//				panic("mock out the UpdateDevice method")
//			},
//			UpdateDeviceStateFunc: func(ctx context.Context, deviceID string, deviceState r.DeviceState) error {
//				panic("mock out the UpdateDeviceState method")
//			},
//			UpdateDeviceStatusFunc: func(ctx context.Context, deviceID string, deviceStatus r.DeviceStatus) error {
//				panic("mock out the UpdateDeviceStatus method")
//			},
//		}
//
//		// use mockedDeviceManagement in code that requires DeviceManagement
//		// and then make assertions.
//
//	}
type DeviceManagementMock struct {
	// CreateDeviceFunc mocks the CreateDevice method.
	CreateDeviceFunc func(ctx context.Context, device t.Device) error

	// GetDeviceByDeviceIDFunc mocks the GetDeviceByDeviceID method.
	GetDeviceByDeviceIDFunc func(ctx context.Context, deviceID string, tenants ...string) (r.Device, error)

	// GetDeviceBySensorIDFunc mocks the GetDeviceBySensorID method.
	GetDeviceBySensorIDFunc func(ctx context.Context, sensorID string, tenants ...string) (r.Device, error)

	// GetDevicesFunc mocks the GetDevices method.
	GetDevicesFunc func(ctx context.Context, tenants ...string) ([]r.Device, error)

	// UpdateDeviceFunc mocks the UpdateDevice method.
	UpdateDeviceFunc func(ctx context.Context, deviceID string, fields map[string]any) error

	// UpdateDeviceStateFunc mocks the UpdateDeviceState method.
	UpdateDeviceStateFunc func(ctx context.Context, deviceID string, deviceState r.DeviceState) error

	// UpdateDeviceStatusFunc mocks the UpdateDeviceStatus method.
	UpdateDeviceStatusFunc func(ctx context.Context, deviceID string, deviceStatus r.DeviceStatus) error

	// calls tracks calls to the methods.
	calls struct {
		// CreateDevice holds details about calls to the CreateDevice method.
		CreateDevice []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Device is the device argument value.
			Device t.Device
		}
		// GetDeviceByDeviceID holds details about calls to the GetDeviceByDeviceID method.
		GetDeviceByDeviceID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetDeviceBySensorID holds details about calls to the GetDeviceBySensorID method.
		GetDeviceBySensorID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// SensorID is the sensorID argument value.
			SensorID string
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetDevices holds details about calls to the GetDevices method.
		GetDevices []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// UpdateDevice holds details about calls to the UpdateDevice method.
		UpdateDevice []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// Fields is the fields argument value.
			Fields map[string]any
		}
		// UpdateDeviceState holds details about calls to the UpdateDeviceState method.
		UpdateDeviceState []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// DeviceState is the deviceState argument value.
			DeviceState r.DeviceState
		}
		// UpdateDeviceStatus holds details about calls to the UpdateDeviceStatus method.
		UpdateDeviceStatus []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// DeviceStatus is the deviceStatus argument value.
			DeviceStatus r.DeviceStatus
		}
	}
	lockCreateDevice        sync.RWMutex
	lockGetDeviceByDeviceID sync.RWMutex
	lockGetDeviceBySensorID sync.RWMutex
	lockGetDevices          sync.RWMutex
	lockUpdateDevice        sync.RWMutex
	lockUpdateDeviceState   sync.RWMutex
	lockUpdateDeviceStatus  sync.RWMutex
}

// CreateDevice calls CreateDeviceFunc.
func (mock *DeviceManagementMock) CreateDevice(ctx context.Context, device t.Device) error {
	if mock.CreateDeviceFunc == nil {
		panic("DeviceManagementMock.CreateDeviceFunc: method is nil but DeviceManagement.CreateDevice was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Device t.Device
	}{
		Ctx:    ctx,
		Device: device,
	}
	mock.lockCreateDevice.Lock()
	mock.calls.CreateDevice = append(mock.calls.CreateDevice, callInfo)
	mock.lockCreateDevice.Unlock()
	return mock.CreateDeviceFunc(ctx, device)
}

// CreateDeviceCalls gets all the calls that were made to CreateDevice.
// Check the length with:
//
//	len(mockedDeviceManagement.CreateDeviceCalls())
func (mock *DeviceManagementMock) CreateDeviceCalls() []struct {
	Ctx    context.Context
	Device t.Device
} {
	var calls []struct {
		Ctx    context.Context
		Device t.Device
	}
	mock.lockCreateDevice.RLock()
	calls = mock.calls.CreateDevice
	mock.lockCreateDevice.RUnlock()
	return calls
}

// GetDeviceByDeviceID calls GetDeviceByDeviceIDFunc.
func (mock *DeviceManagementMock) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (r.Device, error) {
	if mock.GetDeviceByDeviceIDFunc == nil {
		panic("DeviceManagementMock.GetDeviceByDeviceIDFunc: method is nil but DeviceManagement.GetDeviceByDeviceID was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		DeviceID string
		Tenants  []string
	}{
		Ctx:      ctx,
		DeviceID: deviceID,
		Tenants:  tenants,
	}
	mock.lockGetDeviceByDeviceID.Lock()
	mock.calls.GetDeviceByDeviceID = append(mock.calls.GetDeviceByDeviceID, callInfo)
	mock.lockGetDeviceByDeviceID.Unlock()
	return mock.GetDeviceByDeviceIDFunc(ctx, deviceID, tenants...)
}

// GetDeviceByDeviceIDCalls gets all the calls that were made to GetDeviceByDeviceID.
// Check the length with:
//
//	len(mockedDeviceManagement.GetDeviceByDeviceIDCalls())
func (mock *DeviceManagementMock) GetDeviceByDeviceIDCalls() []struct {
	Ctx      context.Context
	DeviceID string
	Tenants  []string
} {
	var calls []struct {
		Ctx      context.Context
		DeviceID string
		Tenants  []string
	}
	mock.lockGetDeviceByDeviceID.RLock()
	calls = mock.calls.GetDeviceByDeviceID
	mock.lockGetDeviceByDeviceID.RUnlock()
	return calls
}

// GetDeviceBySensorID calls GetDeviceBySensorIDFunc.
func (mock *DeviceManagementMock) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (r.Device, error) {
	if mock.GetDeviceBySensorIDFunc == nil {
		panic("DeviceManagementMock.GetDeviceBySensorIDFunc: method is nil but DeviceManagement.GetDeviceBySensorID was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		SensorID string
		Tenants  []string
	}{
		Ctx:      ctx,
		SensorID: sensorID,
		Tenants:  tenants,
	}
	mock.lockGetDeviceBySensorID.Lock()
	mock.calls.GetDeviceBySensorID = append(mock.calls.GetDeviceBySensorID, callInfo)
	mock.lockGetDeviceBySensorID.Unlock()
	return mock.GetDeviceBySensorIDFunc(ctx, sensorID, tenants...)
}

// GetDeviceBySensorIDCalls gets all the calls that were made to GetDeviceBySensorID.
// Check the length with:
//
//	len(mockedDeviceManagement.GetDeviceBySensorIDCalls())
func (mock *DeviceManagementMock) GetDeviceBySensorIDCalls() []struct {
	Ctx      context.Context
	SensorID string
	Tenants  []string
} {
	var calls []struct {
		Ctx      context.Context
		SensorID string
		Tenants  []string
	}
	mock.lockGetDeviceBySensorID.RLock()
	calls = mock.calls.GetDeviceBySensorID
	mock.lockGetDeviceBySensorID.RUnlock()
	return calls
}

// GetDevices calls GetDevicesFunc.
func (mock *DeviceManagementMock) GetDevices(ctx context.Context, tenants ...string) ([]r.Device, error) {
	if mock.GetDevicesFunc == nil {
		panic("DeviceManagementMock.GetDevicesFunc: method is nil but DeviceManagement.GetDevices was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Tenants []string
	}{
		Ctx:     ctx,
		Tenants: tenants,
	}
	mock.lockGetDevices.Lock()
	mock.calls.GetDevices = append(mock.calls.GetDevices, callInfo)
	mock.lockGetDevices.Unlock()
	return mock.GetDevicesFunc(ctx, tenants...)
}

// GetDevicesCalls gets all the calls that were made to GetDevices.
// Check the length with:
//
//	len(mockedDeviceManagement.GetDevicesCalls())
func (mock *DeviceManagementMock) GetDevicesCalls() []struct {
	Ctx     context.Context
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		Tenants []string
	}
	mock.lockGetDevices.RLock()
	calls = mock.calls.GetDevices
	mock.lockGetDevices.RUnlock()
	return calls
}

// UpdateDevice calls UpdateDeviceFunc.
func (mock *DeviceManagementMock) UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error {
	if mock.UpdateDeviceFunc == nil {
		panic("DeviceManagementMock.UpdateDeviceFunc: method is nil but DeviceManagement.UpdateDevice was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		DeviceID string
		Fields   map[string]any
	}{
		Ctx:      ctx,
		DeviceID: deviceID,
		Fields:   fields,
	}
	mock.lockUpdateDevice.Lock()
	mock.calls.UpdateDevice = append(mock.calls.UpdateDevice, callInfo)
	mock.lockUpdateDevice.Unlock()
	return mock.UpdateDeviceFunc(ctx, deviceID, fields)
}

// UpdateDeviceCalls gets all the calls that were made to UpdateDevice.
// Check the length with:
//
//	len(mockedDeviceManagement.UpdateDeviceCalls())
func (mock *DeviceManagementMock) UpdateDeviceCalls() []struct {
	Ctx      context.Context
	DeviceID string
	Fields   map[string]any
} {
	var calls []struct {
		Ctx      context.Context
		DeviceID string
		Fields   map[string]any
	}
	mock.lockUpdateDevice.RLock()
	calls = mock.calls.UpdateDevice
	mock.lockUpdateDevice.RUnlock()
	return calls
}

// UpdateDeviceState calls UpdateDeviceStateFunc.
func (mock *DeviceManagementMock) UpdateDeviceState(ctx context.Context, deviceID string, deviceState r.DeviceState) error {
	if mock.UpdateDeviceStateFunc == nil {
		panic("DeviceManagementMock.UpdateDeviceStateFunc: method is nil but DeviceManagement.UpdateDeviceState was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		DeviceID    string
		DeviceState r.DeviceState
	}{
		Ctx:         ctx,
		DeviceID:    deviceID,
		DeviceState: deviceState,
	}
	mock.lockUpdateDeviceState.Lock()
	mock.calls.UpdateDeviceState = append(mock.calls.UpdateDeviceState, callInfo)
	mock.lockUpdateDeviceState.Unlock()
	return mock.UpdateDeviceStateFunc(ctx, deviceID, deviceState)
}

// UpdateDeviceStateCalls gets all the calls that were made to UpdateDeviceState.
// Check the length with:
//
//	len(mockedDeviceManagement.UpdateDeviceStateCalls())
func (mock *DeviceManagementMock) UpdateDeviceStateCalls() []struct {
	Ctx         context.Context
	DeviceID    string
	DeviceState r.DeviceState
} {
	var calls []struct {
		Ctx         context.Context
		DeviceID    string
		DeviceState r.DeviceState
	}
	mock.lockUpdateDeviceState.RLock()
	calls = mock.calls.UpdateDeviceState
	mock.lockUpdateDeviceState.RUnlock()
	return calls
}

// UpdateDeviceStatus calls UpdateDeviceStatusFunc.
func (mock *DeviceManagementMock) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus r.DeviceStatus) error {
	if mock.UpdateDeviceStatusFunc == nil {
		panic("DeviceManagementMock.UpdateDeviceStatusFunc: method is nil but DeviceManagement.UpdateDeviceStatus was just called")
	}
	callInfo := struct {
		Ctx          context.Context
		DeviceID     string
		DeviceStatus r.DeviceStatus
	}{
		Ctx:          ctx,
		DeviceID:     deviceID,
		DeviceStatus: deviceStatus,
	}
	mock.lockUpdateDeviceStatus.Lock()
	mock.calls.UpdateDeviceStatus = append(mock.calls.UpdateDeviceStatus, callInfo)
	mock.lockUpdateDeviceStatus.Unlock()
	return mock.UpdateDeviceStatusFunc(ctx, deviceID, deviceStatus)
}

// UpdateDeviceStatusCalls gets all the calls that were made to UpdateDeviceStatus.
// Check the length with:
//
//	len(mockedDeviceManagement.UpdateDeviceStatusCalls())
func (mock *DeviceManagementMock) UpdateDeviceStatusCalls() []struct {
	Ctx          context.Context
	DeviceID     string
	DeviceStatus r.DeviceStatus
} {
	var calls []struct {
		Ctx          context.Context
		DeviceID     string
		DeviceStatus r.DeviceStatus
	}
	mock.lockUpdateDeviceStatus.RLock()
	calls = mock.calls.UpdateDeviceStatus
	mock.lockUpdateDeviceStatus.RUnlock()
	return calls
}