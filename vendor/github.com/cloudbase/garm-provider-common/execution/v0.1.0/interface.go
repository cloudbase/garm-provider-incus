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

package executionv010

import (
	common "github.com/cloudbase/garm-provider-common/execution/common"
)

// ExternalProvider defines an interface that external providers need to implement.
// This is very similar to the common.Provider interface, and was redefined here to
// decouple it, in case it may diverge from native providers.
type ExternalProvider interface {
	// The common ExternalProvider interface
	common.ExternalProvider
}
