// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-incus/config"
	"github.com/lxc/incus/v7/shared/api"
	"github.com/stretchr/testify/require"
)

type readinessServer struct {
	*MockIncusServer
	profile    *api.Profile
	profileErr error
	server     *api.Server
	state      *api.ProjectState
	serverErr  error
	stateErr   error
}

func (s *readinessServer) GetProfile(string) (*api.Profile, string, error) {
	return s.profile, "", s.profileErr
}

func (s *readinessServer) GetServer() (*api.Server, string, error) {
	return s.server, "", s.serverErr
}
func (s *readinessServer) GetProjectState(string) (*api.ProjectState, error) {
	return s.state, s.stateErr
}

func TestReadinessConfigurationProbeFailsClosed(t *testing.T) {
	req := ReadinessRequest{Version: 1, RequestID: "test", Generation: "g1", Domain: "host", Project: "test", Image: strings.Repeat("a", 64), Flavor: "runner", OSArch: "amd64", InstanceType: "virtual-machine", Resources: ReadinessResources{CPU: 2, MemoryBytes: 2 << 30, DiskBytes: 10 << 30, VMSlots: 1}}
	for _, tc := range []struct {
		name       string
		profileErr error
		memory     string
		image      string
		want       string
	}{
		{name: "valid metadata", memory: "2GiB", image: strings.Repeat("a", 64), want: "create permission, KVM health, and host-wide capacity are not verified"},
		{name: "denied profile", profileErr: errors.New("secret"), memory: "2GiB", image: strings.Repeat("a", 64), want: "profile or resources unavailable or mismatched"},
		{name: "unbounded memory", memory: "", image: strings.Repeat("a", 64), want: "profile or resources unavailable or mismatched"},
		{name: "remote image", memory: "2GiB", image: "remote:image", want: "image fingerprint required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := new(MockIncusServer)
			profile := &api.Profile{ProfilePut: api.ProfilePut{Config: map[string]string{"limits.cpu": "2", "limits.memory": tc.memory}, Devices: map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "test", "size": "10GiB"}}}}
			cli := &readinessServer{MockIncusServer: base, profile: profile, profileErr: tc.profileErr,
				server: &api.Server{ServerUntrusted: api.ServerUntrusted{Auth: "trusted"}, Environment: api.ServerEnvironment{Driver: "qemu", KernelArchitecture: "x86_64", ServerVersion: "6.0.7"}},
				state:  &api.ProjectState{Resources: map[string]api.ProjectStateResource{"cpu": {Limit: -1}, "memory": {Limit: -1}, "disk": {Limit: -1}, "instances": {Limit: -1}, "virtual-machines": {Limit: -1}}}}

			if tc.image == strings.Repeat("a", 64) {
				base.On("GetProject", "test").Return(&api.Project{Name: "test"}, "", nil).Once()
				base.On("GetProfileNames").Return([]string{"runner"}, nil).Once()
			}
			if tc.want == "create permission, KVM health, and host-wide capacity are not verified" {
				base.On("GetImage", strings.Repeat("a", 64)).Return(&api.Image{Fingerprint: strings.Repeat("a", 64), Architecture: "x86_64", Type: "virtual-machine"}, "", nil).Once()
			}
			p := &Incus{cfg: &config.Incus{ProjectName: "test"}, cli: cli, imageManager: &image{}, admissionBackendIdentity: strings.Repeat("c", 64)}
			q := req
			q.Image = tc.image
			got := p.Readiness(context.Background(), q)
			require.Equal(t, tc.name == "valid metadata", got.Ready)
			require.Equal(t, tc.want, got.Reason)
			require.Equal(t, q, got.Request)
			require.LessOrEqual(t, got.ValidUntil.Sub(got.CheckedAt), 15*time.Second)
			base.AssertExpectations(t)
		})
	}
}

func TestReadinessUnknownAndDeniedAreNotReady(t *testing.T) {
	req := ReadinessRequest{Version: 1, RequestID: "r", Generation: "g", Domain: "d", Project: "p", Image: strings.Repeat("a", 64), Flavor: "f", OSArch: "amd64", InstanceType: "virtual-machine", Resources: ReadinessResources{CPU: 1, MemoryBytes: 1, DiskBytes: 1, VMSlots: 1}}
	validServer := &api.Server{ServerUntrusted: api.ServerUntrusted{Auth: "trusted"}, Environment: api.ServerEnvironment{Driver: "qemu", KernelArchitecture: "x86_64", ServerVersion: "6.0.7"}}
	for _, tc := range []struct {
		name        string
		server      *api.Server
		serverErr   error
		state       *api.ProjectState
		stateErr    error
		projectRead bool
	}{
		{name: "server denied", serverErr: errors.New("secret")},
		{name: "untrusted", server: &api.Server{}},
		{name: "project state denied", server: validServer, stateErr: errors.New("secret"), projectRead: true},
		{name: "unknown quota", server: validServer, state: &api.ProjectState{}, projectRead: true},
		{name: "full quota", server: validServer, state: &api.ProjectState{Resources: map[string]api.ProjectStateResource{"cpu": {Limit: 0}, "memory": {Limit: -1}, "disk": {Limit: -1}, "instances": {Limit: -1}, "virtual-machines": {Limit: -1}}}, projectRead: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := new(MockIncusServer)
			if tc.projectRead {
				base.On("GetProject", "p").Return(&api.Project{Name: "p"}, "", nil).Once()
			}
			cli := &readinessServer{MockIncusServer: base, server: tc.server, serverErr: tc.serverErr, state: tc.state, stateErr: tc.stateErr}
			p := &Incus{cfg: &config.Incus{ProjectName: "p"}, cli: cli, admissionBackendIdentity: strings.Repeat("c", 64)}
			got := p.Readiness(context.Background(), req)
			require.False(t, got.Ready)
			require.NotContains(t, got.Reason, "secret")
			base.AssertExpectations(t)
		})
	}
}

func TestAdmittedCreateSnapshotsAndEnforcesProfile(t *testing.T) {
	fingerprint := strings.Repeat("b", 64)
	resources := AdmittedResourceSpec{CPU: 2, MemoryBytes: 3 << 30, DiskBytes: 11 << 30, VMSlots: 1, BackendIdentity: strings.Repeat("c", 64)}
	for _, tc := range []struct {
		name        string
		cpu         string
		extraDisk   bool
		extraDevice string
		fail        bool
	}{
		{name: "exact", cpu: "2"},
		{name: "smaller is rejected", cpu: "1", fail: true},
		{name: "larger is rejected", cpu: "3", fail: true},
		{name: "extra disk rejected", cpu: "2", extraDisk: true, fail: true},
		{name: "GPU rejected", cpu: "2", extraDevice: "gpu", fail: true},
		{name: "PCI rejected", cpu: "2", extraDevice: "pci", fail: true},
		{name: "profile mask rejected", cpu: "2", extraDevice: "none", fail: true},
		{name: "physical NIC rejected", cpu: "2", extraDevice: "physical-nic", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := new(MockIncusServer)
			root := map[string]string{"type": "disk", "path": "/", "pool": "synthetic", "size": "11GiB"}
			devices := map[string]map[string]string{"root": root, "network": {"type": "nic", "network": "synthetic"}}
			if tc.extraDisk {
				devices["extra"] = map[string]string{"type": "disk", "path": "/data", "pool": "synthetic", "size": "1GiB"}
			}
			if tc.extraDevice != "" {
				devices["unmetered"] = map[string]string{"type": tc.extraDevice}
				if tc.extraDevice == "physical-nic" {
					devices["unmetered"] = map[string]string{"type": "nic", "nictype": "physical"}
				}
			}
			cli := &readinessServer{MockIncusServer: base, profile: &api.Profile{ProfilePut: api.ProfilePut{Config: map[string]string{"limits.cpu": tc.cpu, "limits.memory": "3GiB"}, Devices: devices}}}
			base.On("GetProfileNames").Return([]string{"worker"}, nil).Twice()
			if !tc.fail {
				base.On("GetImage", fingerprint).Return(&api.Image{Fingerprint: fingerprint, Architecture: "x86_64", Type: "virtual-machine"}, "", nil).Once()
			}
			p := &Incus{cfg: &config.Incus{ProjectName: "test"}, cli: cli, imageManager: &image{}, admissionBackendIdentity: strings.Repeat("c", 64)}
			oldFetch, oldConfig := DefaultToolFetch, DefaultGetCloudconfig
			DefaultToolFetch = func(_ commonParams.OSType, _ commonParams.OSArch, _ []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error) {
				return commonParams.RunnerApplicationDownload{}, nil
			}
			DefaultGetCloudconfig = func(_ commonParams.BootstrapInstance, _ commonParams.RunnerApplicationDownload, _ string) (string, error) {
				return "cloud", nil
			}
			defer func() { DefaultToolFetch, DefaultGetCloudconfig = oldFetch, oldConfig }()
			got, err := p.getCreateInstanceArgs(context.Background(), commonParams.BootstrapInstance{Name: "runner", Flavor: "worker", Image: fingerprint, OSArch: commonParams.Amd64}, extraSpecs{AdmissionResources: &resources})
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got.Profiles)
				require.Empty(t, got.Profiles)
				wire, err := json.Marshal(got)
				require.NoError(t, err)
				require.Contains(t, string(wire), `"profiles":[]`)
				require.Equal(t, "2", got.Config["limits.cpu"])
				require.Equal(t, "3221225472", got.Config["limits.memory"])
				require.Equal(t, "11811160064", got.Devices["root"]["size"])
				root["size"] = "99GiB"
				require.Equal(t, "11811160064", got.Devices["root"]["size"])
			}
			base.AssertExpectations(t)
		})
	}
}

func TestAdmissionResourcesExtraSpecs(t *testing.T) {
	data := json.RawMessage(`{"admission_resources":{"cpu":2,"memory_bytes":3221225472,"disk_bytes":11811160064,"vm_slots":1,"backend_identity":"identity"}}`)
	got, err := parseExtraSpecsFromBootstrapParams(commonParams.BootstrapInstance{ExtraSpecs: data})
	require.NoError(t, err)
	require.NotNil(t, got.AdmissionResources)
	require.Equal(t, AdmittedResourceSpec{CPU: 2, MemoryBytes: 3 << 30, DiskBytes: 11 << 30, VMSlots: 1, BackendIdentity: "identity"}, *got.AdmissionResources)
	_, err = parseExtraSpecsFromBootstrapParams(commonParams.BootstrapInstance{ExtraSpecs: json.RawMessage(`{"admission_resources":{"cpu":2,"memory_bytes":3,"disk_bytes":4,"vm_slots":1,"unknown":5}}`)})
	require.Error(t, err)
}
