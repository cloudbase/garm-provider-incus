// SPDX-License-Identifier: Apache-2.0
// Copyright 2023 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package provider

import (
	incus "github.com/lxc/incus/client"
	"github.com/lxc/incus/shared/api"
	"github.com/stretchr/testify/mock"
)

type MockIncusServer struct {
	mock.Mock
}

func (m *MockIncusServer) GetProject(name string) (project *api.Project, ETag string, err error) {
	args := m.Called(name)
	return args.Get(0).(*api.Project), args.String(1), args.Error(2)
}

func (m *MockIncusServer) UseProject(name string) (client incus.InstanceServer) {
	args := m.Called(name)
	return args.Get(0).(incus.InstanceServer)
}

func (m *MockIncusServer) GetProfileNames() (profiles []string, err error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockIncusServer) CreateInstance(instance api.InstancesPost) (op incus.Operation, err error) {
	args := m.Called(instance)
	return args.Get(0).(incus.Operation), args.Error(1)
}

func (m *MockIncusServer) UpdateInstanceState(name string, state api.InstanceStatePut, ETag string) (op incus.Operation, err error) {
	args := m.Called(name, state, ETag)
	return args.Get(0).(incus.Operation), args.Error(1)
}

func (m *MockIncusServer) GetInstanceFull(name string) (instance *api.InstanceFull, ETag string, err error) {
	args := m.Called(name)
	return args.Get(0).(*api.InstanceFull), args.String(1), args.Error(2)
}

func (m *MockIncusServer) DeleteInstance(name string) (op incus.Operation, err error) {
	args := m.Called(name)
	return args.Get(0).(incus.Operation), args.Error(1)
}

func (m *MockIncusServer) GetInstancesFull(instanceType api.InstanceType) (instances []api.InstanceFull, err error) {
	args := m.Called(instanceType)
	return args.Get(0).([]api.InstanceFull), args.Error(1)
}

func (m *MockIncusServer) GetImageAliasArchitectures(imageType string, name string) (entries map[string]*api.ImageAliasesEntry, err error) {
	args := m.Called(imageType, name)
	return args.Get(0).(map[string]*api.ImageAliasesEntry), args.Error(1)
}

func (m *MockIncusServer) GetImage(name string) (image *api.Image, ETag string, err error) {
	args := m.Called(name)
	return args.Get(0).(*api.Image), args.String(1), args.Error(2)
}
