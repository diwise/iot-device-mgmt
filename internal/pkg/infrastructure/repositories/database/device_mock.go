// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package database

import (
	"sync"
)

// Ensure, that DeviceMock does implement Device.
// If this is not the case, regenerate this file with moq.
var _ Device = &DeviceMock{}

// DeviceMock is a mock implementation of Device.
//
// 	func TestSomethingThatUsesDevice(t *testing.T) {
//
// 		// make and configure a mocked Device
// 		mockedDevice := &DeviceMock{
// 			IDFunc: func() string {
// 				panic("mock out the ID method")
// 			},
// 		}
//
// 		// use mockedDevice in code that requires Device
// 		// and then make assertions.
//
// 	}
type DeviceMock struct {
	// IDFunc mocks the ID method.
	IDFunc func() string

	// calls tracks calls to the methods.
	calls struct {
		// ID holds details about calls to the ID method.
		ID []struct {
		}
	}
	lockID sync.RWMutex
}

// ID calls IDFunc.
func (mock *DeviceMock) ID() string {
	if mock.IDFunc == nil {
		panic("DeviceMock.IDFunc: method is nil but Device.ID was just called")
	}
	callInfo := struct {
	}{}
	mock.lockID.Lock()
	mock.calls.ID = append(mock.calls.ID, callInfo)
	mock.lockID.Unlock()
	return mock.IDFunc()
}

// IDCalls gets all the calls that were made to ID.
// Check the length with:
//     len(mockedDevice.IDCalls())
func (mock *DeviceMock) IDCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockID.RLock()
	calls = mock.calls.ID
	mock.lockID.RUnlock()
	return calls
}
