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
	"testing"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCases = []struct {
	name           string
	input          json.RawMessage
	expectedOutput extraSpecs
	errString      string
}{
	{
		name:  "full specs",
		input: json.RawMessage(`{"disable_updates": true, "extra_packages": ["package1", "package2"], "enable_boot_debug": true, "runner_install_template": "IyEvYmluL2Jhc2gKZWNobyBJbnN0YWxsaW5nIHJ1bm5lci4uLg==", "pre_install_scripts": {"setup.sh": "IyEvYmluL2Jhc2gKZWNobyBTZXR1cCBzY3JpcHQuLi4="}, "extra_context": {"key": "value"}}`),
		expectedOutput: extraSpecs{
			DisableUpdates:  true,
			ExtraPackages:   []string{"package1", "package2"},
			EnableBootDebug: true,
			CloudConfigSpec: cloudconfig.CloudConfigSpec{
				RunnerInstallTemplate: []byte("#!/bin/bash\necho Installing runner..."),
				PreInstallScripts: map[string][]byte{
					"setup.sh": []byte("#!/bin/bash\necho Setup script..."),
				},
				ExtraContext: map[string]string{"key": "value"},
			},
		},
		errString: "",
	},
	{
		name:  "specs just with disable_updates",
		input: json.RawMessage(`{"disable_updates": true}`),
		expectedOutput: extraSpecs{
			DisableUpdates: true,
		},
		errString: "",
	},
	{
		name:  "specs just with extra_packages",
		input: json.RawMessage(`{"extra_packages": ["package1", "package2"]}`),
		expectedOutput: extraSpecs{
			ExtraPackages: []string{"package1", "package2"},
		},
		errString: "",
	},
	{
		name:  "specs just with enable_boot_debug",
		input: json.RawMessage(`{"enable_boot_debug": true}`),
		expectedOutput: extraSpecs{
			EnableBootDebug: true,
		},
		errString: "",
	},
	{
		name:  "specs just with runner_install_template",
		input: json.RawMessage(`{"runner_install_template": "IyEvYmluL2Jhc2gKZWNobyBJbnN0YWxsaW5nIHJ1bm5lci4uLg=="}`),
		expectedOutput: extraSpecs{
			CloudConfigSpec: cloudconfig.CloudConfigSpec{
				RunnerInstallTemplate: []byte("#!/bin/bash\necho Installing runner..."),
			},
		},
		errString: "",
	},
	{
		name:  "specs just with pre_install_scripts",
		input: json.RawMessage(`{"pre_install_scripts": {"setup.sh": "IyEvYmluL2Jhc2gKZWNobyBTZXR1cCBzY3JpcHQuLi4="}}`),
		expectedOutput: extraSpecs{
			CloudConfigSpec: cloudconfig.CloudConfigSpec{
				PreInstallScripts: map[string][]byte{
					"setup.sh": []byte("#!/bin/bash\necho Setup script..."),
				},
			},
		},
		errString: "",
	},
	{
		name:  "specs just with extra_context",
		input: json.RawMessage(`{"extra_context": {"key": "value"}}`),
		expectedOutput: extraSpecs{
			CloudConfigSpec: cloudconfig.CloudConfigSpec{
				ExtraContext: map[string]string{"key": "value"},
			},
		},
		errString: "",
	},
	{
		name:           "empty specs",
		input:          json.RawMessage(`{}`),
		expectedOutput: extraSpecs{},
		errString:      "",
	},
	{
		name:           "invalid json",
		input:          json.RawMessage(`{"disable_updates": true, "extra_packages": ["package1", "package2", "enable_boot_debug": true}`),
		expectedOutput: extraSpecs{},
		errString:      "failed to validate extra specs",
	},
	{
		name:           "invalid input for disable_updates - wrong data type",
		input:          json.RawMessage(`{"disable_updates": "true"}`),
		expectedOutput: extraSpecs{},
		errString:      "schema validation failed: [disable_updates: Invalid type. Expected: boolean, given: string]",
	},
	{
		name:           "invalid input for extra_packages - wrong data type",
		input:          json.RawMessage(`{"extra_packages": "package1"}`),
		expectedOutput: extraSpecs{},
		errString:      "schema validation failed: [extra_packages: Invalid type. Expected: array, given: string]",
	},
	{
		name:           "invalid input for enable_boot_debug - wrong data type",
		input:          json.RawMessage(`{"enable_boot_debug": "true"}`),
		expectedOutput: extraSpecs{},
		errString:      "schema validation failed: [enable_boot_debug: Invalid type. Expected: boolean, given: string]",
	},
	{
		name:           "invalid input for runner_install_template - wrong data type",
		input:          json.RawMessage(`{"runner_install_template": true}`),
		expectedOutput: extraSpecs{},
		errString:      "schema validation failed: [runner_install_template: Invalid type. Expected: string, given: boolean]",
	},
	{
		name:           "invalid input for pre_install_scripts - wrong data type",
		input:          json.RawMessage(`{"pre_install_scripts": "setup.sh"}`),
		expectedOutput: extraSpecs{},
		errString:      "schema validation failed: [pre_install_scripts: Invalid type. Expected: object, given: string]",
	},
	{
		name:           "invalid input for extra_context - wrong data type",
		input:          json.RawMessage(`{"extra_context": ["key", "value"]}`),
		expectedOutput: extraSpecs{},
		errString:      "schema validation failed: [extra_context: Invalid type. Expected: object, given: array]",
	},
	{
		name:           "invalid input - additional property",
		input:          json.RawMessage(`{"additional_property": true}`),
		expectedOutput: extraSpecs{},
		errString:      "Additional property additional_property is not allowed",
	},
}

func TestParseExtraSpecsFromBootstrapParams(t *testing.T) {
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExtraSpecsFromBootstrapParams(params.BootstrapInstance{ExtraSpecs: tt.input})
			assert.Equal(t, tt.expectedOutput, got)
			if tt.errString != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
