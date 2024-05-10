// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package devicemanagement

import (
	"context"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
	"io"
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
//			AddDeviceProfilesFunc: func(ctx context.Context, reader io.Reader, tenants []string) error {
//				panic("mock out the AddDeviceProfiles method")
//			},
//			CreateFunc: func(ctx context.Context, device models.Device) error {
//				panic("mock out the Create method")
//			},
//			GetFunc: func(ctx context.Context, offset int, limit int, tenants []string) (repositories.Collection[models.Device], error) {
//				panic("mock out the Get method")
//			},
//			GetByDeviceIDFunc: func(ctx context.Context, deviceID string, tenants []string) (models.Device, error) {
//				panic("mock out the GetByDeviceID method")
//			},
//			GetBySensorIDFunc: func(ctx context.Context, sensorID string, tenants []string) (models.Device, error) {
//				panic("mock out the GetBySensorID method")
//			},
//			GetDeviceProfilesFunc: func(ctx context.Context, name string, tenants []string) (repositories.Collection[models.DeviceProfile], error) {
//				panic("mock out the GetDeviceProfiles method")
//			},
//			GetWithAlarmIDFunc: func(ctx context.Context, alarmID string, tenants []string) (models.Device, error) {
//				panic("mock out the GetWithAlarmID method")
//			},
//			MergeFunc: func(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
//				panic("mock out the Merge method")
//			},
//			SeedFunc: func(ctx context.Context, reader io.Reader, tenants []string) error {
//				panic("mock out the Seed method")
//			},
//			UpdateFunc: func(ctx context.Context, device models.Device) error {
//				panic("mock out the Update method")
//			},
//			UpdateStateFunc: func(ctx context.Context, deviceID string, tenant string, deviceState models.DeviceState) error {
//				panic("mock out the UpdateState method")
//			},
//			UpdateStatusFunc: func(ctx context.Context, deviceID string, tenant string, deviceStatus models.DeviceStatus) error {
//				panic("mock out the UpdateStatus method")
//			},
//		}
//
//		// use mockedDeviceManagement in code that requires DeviceManagement
//		// and then make assertions.
//
//	}
type DeviceManagementMock struct {
	// AddDeviceProfilesFunc mocks the AddDeviceProfiles method.
	AddDeviceProfilesFunc func(ctx context.Context, reader io.Reader, tenants []string) error

	// CreateFunc mocks the Create method.
	CreateFunc func(ctx context.Context, device models.Device) error

	// GetFunc mocks the Get method.
	GetFunc func(ctx context.Context, offset int, limit int, tenants []string) (repositories.Collection[models.Device], error)

	// GetByDeviceIDFunc mocks the GetByDeviceID method.
	GetByDeviceIDFunc func(ctx context.Context, deviceID string, tenants []string) (models.Device, error)

	// GetBySensorIDFunc mocks the GetBySensorID method.
	GetBySensorIDFunc func(ctx context.Context, sensorID string, tenants []string) (models.Device, error)

	// GetDeviceProfilesFunc mocks the GetDeviceProfiles method.
	GetDeviceProfilesFunc func(ctx context.Context, name string, tenants []string) (repositories.Collection[models.DeviceProfile], error)

	// GetWithAlarmIDFunc mocks the GetWithAlarmID method.
	GetWithAlarmIDFunc func(ctx context.Context, alarmID string, tenants []string) (models.Device, error)

	// MergeFunc mocks the Merge method.
	MergeFunc func(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error

	// SeedFunc mocks the Seed method.
	SeedFunc func(ctx context.Context, reader io.Reader, tenants []string) error

	// UpdateFunc mocks the Update method.
	UpdateFunc func(ctx context.Context, device models.Device) error

	// UpdateStateFunc mocks the UpdateState method.
	UpdateStateFunc func(ctx context.Context, deviceID string, tenant string, deviceState models.DeviceState) error

	// UpdateStatusFunc mocks the UpdateStatus method.
	UpdateStatusFunc func(ctx context.Context, deviceID string, tenant string, deviceStatus models.DeviceStatus) error

	// calls tracks calls to the methods.
	calls struct {
		// AddDeviceProfiles holds details about calls to the AddDeviceProfiles method.
		AddDeviceProfiles []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Reader is the reader argument value.
			Reader io.Reader
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// Create holds details about calls to the Create method.
		Create []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Device is the device argument value.
			Device models.Device
		}
		// Get holds details about calls to the Get method.
		Get []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Offset is the offset argument value.
			Offset int
			// Limit is the limit argument value.
			Limit int
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetByDeviceID holds details about calls to the GetByDeviceID method.
		GetByDeviceID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetBySensorID holds details about calls to the GetBySensorID method.
		GetBySensorID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// SensorID is the sensorID argument value.
			SensorID string
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetDeviceProfiles holds details about calls to the GetDeviceProfiles method.
		GetDeviceProfiles []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Name is the name argument value.
			Name string
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// GetWithAlarmID holds details about calls to the GetWithAlarmID method.
		GetWithAlarmID []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// AlarmID is the alarmID argument value.
			AlarmID string
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// Merge holds details about calls to the Merge method.
		Merge []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// Fields is the fields argument value.
			Fields map[string]any
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// Seed holds details about calls to the Seed method.
		Seed []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Reader is the reader argument value.
			Reader io.Reader
			// Tenants is the tenants argument value.
			Tenants []string
		}
		// Update holds details about calls to the Update method.
		Update []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Device is the device argument value.
			Device models.Device
		}
		// UpdateState holds details about calls to the UpdateState method.
		UpdateState []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// Tenant is the tenant argument value.
			Tenant string
			// DeviceState is the deviceState argument value.
			DeviceState models.DeviceState
		}
		// UpdateStatus holds details about calls to the UpdateStatus method.
		UpdateStatus []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// DeviceID is the deviceID argument value.
			DeviceID string
			// Tenant is the tenant argument value.
			Tenant string
			// DeviceStatus is the deviceStatus argument value.
			DeviceStatus models.DeviceStatus
		}
	}
	lockAddDeviceProfiles sync.RWMutex
	lockCreate            sync.RWMutex
	lockGet               sync.RWMutex
	lockGetByDeviceID     sync.RWMutex
	lockGetBySensorID     sync.RWMutex
	lockGetDeviceProfiles sync.RWMutex
	lockGetWithAlarmID    sync.RWMutex
	lockMerge             sync.RWMutex
	lockSeed              sync.RWMutex
	lockUpdate            sync.RWMutex
	lockUpdateState       sync.RWMutex
	lockUpdateStatus      sync.RWMutex
}

// AddDeviceProfiles calls AddDeviceProfilesFunc.
func (mock *DeviceManagementMock) AddDeviceProfiles(ctx context.Context, reader io.Reader, tenants []string) error {
	if mock.AddDeviceProfilesFunc == nil {
		panic("DeviceManagementMock.AddDeviceProfilesFunc: method is nil but DeviceManagement.AddDeviceProfiles was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Reader  io.Reader
		Tenants []string
	}{
		Ctx:     ctx,
		Reader:  reader,
		Tenants: tenants,
	}
	mock.lockAddDeviceProfiles.Lock()
	mock.calls.AddDeviceProfiles = append(mock.calls.AddDeviceProfiles, callInfo)
	mock.lockAddDeviceProfiles.Unlock()
	return mock.AddDeviceProfilesFunc(ctx, reader, tenants)
}

// AddDeviceProfilesCalls gets all the calls that were made to AddDeviceProfiles.
// Check the length with:
//
//	len(mockedDeviceManagement.AddDeviceProfilesCalls())
func (mock *DeviceManagementMock) AddDeviceProfilesCalls() []struct {
	Ctx     context.Context
	Reader  io.Reader
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		Reader  io.Reader
		Tenants []string
	}
	mock.lockAddDeviceProfiles.RLock()
	calls = mock.calls.AddDeviceProfiles
	mock.lockAddDeviceProfiles.RUnlock()
	return calls
}

// Create calls CreateFunc.
func (mock *DeviceManagementMock) Create(ctx context.Context, device models.Device) error {
	if mock.CreateFunc == nil {
		panic("DeviceManagementMock.CreateFunc: method is nil but DeviceManagement.Create was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Device models.Device
	}{
		Ctx:    ctx,
		Device: device,
	}
	mock.lockCreate.Lock()
	mock.calls.Create = append(mock.calls.Create, callInfo)
	mock.lockCreate.Unlock()
	return mock.CreateFunc(ctx, device)
}

// CreateCalls gets all the calls that were made to Create.
// Check the length with:
//
//	len(mockedDeviceManagement.CreateCalls())
func (mock *DeviceManagementMock) CreateCalls() []struct {
	Ctx    context.Context
	Device models.Device
} {
	var calls []struct {
		Ctx    context.Context
		Device models.Device
	}
	mock.lockCreate.RLock()
	calls = mock.calls.Create
	mock.lockCreate.RUnlock()
	return calls
}

// Get calls GetFunc.
func (mock *DeviceManagementMock) Get(ctx context.Context, offset int, limit int, tenants []string) (repositories.Collection[models.Device], error) {
	if mock.GetFunc == nil {
		panic("DeviceManagementMock.GetFunc: method is nil but DeviceManagement.Get was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Offset  int
		Limit   int
		Tenants []string
	}{
		Ctx:     ctx,
		Offset:  offset,
		Limit:   limit,
		Tenants: tenants,
	}
	mock.lockGet.Lock()
	mock.calls.Get = append(mock.calls.Get, callInfo)
	mock.lockGet.Unlock()
	return mock.GetFunc(ctx, offset, limit, tenants)
}

// GetCalls gets all the calls that were made to Get.
// Check the length with:
//
//	len(mockedDeviceManagement.GetCalls())
func (mock *DeviceManagementMock) GetCalls() []struct {
	Ctx     context.Context
	Offset  int
	Limit   int
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		Offset  int
		Limit   int
		Tenants []string
	}
	mock.lockGet.RLock()
	calls = mock.calls.Get
	mock.lockGet.RUnlock()
	return calls
}

// GetByDeviceID calls GetByDeviceIDFunc.
func (mock *DeviceManagementMock) GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error) {
	if mock.GetByDeviceIDFunc == nil {
		panic("DeviceManagementMock.GetByDeviceIDFunc: method is nil but DeviceManagement.GetByDeviceID was just called")
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
	mock.lockGetByDeviceID.Lock()
	mock.calls.GetByDeviceID = append(mock.calls.GetByDeviceID, callInfo)
	mock.lockGetByDeviceID.Unlock()
	return mock.GetByDeviceIDFunc(ctx, deviceID, tenants)
}

// GetByDeviceIDCalls gets all the calls that were made to GetByDeviceID.
// Check the length with:
//
//	len(mockedDeviceManagement.GetByDeviceIDCalls())
func (mock *DeviceManagementMock) GetByDeviceIDCalls() []struct {
	Ctx      context.Context
	DeviceID string
	Tenants  []string
} {
	var calls []struct {
		Ctx      context.Context
		DeviceID string
		Tenants  []string
	}
	mock.lockGetByDeviceID.RLock()
	calls = mock.calls.GetByDeviceID
	mock.lockGetByDeviceID.RUnlock()
	return calls
}

// GetBySensorID calls GetBySensorIDFunc.
func (mock *DeviceManagementMock) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error) {
	if mock.GetBySensorIDFunc == nil {
		panic("DeviceManagementMock.GetBySensorIDFunc: method is nil but DeviceManagement.GetBySensorID was just called")
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
	mock.lockGetBySensorID.Lock()
	mock.calls.GetBySensorID = append(mock.calls.GetBySensorID, callInfo)
	mock.lockGetBySensorID.Unlock()
	return mock.GetBySensorIDFunc(ctx, sensorID, tenants)
}

// GetBySensorIDCalls gets all the calls that were made to GetBySensorID.
// Check the length with:
//
//	len(mockedDeviceManagement.GetBySensorIDCalls())
func (mock *DeviceManagementMock) GetBySensorIDCalls() []struct {
	Ctx      context.Context
	SensorID string
	Tenants  []string
} {
	var calls []struct {
		Ctx      context.Context
		SensorID string
		Tenants  []string
	}
	mock.lockGetBySensorID.RLock()
	calls = mock.calls.GetBySensorID
	mock.lockGetBySensorID.RUnlock()
	return calls
}

// GetDeviceProfiles calls GetDeviceProfilesFunc.
func (mock *DeviceManagementMock) GetDeviceProfiles(ctx context.Context, name string, tenants []string) (repositories.Collection[models.DeviceProfile], error) {
	if mock.GetDeviceProfilesFunc == nil {
		panic("DeviceManagementMock.GetDeviceProfilesFunc: method is nil but DeviceManagement.GetDeviceProfiles was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Name    string
		Tenants []string
	}{
		Ctx:     ctx,
		Name:    name,
		Tenants: tenants,
	}
	mock.lockGetDeviceProfiles.Lock()
	mock.calls.GetDeviceProfiles = append(mock.calls.GetDeviceProfiles, callInfo)
	mock.lockGetDeviceProfiles.Unlock()
	return mock.GetDeviceProfilesFunc(ctx, name, tenants)
}

// GetDeviceProfilesCalls gets all the calls that were made to GetDeviceProfiles.
// Check the length with:
//
//	len(mockedDeviceManagement.GetDeviceProfilesCalls())
func (mock *DeviceManagementMock) GetDeviceProfilesCalls() []struct {
	Ctx     context.Context
	Name    string
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		Name    string
		Tenants []string
	}
	mock.lockGetDeviceProfiles.RLock()
	calls = mock.calls.GetDeviceProfiles
	mock.lockGetDeviceProfiles.RUnlock()
	return calls
}

// GetWithAlarmID calls GetWithAlarmIDFunc.
func (mock *DeviceManagementMock) GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (models.Device, error) {
	if mock.GetWithAlarmIDFunc == nil {
		panic("DeviceManagementMock.GetWithAlarmIDFunc: method is nil but DeviceManagement.GetWithAlarmID was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		AlarmID string
		Tenants []string
	}{
		Ctx:     ctx,
		AlarmID: alarmID,
		Tenants: tenants,
	}
	mock.lockGetWithAlarmID.Lock()
	mock.calls.GetWithAlarmID = append(mock.calls.GetWithAlarmID, callInfo)
	mock.lockGetWithAlarmID.Unlock()
	return mock.GetWithAlarmIDFunc(ctx, alarmID, tenants)
}

// GetWithAlarmIDCalls gets all the calls that were made to GetWithAlarmID.
// Check the length with:
//
//	len(mockedDeviceManagement.GetWithAlarmIDCalls())
func (mock *DeviceManagementMock) GetWithAlarmIDCalls() []struct {
	Ctx     context.Context
	AlarmID string
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		AlarmID string
		Tenants []string
	}
	mock.lockGetWithAlarmID.RLock()
	calls = mock.calls.GetWithAlarmID
	mock.lockGetWithAlarmID.RUnlock()
	return calls
}

// Merge calls MergeFunc.
func (mock *DeviceManagementMock) Merge(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
	if mock.MergeFunc == nil {
		panic("DeviceManagementMock.MergeFunc: method is nil but DeviceManagement.Merge was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		DeviceID string
		Fields   map[string]any
		Tenants  []string
	}{
		Ctx:      ctx,
		DeviceID: deviceID,
		Fields:   fields,
		Tenants:  tenants,
	}
	mock.lockMerge.Lock()
	mock.calls.Merge = append(mock.calls.Merge, callInfo)
	mock.lockMerge.Unlock()
	return mock.MergeFunc(ctx, deviceID, fields, tenants)
}

// MergeCalls gets all the calls that were made to Merge.
// Check the length with:
//
//	len(mockedDeviceManagement.MergeCalls())
func (mock *DeviceManagementMock) MergeCalls() []struct {
	Ctx      context.Context
	DeviceID string
	Fields   map[string]any
	Tenants  []string
} {
	var calls []struct {
		Ctx      context.Context
		DeviceID string
		Fields   map[string]any
		Tenants  []string
	}
	mock.lockMerge.RLock()
	calls = mock.calls.Merge
	mock.lockMerge.RUnlock()
	return calls
}

// Seed calls SeedFunc.
func (mock *DeviceManagementMock) Seed(ctx context.Context, reader io.Reader, tenants []string) error {
	if mock.SeedFunc == nil {
		panic("DeviceManagementMock.SeedFunc: method is nil but DeviceManagement.Seed was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Reader  io.Reader
		Tenants []string
	}{
		Ctx:     ctx,
		Reader:  reader,
		Tenants: tenants,
	}
	mock.lockSeed.Lock()
	mock.calls.Seed = append(mock.calls.Seed, callInfo)
	mock.lockSeed.Unlock()
	return mock.SeedFunc(ctx, reader, tenants)
}

// SeedCalls gets all the calls that were made to Seed.
// Check the length with:
//
//	len(mockedDeviceManagement.SeedCalls())
func (mock *DeviceManagementMock) SeedCalls() []struct {
	Ctx     context.Context
	Reader  io.Reader
	Tenants []string
} {
	var calls []struct {
		Ctx     context.Context
		Reader  io.Reader
		Tenants []string
	}
	mock.lockSeed.RLock()
	calls = mock.calls.Seed
	mock.lockSeed.RUnlock()
	return calls
}

// Update calls UpdateFunc.
func (mock *DeviceManagementMock) Update(ctx context.Context, device models.Device) error {
	if mock.UpdateFunc == nil {
		panic("DeviceManagementMock.UpdateFunc: method is nil but DeviceManagement.Update was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Device models.Device
	}{
		Ctx:    ctx,
		Device: device,
	}
	mock.lockUpdate.Lock()
	mock.calls.Update = append(mock.calls.Update, callInfo)
	mock.lockUpdate.Unlock()
	return mock.UpdateFunc(ctx, device)
}

// UpdateCalls gets all the calls that were made to Update.
// Check the length with:
//
//	len(mockedDeviceManagement.UpdateCalls())
func (mock *DeviceManagementMock) UpdateCalls() []struct {
	Ctx    context.Context
	Device models.Device
} {
	var calls []struct {
		Ctx    context.Context
		Device models.Device
	}
	mock.lockUpdate.RLock()
	calls = mock.calls.Update
	mock.lockUpdate.RUnlock()
	return calls
}

// UpdateState calls UpdateStateFunc.
func (mock *DeviceManagementMock) UpdateState(ctx context.Context, deviceID string, tenant string, deviceState models.DeviceState) error {
	if mock.UpdateStateFunc == nil {
		panic("DeviceManagementMock.UpdateStateFunc: method is nil but DeviceManagement.UpdateState was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		DeviceID    string
		Tenant      string
		DeviceState models.DeviceState
	}{
		Ctx:         ctx,
		DeviceID:    deviceID,
		Tenant:      tenant,
		DeviceState: deviceState,
	}
	mock.lockUpdateState.Lock()
	mock.calls.UpdateState = append(mock.calls.UpdateState, callInfo)
	mock.lockUpdateState.Unlock()
	return mock.UpdateStateFunc(ctx, deviceID, tenant, deviceState)
}

// UpdateStateCalls gets all the calls that were made to UpdateState.
// Check the length with:
//
//	len(mockedDeviceManagement.UpdateStateCalls())
func (mock *DeviceManagementMock) UpdateStateCalls() []struct {
	Ctx         context.Context
	DeviceID    string
	Tenant      string
	DeviceState models.DeviceState
} {
	var calls []struct {
		Ctx         context.Context
		DeviceID    string
		Tenant      string
		DeviceState models.DeviceState
	}
	mock.lockUpdateState.RLock()
	calls = mock.calls.UpdateState
	mock.lockUpdateState.RUnlock()
	return calls
}

// UpdateStatus calls UpdateStatusFunc.
func (mock *DeviceManagementMock) UpdateStatus(ctx context.Context, deviceID string, tenant string, deviceStatus models.DeviceStatus) error {
	if mock.UpdateStatusFunc == nil {
		panic("DeviceManagementMock.UpdateStatusFunc: method is nil but DeviceManagement.UpdateStatus was just called")
	}
	callInfo := struct {
		Ctx          context.Context
		DeviceID     string
		Tenant       string
		DeviceStatus models.DeviceStatus
	}{
		Ctx:          ctx,
		DeviceID:     deviceID,
		Tenant:       tenant,
		DeviceStatus: deviceStatus,
	}
	mock.lockUpdateStatus.Lock()
	mock.calls.UpdateStatus = append(mock.calls.UpdateStatus, callInfo)
	mock.lockUpdateStatus.Unlock()
	return mock.UpdateStatusFunc(ctx, deviceID, tenant, deviceStatus)
}

// UpdateStatusCalls gets all the calls that were made to UpdateStatus.
// Check the length with:
//
//	len(mockedDeviceManagement.UpdateStatusCalls())
func (mock *DeviceManagementMock) UpdateStatusCalls() []struct {
	Ctx          context.Context
	DeviceID     string
	Tenant       string
	DeviceStatus models.DeviceStatus
} {
	var calls []struct {
		Ctx          context.Context
		DeviceID     string
		Tenant       string
		DeviceStatus models.DeviceStatus
	}
	mock.lockUpdateStatus.RLock()
	calls = mock.calls.UpdateStatus
	mock.lockUpdateStatus.RUnlock()
	return calls
}
