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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	if os.Getenv("GARM_COMMAND") == "inventory_v1" {
		if err := runInventory(ctx, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "inventory command failed")
			os.Exit(1)
		}
		return
	}
	if os.Getenv("GARM_COMMAND") == "delete_admitted_v1" {
		if err := runAdmittedDelete(ctx, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "admitted delete command failed")
			os.Exit(1)
		}
		return
	}
	if os.Getenv("GARM_COMMAND") == "absence_v1" {
		if err := runAbsence(ctx, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "absence command failed")
			os.Exit(1)
		}
		return
	}
	if os.Getenv("GARM_COMMAND") == "readiness_v1" {
		if err := runReadiness(ctx, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "readiness command failed")
			os.Exit(1)
		}
		return
	}

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
		fmt.Print(result)
	}
}

// runReadiness uses a bounded input and deadline; errors do not disclose
// provider configuration or credentials.
func runReadiness(ctx context.Context, input io.Reader, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	type outcome struct {
		data []byte
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		var buf bytes.Buffer
		err := runReadinessRequest(ctx, input, &buf)
		done <- outcome{data: buf.Bytes(), err: err}
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("readiness deadline exceeded")
	case result := <-done:
		if result.err != nil {
			return result.err
		}
		_, err := output.Write(result.data)
		return err
	}
}

func runReadinessRequest(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("invalid readiness input")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req provider.ReadinessRequest
	if err := dec.Decode(&req); err != nil {
		return err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid trailing input")
	}
	if err := req.Validate(); err != nil {
		return err
	}
	results := make(chan provider.ReadinessResponse, 1)
	go func() {
		result := provider.ReadinessResponse{Request: req, CheckedAt: time.Now().UTC(), Reason: "provider configuration unavailable"}
		result.ValidUntil = result.CheckedAt.Add(15 * time.Second)
		p, err := provider.NewIncusProvider(os.Getenv("GARM_PROVIDER_CONFIG_FILE"), os.Getenv("GARM_CONTROLLER_ID"))
		if err == nil {
			if impl, ok := p.(*provider.Incus); ok {
				result = impl.Readiness(ctx, req)
			}
		}
		results <- result
	}()
	select {
	case result := <-results:
		return json.NewEncoder(output).Encode(result)
	case <-ctx.Done():
		return fmt.Errorf("readiness deadline exceeded")
	}
}

// runAbsence caps input, execution time and emitted diagnostics.
func runAbsence(ctx context.Context, input io.Reader, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	type outcome struct {
		data []byte
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		var buf bytes.Buffer
		err := runAbsenceRequest(ctx, input, &buf)
		done <- outcome{buf.Bytes(), err}
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("absence deadline exceeded")
	case result := <-done:
		if result.err != nil {
			return result.err
		}
		_, err := output.Write(result.data)
		return err
	}
}
func runAbsenceRequest(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("invalid absence input")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req provider.AbsenceRequest
	if err := dec.Decode(&req); err != nil {
		return err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid trailing input")
	}
	if err := req.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	result := provider.AbsenceResponse{Request: req, CheckedAt: now, ValidUntil: now.Add(15 * time.Second), Contract: provider.AbsenceContract, Reason: "provider configuration unavailable"}
	p, err := provider.NewIncusProvider(os.Getenv("GARM_PROVIDER_CONFIG_FILE"), os.Getenv("GARM_CONTROLLER_ID"))
	if err == nil {
		if impl, ok := p.(*provider.Incus); ok {
			result = impl.Absence(ctx, req)
		}
	}
	if ctx.Err() != nil {
		return fmt.Errorf("absence deadline exceeded")
	}
	return json.NewEncoder(output).Encode(result)
}

func runAdmittedDelete(ctx context.Context, input io.Reader, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	type outcome struct {
		data []byte
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		var buf bytes.Buffer
		err := runAdmittedDeleteRequest(ctx, input, &buf)
		done <- outcome{buf.Bytes(), err}
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("admitted delete deadline exceeded")
	case result := <-done:
		if result.err != nil {
			return result.err
		}
		_, err := output.Write(result.data)
		return err
	}
}
func runAdmittedDeleteRequest(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("invalid admitted delete input")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req provider.AdmittedDeleteRequest
	if err := dec.Decode(&req); err != nil {
		return err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid trailing input")
	}
	if err := req.Validate(); err != nil {
		return err
	}
	result := provider.AdmittedDeleteResponse{Request: req, Reason: "provider configuration unavailable"}
	p, err := provider.NewIncusProvider(os.Getenv("GARM_PROVIDER_CONFIG_FILE"), os.Getenv("GARM_CONTROLLER_ID"))
	if err == nil {
		if impl, ok := p.(*provider.Incus); ok {
			result = impl.AdmittedDelete(ctx, req)
		}
	}
	if ctx.Err() != nil {
		return fmt.Errorf("admitted delete deadline exceeded")
	}
	return json.NewEncoder(output).Encode(result)
}

func runInventory(ctx context.Context, input io.Reader, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	type outcome struct {
		data []byte
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		var buf bytes.Buffer
		err := runInventoryRequest(ctx, input, &buf)
		done <- outcome{buf.Bytes(), err}
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("inventory deadline exceeded")
	case result := <-done:
		if result.err != nil {
			return result.err
		}
		_, err := output.Write(result.data)
		return err
	}
}
func runInventoryRequest(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("invalid inventory input")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req provider.InventoryRequest
	if err := dec.Decode(&req); err != nil {
		return err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid trailing input")
	}
	if err := req.Validate(); err != nil {
		return err
	}
	result := provider.InventoryResponse{Request: req, Instances: []provider.InventoryInstance{}, Reason: "provider configuration unavailable"}
	p, err := provider.NewIncusProvider(os.Getenv("GARM_PROVIDER_CONFIG_FILE"), os.Getenv("GARM_CONTROLLER_ID"))
	if err == nil {
		if impl, ok := p.(*provider.Incus); ok {
			result = impl.Inventory(ctx, req)
		}
	}
	if ctx.Err() != nil {
		return fmt.Errorf("inventory deadline exceeded")
	}
	return json.NewEncoder(output).Encode(result)
}
