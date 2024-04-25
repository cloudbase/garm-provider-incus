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
	"testing"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-incus/config"
	"github.com/lxc/incus/shared/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetCLI(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Incus{
		UnixSocket: "/var/run/incus.sock",
	}
	l := &Incus{
		cfg: cfg,
		cli: &MockIncusServer{},
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}

	_, err := l.getCLI(ctx)
	require.NoError(t, err)
}

func TestGetProfiles(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)

	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	expected := []string{"default", "project"}

	cli.On("GetProfileNames").Return(expected, nil)
	profiles, err := l.getProfiles(ctx, "project")
	require.NoError(t, err)
	require.Equal(t, expected, profiles)
}

func TestGetCreateInstanceArgsContainer(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)

	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "container",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:           ptr("ubuntu"),
			Architecture: ptr("x86_64"),
			DownloadURL:  ptr("https://example.com"),
			Filename:     ptr("test-app"),
		},
	}
	aliases := map[string]*api.ImageAliasesEntry{
		"x86_64": {
			Name: "ubuntu",
			Type: "container",
		},
	}
	DefaultToolFetch = func(_ commonParams.OSType, _ commonParams.OSArch, tools []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error) {
		return tools[0], nil
	}
	DefaultGetCloudconfig = func(_ commonParams.BootstrapInstance, _ commonParams.RunnerApplicationDownload, _ string) (string, error) {
		return "#cloud-config", nil
	}

	cli.On("GetImageAliasArchitectures", config.IncusImageType("container").String(), "ubuntu").Return(aliases, nil)
	cli.On("GetImage", aliases["x86_64"].Target).Return(&api.Image{Fingerprint: "123abc"}, "", nil)
	cli.On("GetProfileNames").Return([]string{"default", "container"}, nil)
	specs := extraSpecs{}
	tests := []struct {
		name            string
		bootstrapParams commonParams.BootstrapInstance
		expected        api.InstancesPost
		errString       string
	}{
		{
			name:            "missing name",
			bootstrapParams: commonParams.BootstrapInstance{},
			expected:        api.InstancesPost{},
			errString:       "missing name",
		},
		{
			name: "looking for profile fails",
			bootstrapParams: commonParams.BootstrapInstance{
				Name:    "test-instance",
				Tools:   tools,
				Image:   "ubuntu",
				Flavor:  "bad-flavor",
				RepoURL: "mock-repo-url",
				PoolID:  "default",
				OSArch:  commonParams.Amd64,
				OSType:  commonParams.Linux,
			},
			expected:  api.InstancesPost{},
			errString: "looking for profile",
		},
		{
			name: "bad architecture fails",
			bootstrapParams: commonParams.BootstrapInstance{
				Name:    "test-instance",
				Tools:   tools,
				Image:   "ubuntu",
				Flavor:  "container",
				RepoURL: "mock-repo-url",
				PoolID:  "default",
				OSArch:  "bad-arch",
				OSType:  commonParams.Linux,
			},
			expected:  api.InstancesPost{},
			errString: "architecture bad-arch is not supported",
		},
		{
			name: "success container instance",
			bootstrapParams: commonParams.BootstrapInstance{
				Name:    "test-instance",
				Tools:   tools,
				Image:   "ubuntu",
				Flavor:  "container",
				RepoURL: "mock-repo-url",
				PoolID:  "default",
				OSArch:  commonParams.Amd64,
				OSType:  commonParams.Linux,
			},
			expected: api.InstancesPost{
				Name: "test-instance",
				InstancePut: api.InstancePut{
					Architecture: "x86_64",
					Profiles:     []string{"default", "container"},
					Description:  "Github runner provisioned by garm",
					Config: map[string]string{
						"user.user-data":    `#cloud-config`,
						osTypeKeyName:       "linux",
						osArchKeyNAme:       "amd64",
						controllerIDKeyName: "controller",
						poolIDKey:           "default",
					},
				},
				Source: api.InstanceSource{
					Type:        "image",
					Fingerprint: "123abc",
				},
				Type: "container",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := l.getCreateInstanceArgs(ctx, tt.bootstrapParams, specs)
			if tt.errString != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, ret)
		})
	}
}

func TestGetCreateInstanceArgsVM(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)

	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "virtual-machine",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:           ptr("windows"),
			Architecture: ptr("x86_64"),
			DownloadURL:  ptr("https://example.com"),
			Filename:     ptr("test-app"),
		},
	}
	aliases := map[string]*api.ImageAliasesEntry{
		"x86_64": {
			Name: "windows",
			Type: "container",
		},
	}
	DefaultToolFetch = func(_ commonParams.OSType, _ commonParams.OSArch, tools []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error) {
		return tools[0], nil
	}
	DefaultGetCloudconfig = func(_ commonParams.BootstrapInstance, _ commonParams.RunnerApplicationDownload, _ string) (string, error) {
		return "#cloud-config", nil
	}

	cli.On("GetImageAliasArchitectures", config.IncusImageType("virtual-machine").String(), "windows").Return(aliases, nil)
	cli.On("GetImage", aliases["x86_64"].Target).Return(&api.Image{Fingerprint: "123abc"}, "", nil)
	cli.On("GetProfileNames").Return([]string{"default", "virtual-machine"}, nil)
	specs := extraSpecs{}
	tests := []struct {
		name            string
		bootstrapParams commonParams.BootstrapInstance
		expected        api.InstancesPost
		errString       string
	}{
		{
			name:            "missing name",
			bootstrapParams: commonParams.BootstrapInstance{},
			expected:        api.InstancesPost{},
			errString:       "missing name",
		},
		{
			name: "success vm instance",
			bootstrapParams: commonParams.BootstrapInstance{
				Name:    "test-instance",
				Tools:   tools,
				Image:   "windows",
				Flavor:  "virtual-machine",
				RepoURL: "mock-repo-url",
				PoolID:  "default",
				OSArch:  commonParams.Amd64,
				OSType:  commonParams.Windows,
			},
			expected: api.InstancesPost{
				Name: "test-instance",
				InstancePut: api.InstancePut{
					Architecture: "x86_64",
					Profiles:     []string{"default", "virtual-machine"},
					Description:  "Github runner provisioned by garm",
					Config: map[string]string{
						"user.user-data":      "#ps1_sysnative\n" + "#cloud-config",
						osTypeKeyName:         "windows",
						osArchKeyNAme:         "amd64",
						controllerIDKeyName:   "controller",
						poolIDKey:             "default",
						"security.secureboot": "false",
					},
				},
				Source: api.InstanceSource{
					Type:        "image",
					Fingerprint: "123abc",
				},
				Type: "virtual-machine",
			},
			errString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := l.getCreateInstanceArgs(ctx, tt.bootstrapParams, specs)
			if tt.errString != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, ret)
		})
	}
}

func TestLaunchInstance(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)

	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "virtual-machine",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	createArgs := api.InstancesPost{
		Name: "test-instance",
		InstancePut: api.InstancePut{
			Architecture: "x86_64",
			Profiles:     []string{"default", "container"},
			Description:  "Github runner provisioned by garm",
			Config: map[string]string{
				"user.user-data":    `#cloud-config`,
				osTypeKeyName:       "linux",
				osArchKeyNAme:       "amd64",
				controllerIDKeyName: "controller",
				poolIDKey:           "default",
			},
		},
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: "123abc",
		},
		Type: "container",
	}
	DefaultToolFetch = func(_ commonParams.OSType, _ commonParams.OSArch, tools []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error) {
		return tools[0], nil
	}
	DefaultGetCloudconfig = func(_ commonParams.BootstrapInstance, _ commonParams.RunnerApplicationDownload, _ string) (string, error) {
		return "#cloud-config", nil
	}
	mockOp := new(MockOperation)
	mockOp.On("Wait").Return(nil)
	cli.On("CreateInstance", createArgs).Return(mockOp, nil)
	cli.On("UpdateInstanceState", "test-instance", api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}, "").Return(mockOp, nil)

	err := l.launchInstance(ctx, createArgs)
	require.NoError(t, err)
}

func TestCreateInstance(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)
	boostrapParams := commonParams.BootstrapInstance{
		Name: "test-instance",
		Tools: []commonParams.RunnerApplicationDownload{
			{
				OS:           ptr("windows"),
				Architecture: ptr("x86_64"),
				DownloadURL:  ptr("https://example.com"),
				Filename:     ptr("test-app"),
			},
		},
		Image:   "windows",
		Flavor:  "virtual-machine",
		RepoURL: "mock-repo-url",
		PoolID:  "default",
		OSArch:  commonParams.Amd64,
		OSType:  commonParams.Windows,
	}
	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "virtual-machine",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	aliases := map[string]*api.ImageAliasesEntry{
		"x86_64": {
			Name: "windows",
			Type: "virtual-machine",
		},
	}
	expectedOutput := commonParams.ProviderInstance{
		OSArch:     commonParams.Amd64,
		ProviderID: "test-instance",
		Name:       "test-instance",
		OSType:     commonParams.Windows,
		OSName:     "windows",
		OSVersion:  "",
		Addresses: []commonParams.Address{
			{
				Address: "10.10.0.0",
				Type:    commonParams.PublicAddress,
			},
		},
		Status: commonParams.InstanceRunning,
	}
	DefaultToolFetch = func(_ commonParams.OSType, _ commonParams.OSArch, tools []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error) {
		return tools[0], nil
	}
	DefaultGetCloudconfig = func(_ commonParams.BootstrapInstance, _ commonParams.RunnerApplicationDownload, _ string) (string, error) {
		return "#cloud-config", nil
	}
	cli.On("GetImageAliasArchitectures", config.IncusImageType("virtual-machine").String(), "windows").Return(aliases, nil)
	cli.On("GetImage", aliases["x86_64"].Target).Return(&api.Image{Fingerprint: "123abc"}, "", nil)
	cli.On("GetProfileNames").Return([]string{"default", "virtual-machine"}, nil)
	mockOp := new(MockOperation)
	mockOp.On("Wait").Return(nil)
	cli.On("CreateInstance", mock.Anything).Return(mockOp, nil)
	cli.On("UpdateInstanceState", "test-instance", api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}, "").Return(mockOp, nil)
	cli.On("GetInstanceFull", "test-instance").Return(&api.InstanceFull{
		Instance: api.Instance{
			InstancePut: api.InstancePut{
				Architecture: "x86_64",
			},
			Name: "test-instance",
			ExpandedConfig: map[string]string{
				"image.os":      "windows",
				"image.release": "",
			},
			Type: "container",
		},
		State: &api.InstanceState{
			Status: "Running",
			Network: map[string]api.InstanceStateNetwork{
				"eth0": {
					Addresses: []api.InstanceStateNetworkAddress{
						{
							Address: "10.10.0.0",
							Scope:   "global",
						},
					},
				},
			},
		},
	}, "", nil)

	ret, err := l.CreateInstance(ctx, boostrapParams)
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, ret)
}

func TestGetInstance(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)
	instanceName := "test-instance"
	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "virtual-machine",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	cli.On("GetInstanceFull", "test-instance").Return(&api.InstanceFull{
		Instance: api.Instance{
			InstancePut: api.InstancePut{
				Architecture: "x86_64",
			},
			Name: "test-instance",
			ExpandedConfig: map[string]string{
				"image.os":      "windows",
				"image.release": "",
			},
			Type: "container",
		},
		State: &api.InstanceState{
			Status: "Running",
			Network: map[string]api.InstanceStateNetwork{
				"eth0": {
					Addresses: []api.InstanceStateNetworkAddress{
						{
							Address: "10.10.0.0",
							Scope:   "global",
						},
					},
				},
			},
		},
	}, "", nil)
	expectedOutput := commonParams.ProviderInstance{
		OSArch:     commonParams.Amd64,
		ProviderID: "test-instance",
		Name:       "test-instance",
		OSType:     commonParams.Windows,
		OSName:     "windows",
		OSVersion:  "",
		Addresses: []commonParams.Address{
			{
				Address: "10.10.0.0",
				Type:    commonParams.PublicAddress,
			},
		},
		Status: commonParams.InstanceRunning,
	}

	ret, err := l.GetInstance(ctx, instanceName)
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, ret)
}

func TestDeleteInstance(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)
	instanceName := "test-instance"
	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "virtual-machine",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	mockOp := new(MockOperation)
	mockOp.On("WaitContext", mock.Anything).Return(nil)
	cli.On("DeleteInstance", "test-instance").Return(mockOp, nil)
	cli.On("UpdateInstanceState", "test-instance", api.InstanceStatePut{
		Action:  "stop",
		Timeout: -1,
		Force:   true,
	}, "").Return(mockOp, nil)

	err := l.DeleteInstance(ctx, instanceName)
	require.NoError(t, err)
}

func TestListInstances(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)
	poolID := "test-pool-id"
	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "virtual-machine",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	DefaultToolFetch = func(_ commonParams.OSType, _ commonParams.OSArch, tools []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error) {
		return tools[0], nil
	}
	DefaultGetCloudconfig = func(_ commonParams.BootstrapInstance, _ commonParams.RunnerApplicationDownload, _ string) (string, error) {
		return "#cloud-config", nil
	}
	cli.On("GetInstancesFull", api.InstanceTypeAny).Return([]api.InstanceFull{
		{
			Instance: api.Instance{
				InstancePut: api.InstancePut{
					Architecture: "x86_64",
				},
				Name: "test-instance",
				ExpandedConfig: map[string]string{
					"image.os":          "windows",
					"image.release":     "",
					poolIDKey:           poolID,
					controllerIDKeyName: "controller",
				},
				Type: "container",
			},
			State: &api.InstanceState{
				Status: "Running",
				Network: map[string]api.InstanceStateNetwork{
					"eth0": {
						Addresses: []api.InstanceStateNetworkAddress{
							{
								Address: "10.10.0.0",
								Scope:   "global",
							},
						},
					},
				},
			},
		},
	}, nil)
	expectedOutput := []commonParams.ProviderInstance{
		{
			OSArch:     commonParams.Amd64,
			ProviderID: "test-instance",
			Name:       "test-instance",
			OSType:     commonParams.Windows,
			OSName:     "windows",
			OSVersion:  "",
			Addresses: []commonParams.Address{
				{
					Address: "10.10.0.0",
					Type:    commonParams.PublicAddress,
				},
			},
			Status: commonParams.InstanceRunning,
		},
	}

	ret, err := l.ListInstances(ctx, poolID)
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, ret)
}

func TestRemoveAllInstances(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)
	poolID := "test-pool-id"
	instanceName := "test-instance"
	cfg := &config.Incus{
		UnixSocket:            "/var/run/incus.sock",
		InstanceType:          "virtual-machine",
		IncludeDefaultProfile: true,
	}
	l := &Incus{
		cfg: cfg,
		cli: cli,
		imageManager: &image{
			remotes: map[string]config.IncusImageRemote{
				"remote1": {
					Address: "remote1",
				},
			},
		},
		controllerID: "controller",
	}
	DefaultToolFetch = func(_ commonParams.OSType, _ commonParams.OSArch, tools []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error) {
		return tools[0], nil
	}
	DefaultGetCloudconfig = func(_ commonParams.BootstrapInstance, _ commonParams.RunnerApplicationDownload, _ string) (string, error) {
		return "#cloud-config", nil
	}
	cli.On("GetInstancesFull", api.InstanceTypeAny).Return([]api.InstanceFull{
		{
			Instance: api.Instance{
				InstancePut: api.InstancePut{
					Architecture: "x86_64",
				},
				Name: instanceName,
				ExpandedConfig: map[string]string{
					"image.os":          "windows",
					"image.release":     "",
					poolIDKey:           poolID,
					controllerIDKeyName: "controller",
				},
				Type: "container",
			},
			State: &api.InstanceState{
				Status: "Running",
			},
		},
	}, nil)
	mockOp := new(MockOperation)
	mockOp.On("WaitContext", mock.Anything).Return(nil)
	cli.On("DeleteInstance", instanceName).Return(mockOp, nil)
	cli.On("UpdateInstanceState", "test-instance", api.InstanceStatePut{
		Action:  "stop",
		Timeout: -1,
		Force:   true,
	}, "").Return(mockOp, nil)

	err := l.RemoveAllInstances(ctx)
	require.NoError(t, err)
}

func TestStop(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)
	force := true
	instanceName := "test-instance"
	l := &Incus{
		cfg:          &config.Incus{},
		cli:          cli,
		imageManager: &image{},
		controllerID: "controller",
	}
	mockOp := new(MockOperation)
	mockOp.On("WaitContext", mock.Anything).Return(nil)
	cli.On("UpdateInstanceState", instanceName, api.InstanceStatePut{
		Action:  "stop",
		Timeout: -1,
		Force:   force,
	}, "").Return(mockOp, nil)
	err := l.Stop(ctx, instanceName, force)
	require.NoError(t, err)
}

func TestStart(t *testing.T) {
	ctx := context.Background()
	cli := new(MockIncusServer)
	instanceName := "test-instance"
	l := &Incus{
		cfg:          &config.Incus{},
		cli:          cli,
		imageManager: &image{},
		controllerID: "controller",
	}
	mockOp := new(MockOperation)
	mockOp.On("WaitContext", mock.Anything).Return(nil)
	cli.On("UpdateInstanceState", instanceName, api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
		Force:   false,
	}, "").Return(mockOp, nil)
	err := l.Start(ctx, instanceName)
	require.NoError(t, err)
}
