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

package executionv011

import (
	"context"

	"github.com/cloudbase/garm-provider-common/execution/common"
)

// ExternalProvider defines an interface that external providers need to implement.
// This is very similar to the common.Provider interface, and was redefined here to
// decouple it, in case it may diverge from native providers.
type ExternalProvider interface {
	// The common ExternalProvider interface
	common.ExternalProvider
	// GetSupportedInterfaceVersions will return the supported interface versions.
	GetSupportedInterfaceVersions(ctx context.Context) []string
	// ValidatePoolInfo will validate the pool info and return an error if it's not valid.
	ValidatePoolInfo(ctx context.Context, image string, flavor string, providerConfig string, extraspecs string) error
	// GetConfigJSONSchema will return the JSON schema for the provider's configuration.
	GetConfigJSONSchema(ctx context.Context) (string, error)
	// GetExtraSpecsJSONSchema will return the JSON schema for the provider's extra specs.
	GetExtraSpecsJSONSchema(ctx context.Context) (string, error)
}
