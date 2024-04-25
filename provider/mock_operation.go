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
	"context"

	"github.com/gorilla/websocket"
	incus "github.com/lxc/incus/client"
	"github.com/lxc/incus/shared/api"
	"github.com/stretchr/testify/mock"
)

type MockOperation struct {
	mock.Mock
}

func (o *MockOperation) Wait() error {
	args := o.Called()
	return args.Error(0)
}

func (m *MockOperation) AddHandler(handler func(api.Operation)) (target *incus.EventTarget, err error) {
	args := m.Called(handler)
	return args.Get(0).(*incus.EventTarget), args.Error(1)
}

func (m *MockOperation) Cancel() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockOperation) Get() api.Operation {
	args := m.Called()
	return args.Get(0).(api.Operation)
}

func (m *MockOperation) GetWebsocket(secret string) (conn *websocket.Conn, err error) {
	args := m.Called(secret)
	return args.Get(0).(*websocket.Conn), args.Error(1)
}

func (m *MockOperation) RemoveHandler(target *incus.EventTarget) error {
	args := m.Called(target)
	return args.Error(0)
}

func (m *MockOperation) Refresh() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockOperation) WaitContext(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
