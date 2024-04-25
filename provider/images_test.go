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
	"fmt"
	"testing"

	"github.com/cloudbase/garm-provider-incus/config"
	"github.com/lxc/incus/shared/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageNa(t *testing.T) {
	tests := []struct {
		name              string
		image             *image
		imageName         string
		expectedRemote    config.IncusImageRemote
		expectedImageName string
		errString         string
	}{
		{
			name: "image with remote",
			image: &image{
				remotes: map[string]config.IncusImageRemote{
					"remote1": {},
				},
			},
			imageName:         "remote1:image1",
			expectedRemote:    config.IncusImageRemote{},
			expectedImageName: "image1",
			errString:         "",
		},
		{
			name: "image without remote",
			image: &image{
				remotes: map[string]config.IncusImageRemote{
					"remote1": {},
				},
			},
			imageName:         "image1",
			expectedRemote:    config.IncusImageRemote{},
			expectedImageName: "",
			errString:         "image does not include a remote",
		},
		{
			name: "image with invalid remote",
			image: &image{
				remotes: map[string]config.IncusImageRemote{
					"remote1": {},
				},
			},
			imageName:         "invalid:image1",
			expectedRemote:    config.IncusImageRemote{},
			expectedImageName: "",
			errString:         "could not find invalid:image1 in map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote, imageName, err := tt.image.parseImageName(tt.imageName)
			if tt.errString == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.errString)
			}
			assert.Equal(t, tt.expectedRemote, remote)
			assert.Equal(t, tt.expectedImageName, imageName)
		})
	}
}

func TestGetLocalImageByAlias_Success(t *testing.T) {
	cli := new(MockIncusServer)
	i := &image{
		remotes: map[string]config.IncusImageRemote{
			"remote1": {},
		},
	}
	imageName := "image1"
	imageType := config.IncusImageType("container")
	arch := "amd64"
	expectedImage := &api.Image{
		Fingerprint: "fingerprint",
	}
	aliases := map[string]*api.ImageAliasesEntry{
		"amd64": {},
	}

	cli.On("GetImageAliasArchitectures", "container", imageName).Return(aliases, nil)
	cli.On("GetImage", aliases[arch].Target).Return(expectedImage, "", nil)

	image, err := i.getLocalImageByAlias(imageName, imageType, arch, cli)
	require.NoError(t, err)
	assert.Equal(t, expectedImage, image)
}

func TestGetLocalImageByAlias_Error(t *testing.T) {
	cli := new(MockIncusServer)
	i := &image{
		remotes: map[string]config.IncusImageRemote{
			"remote1": {},
		},
	}
	imageName := "image1"
	imageType := config.IncusImageType("container")
	arch := "amd64"
	aliases := map[string]*api.ImageAliasesEntry{
		"amd64": {},
	}

	cli.On("GetImageAliasArchitectures", "container", imageName).Return(aliases, fmt.Errorf("error"))

	image, err := i.getLocalImageByAlias(imageName, imageType, arch, cli)
	require.Error(t, err)
	assert.Nil(t, image)
	cli.AssertExpectations(t)
}

func TestGetInstanceSource_Success(t *testing.T) {
	cli := new(MockIncusServer)
	i := &image{
		remotes: map[string]config.IncusImageRemote{
			"remote1": {},
		},
	}
	imageName := "image1"
	imageType := config.IncusImageType("container")
	arch := "amd64"
	aliases := map[string]*api.ImageAliasesEntry{
		"amd64": {},
	}
	expectedImage := &api.Image{
		Fingerprint: "fingerprint",
	}

	cli.On("GetImageAliasArchitectures", "container", imageName).Return(aliases, nil)
	cli.On("GetImage", aliases[arch].Target).Return(expectedImage, "", nil)

	instanceSource, err := i.getInstanceSource(imageName, imageType, arch, cli)
	require.NoError(t, err)
	assert.Equal(t, api.InstanceSource{
		Type:        "image",
		Fingerprint: "fingerprint",
	}, instanceSource)
	cli.AssertExpectations(t)
}

func TestGetInstanceSource_Error(t *testing.T) {
	cli := new(MockIncusServer)
	i := &image{
		remotes: map[string]config.IncusImageRemote{
			"remote1": {},
		},
	}
	imageName := "image1"
	imageType := config.IncusImageType("container")
	arch := "amd64"
	aliases := map[string]*api.ImageAliasesEntry{
		"amd64": {},
	}

	cli.On("GetImageAliasArchitectures", "container", imageName).Return(aliases, fmt.Errorf("error"))

	instanceSource, err := i.getInstanceSource(imageName, imageType, arch, cli)
	require.Error(t, err)
	assert.Equal(t, api.InstanceSource{}, instanceSource)
	cli.AssertExpectations(t)
}
