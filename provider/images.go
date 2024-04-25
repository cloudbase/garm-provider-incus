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
	"strings"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-incus/config"

	"github.com/lxc/incus/shared/api"
	"github.com/pkg/errors"
)

type image struct {
	remotes map[string]config.IncusImageRemote
}

// parseImageName parses the image name that comes in from the config and returns a
// remote. If no remote is configured with the given name, an error is returned.
func (i *image) parseImageName(imageName string) (config.IncusImageRemote, string, error) {
	if !strings.Contains(imageName, ":") {
		return config.IncusImageRemote{}, "", fmt.Errorf("image does not include a remote")
	}

	details := strings.SplitN(imageName, ":", 2)
	for remoteName, val := range i.remotes {
		if remoteName == details[0] {
			return val, details[1], nil
		}
	}
	return config.IncusImageRemote{}, "", fmt.Errorf("could not find %s in %v: %w", imageName, i.remotes, runnerErrors.ErrNotFound)
}

func (i *image) getLocalImageByAlias(imageName string, imageType config.IncusImageType, arch string, cli InstanceServerInterface) (*api.Image, error) {
	aliases, err := cli.GetImageAliasArchitectures(imageType.String(), imageName)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving alias: %s", imageName)
	}

	alias, ok := aliases[arch]
	if !ok {
		return nil, fmt.Errorf("no image found for arch %s and image type %s with name %s", arch, imageType, imageName)
	}

	image, _, err := cli.GetImage(alias.Target)
	if err != nil {
		return nil, errors.Wrap(err, "fetching image details")
	}
	return image, nil
}

func (i *image) getInstanceSource(imageName string, imageType config.IncusImageType, arch string, cli InstanceServerInterface) (api.InstanceSource, error) {
	instanceSource := api.InstanceSource{
		Type: "image",
	}
	if !strings.Contains(imageName, ":") {
		// A remote was not specified, try to find an image using the imageName as
		// an alias.
		imageDetails, err := i.getLocalImageByAlias(imageName, imageType, arch, cli)
		if err != nil {
			return api.InstanceSource{}, errors.Wrap(err, "fetching image")
		}
		instanceSource.Fingerprint = imageDetails.Fingerprint
	} else {
		remote, parsedName, err := i.parseImageName(imageName)
		if err != nil {
			return api.InstanceSource{}, errors.Wrapf(err, "parsing image name: %s", imageName)
		}
		instanceSource.Alias = parsedName
		instanceSource.Server = remote.Address
		instanceSource.Protocol = string(remote.Protocol)
	}
	return instanceSource, nil
}
