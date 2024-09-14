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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudbase/garm-provider-common/execution"
	commonExecution "github.com/cloudbase/garm-provider-common/execution/common"

	"github.com/cloudbase/garm-provider-incus/provider"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()

	executionEnv, err := execution.GetEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	prov, err := provider.NewIncusProvider(executionEnv.ProviderConfigFile, executionEnv.ControllerID)
	if err != nil {
		log.Fatal(err)
	}

	result, err := executionEnv.Run(ctx, prov)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run command: %s", err)
		os.Exit(commonExecution.ResolveErrorToExitCode(err))
	}
	if len(result) > 0 {
		fmt.Fprint(os.Stdout, result)
	}
}
