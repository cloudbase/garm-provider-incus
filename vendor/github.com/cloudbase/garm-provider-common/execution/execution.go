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

package execution

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudbase/garm-provider-common/execution/common"
	executionv010 "github.com/cloudbase/garm-provider-common/execution/v0.1.0"
	executionv011 "github.com/cloudbase/garm-provider-common/execution/v0.1.1"
)

type Environment struct {
	EnvironmentV010    executionv010.EnvironmentV010
	EnvironmentV011    executionv011.EnvironmentV011
	InterfaceVersion   string
	ProviderConfigFile string
	ControllerID       string
}

func GetEnvironment() (Environment, error) {
	interfaceVersion := os.Getenv("GARM_INTERFACE_VERSION")

	switch interfaceVersion {
	case common.Version010, "":
		env, err := executionv010.GetEnvironment()
		if err != nil {
			return Environment{}, err
		}
		return Environment{
			EnvironmentV010:    env,
			ProviderConfigFile: env.ProviderConfigFile,
			ControllerID:       env.ControllerID,
			InterfaceVersion:   interfaceVersion,
		}, nil
	case common.Version011:
		env, err := executionv011.GetEnvironment()
		if err != nil {
			return Environment{}, err
		}
		return Environment{
			EnvironmentV011:    env,
			ProviderConfigFile: env.ProviderConfigFile,
			ControllerID:       env.ControllerID,
			InterfaceVersion:   interfaceVersion,
		}, nil
	default:
		return Environment{}, fmt.Errorf("unsupported interface version: %s", interfaceVersion)
	}
}

func (e Environment) Run(ctx context.Context, provider interface{}) (string, error) {
	switch e.InterfaceVersion {
	case common.Version010, "":
		prov, ok := provider.(executionv010.ExternalProvider)
		if !ok {
			return "", fmt.Errorf("provider does not implement %s ExternalProvider", e.InterfaceVersion)
		}
		return e.EnvironmentV010.Run(ctx, prov)

	case common.Version011:
		prov, ok := provider.(executionv011.ExternalProvider)
		if !ok {
			return "", fmt.Errorf("provider does not implement %s ExternalProvider", e.InterfaceVersion)
		}
		return e.EnvironmentV011.Run(ctx, prov)

	default:
		return "", fmt.Errorf("unsupported interface version: %s", e.InterfaceVersion)
	}
}
