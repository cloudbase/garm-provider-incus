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
	"github.com/lxc/incus/v7/shared/api"
	"github.com/stretchr/testify/assert"
)

func TestIncusInstanceToAPIInstance(t *testing.T) {
	instance := &api.InstanceFull{
		Instance: api.Instance{
			Name: "test-instance",
			InstancePut: api.InstancePut{
				Architecture: "x86_64",
			},
			ExpandedConfig: map[string]string{
				"image.os":      "ubuntu",
				osTypeKeyName:   "linux",
				"image.release": "20.04",
			},
		},
		State: &api.InstanceState{
			Network: map[string]api.InstanceStateNetwork{
				"eth0": {
					Addresses: []api.InstanceStateNetworkAddress{
						{
							Family:  "inet",
							Address: "10.10.0.4",
							Netmask: "24",
							Scope:   "global",
						},
					},
				},
			},
			Status: "Running",
		},
	}
	expectedOutput := commonParams.ProviderInstance{
		OSArch:     "amd64",
		ProviderID: "test-instance",
		Name:       "test-instance",
		OSType:     "linux",
		OSName:     "ubuntu",
		OSVersion:  "20.04",
		Addresses: []commonParams.Address{
			{
				Address: "10.10.0.4",
				Type:    "public",
			},
		},
		Status: "running",
	}

	apiInstance := incusInstanceToAPIInstance(instance)
	assert.Equal(t, expectedOutput, apiInstance)
}

func TestGetClientFromConfig(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		cfg       *config.Incus
		errString string
	}{
		{
			name:      "Nil config",
			cfg:       nil,
			errString: "no Incus configuration found",
		},
		{
			name:      "empty config",
			cfg:       &config.Incus{},
			errString: "no URL or UnixSocket specified",
		},
		{
			name: "invalid TSLServerCert",
			cfg: &config.Incus{
				URL:           "https://localhost:8443",
				TLSServerCert: "invalid",
			},
			errString: "reading TLSServerCert",
		},
		{
			name: "invalid TLSCA",
			cfg: &config.Incus{
				URL:   "https://localhost:8443",
				TLSCA: "invalid",
			},
			errString: "reading TLSCA",
		},
		{
			name: "invalid ClientCertificate",
			cfg: &config.Incus{
				URL:               "https://localhost:8443",
				ClientCertificate: "invalid",
			},
			errString: "reading ClientCertificate",
		},
		{
			name: "invalid ClientKey",
			cfg: &config.Incus{
				URL:       "https://localhost:8443",
				ClientKey: "invalid",
			},
			errString: "reading ClientKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := getClientFromConfig(ctx, tt.cfg)
			if tt.errString != "" {
				assert.ErrorContains(t, err, tt.errString)
				assert.Nil(t, output)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, output)
			}
		})
	}

}

// TestResolveArchitecture checks that every architecture GARM supports maps
// to its Incus name, and that the Incus name maps back to the same
// architecture, so instances listed from Incus report the arch they were
// created with.
func TestResolveArchitecture(t *testing.T) {
	tests := []struct {
		osArch commonParams.OSArch
		want   string
	}{
		{osArch: "amd64", want: "x86_64"},
		{osArch: "arm64", want: "aarch64"},
		{osArch: "arm", want: "armv7l"},
		{osArch: "ppc64le", want: "ppc64le"},
		{osArch: "s390x", want: "s390x"},
		{osArch: "riscv64", want: "riscv64"},
	}

	for _, tt := range tests {
		t.Run(string(tt.osArch), func(t *testing.T) {
			got, err := resolveArchitecture(tt.osArch)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.osArch, incusToConfigArch[got])
		})
	}

	t.Run("empty defaults to amd64", func(t *testing.T) {
		got, err := resolveArchitecture("")
		assert.NoError(t, err)
		assert.Equal(t, "x86_64", got)
	})
}

// TestResolveArchitectureErrors checks that architectures with no Incus
// mapping are rejected rather than passed through to Incus.
func TestResolveArchitectureErrors(t *testing.T) {
	tests := []commonParams.OSArch{"i386", "mips64", "x86_64"}

	for _, osArch := range tests {
		t.Run(string(osArch), func(t *testing.T) {
			_, err := resolveArchitecture(osArch)
			assert.ErrorContains(t, err, "is not supported")
		})
	}
}
