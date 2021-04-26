// Copyright 2021 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient (interfaces: ClusterClient)

// Package testing is a generated GoMock package.
package testing

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	v1alpha1 "github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	clusterclient "github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	workqueue "k8s.io/client-go/util/workqueue"
	reflect "reflect"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// MockClusterClient is a mock of ClusterClient interface
type MockClusterClient struct {
	ctrl     *gomock.Controller
	recorder *MockClusterClientMockRecorder
}

// MockClusterClientMockRecorder is the mock recorder for MockClusterClient
type MockClusterClientMockRecorder struct {
	mock *MockClusterClient
}

// NewMockClusterClient creates a new mock instance
func NewMockClusterClient(ctrl *gomock.Controller) *MockClusterClient {
	mock := &MockClusterClient{ctrl: ctrl}
	mock.recorder = &MockClusterClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockClusterClient) EXPECT() *MockClusterClientMockRecorder {
	return m.recorder
}

// Create mocks base method
func (m *MockClusterClient) Create(arg0 context.Context, arg1 runtime.Object, arg2 ...client.CreateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create
func (mr *MockClusterClientMockRecorder) Create(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockClusterClient)(nil).Create), varargs...)
}

// Delete mocks base method
func (m *MockClusterClient) Delete(arg0 context.Context, arg1 runtime.Object, arg2 ...client.DeleteOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Delete", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockClusterClientMockRecorder) Delete(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockClusterClient)(nil).Delete), varargs...)
}

// DeleteAllOf mocks base method
func (m *MockClusterClient) DeleteAllOf(arg0 context.Context, arg1 runtime.Object, arg2 ...client.DeleteAllOfOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteAllOf", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAllOf indicates an expected call of DeleteAllOf
func (mr *MockClusterClientMockRecorder) DeleteAllOf(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAllOf", reflect.TypeOf((*MockClusterClient)(nil).DeleteAllOf), varargs...)
}

// Get mocks base method
func (m *MockClusterClient) Get(arg0 context.Context, arg1 types.NamespacedName, arg2 runtime.Object) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Get indicates an expected call of Get
func (mr *MockClusterClientMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockClusterClient)(nil).Get), arg0, arg1, arg2)
}

// GetClusterID mocks base method
func (m *MockClusterClient) GetClusterID() clusterclient.ClusterID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClusterID")
	ret0, _ := ret[0].(clusterclient.ClusterID)
	return ret0
}

// GetClusterID indicates an expected call of GetClusterID
func (mr *MockClusterClientMockRecorder) GetClusterID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClusterID", reflect.TypeOf((*MockClusterClient)(nil).GetClusterID))
}

// GetClusterStatus mocks base method
func (m *MockClusterClient) GetClusterStatus() *v1alpha1.MemberClusterStatus {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClusterStatus")
	ret0, _ := ret[0].(*v1alpha1.MemberClusterStatus)
	return ret0
}

// GetClusterStatus indicates an expected call of GetClusterStatus
func (mr *MockClusterClientMockRecorder) GetClusterStatus() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClusterStatus", reflect.TypeOf((*MockClusterClient)(nil).GetClusterStatus))
}

// List mocks base method
func (m *MockClusterClient) List(arg0 context.Context, arg1 runtime.Object, arg2 ...client.ListOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "List", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// List indicates an expected call of List
func (mr *MockClusterClientMockRecorder) List(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockClusterClient)(nil).List), varargs...)
}

// Patch mocks base method
func (m *MockClusterClient) Patch(arg0 context.Context, arg1 runtime.Object, arg2 client.Patch, arg3 ...client.PatchOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Patch", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Patch indicates an expected call of Patch
func (mr *MockClusterClientMockRecorder) Patch(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Patch", reflect.TypeOf((*MockClusterClient)(nil).Patch), varargs...)
}

// StartLeaderElection mocks base method
func (m *MockClusterClient) StartLeaderElection(arg0 workqueue.RateLimitingInterface) (chan<- struct{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartLeaderElection", arg0)
	ret0, _ := ret[0].(chan<- struct{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartLeaderElection indicates an expected call of StartLeaderElection
func (mr *MockClusterClientMockRecorder) StartLeaderElection(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartLeaderElection", reflect.TypeOf((*MockClusterClient)(nil).StartLeaderElection), arg0)
}

// StartMonitorResources mocks base method
func (m *MockClusterClient) StartMonitorResources(arg0 workqueue.RateLimitingInterface) (chan<- struct{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartMonitorResources", arg0)
	ret0, _ := ret[0].(chan<- struct{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartMonitorResources indicates an expected call of StartMonitorResources
func (mr *MockClusterClientMockRecorder) StartMonitorResources(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartMonitorResources", reflect.TypeOf((*MockClusterClient)(nil).StartMonitorResources), arg0)
}

// Status mocks base method
func (m *MockClusterClient) Status() client.StatusWriter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(client.StatusWriter)
	return ret0
}

// Status indicates an expected call of Status
func (mr *MockClusterClientMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockClusterClient)(nil).Status))
}

// Update mocks base method
func (m *MockClusterClient) Update(arg0 context.Context, arg1 runtime.Object, arg2 ...client.UpdateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update
func (mr *MockClusterClientMockRecorder) Update(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockClusterClient)(nil).Update), varargs...)
}
