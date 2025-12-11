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
	"encoding/json"
	"fmt"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

type extraSpecs struct {
	ExtraPackages         []string `json:"extra_packages,omitempty" jsonschema:"description=A list of packages that cloud-init should install on the instance."`
	DisableUpdates        bool     `json:"disable_updates,omitempty" jsonschema:"description=Whether to disable updates when cloud-init comes online."`
	EnableBootDebug       bool     `json:"enable_boot_debug,omitempty" jsonschema:"description=Allows providers to set the -x flag in the runner install script."`
	UseLowerCaseHostnames bool     `json:"use_lowercase_hostnames,omitempty" jsonschema:"description=Instructs the provider to make the hostname of the runner all lowercase."`
	cloudconfig.CloudConfigSpec
}

func jsonSchemaValidation(schema json.RawMessage) error {
	jsonSchema := generateJSONSchema()
	schemaLoader := gojsonschema.NewGoLoader(jsonSchema)
	extraSpecsLoader := gojsonschema.NewBytesLoader(schema)
	result, err := gojsonschema.Validate(schemaLoader, extraSpecsLoader)
	if err != nil {
		return fmt.Errorf("failed to validate schema: %w", err)
	}
	if !result.Valid() {
		return fmt.Errorf("schema validation failed: %s", result.Errors())
	}
	return nil
}

func parseExtraSpecsFromBootstrapParams(bootstrapParams commonParams.BootstrapInstance) (extraSpecs, error) {
	specs := extraSpecs{}
	if bootstrapParams.ExtraSpecs == nil {
		return specs, nil
	}

	if err := jsonSchemaValidation(bootstrapParams.ExtraSpecs); err != nil {
		return specs, fmt.Errorf("failed to validate extra specs: %w", err)
	}

	if err := json.Unmarshal(bootstrapParams.ExtraSpecs, &specs); err != nil {
		return specs, errors.Wrap(err, "unmarshaling extra specs")
	}
	return specs, nil
}
