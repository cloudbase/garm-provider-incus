// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL
package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/lxc/incus/v7/shared/api"
)

type InventoryRequest struct {
	Version                 int    `json:"version"`
	RequestID               string `json:"request_id"`
	ExpectedBackendIdentity string `json:"expected_backend_identity"`
	Project                 string `json:"project"`
}
type InventoryInstance struct {
	Name       string `json:"name"`
	ProviderID string `json:"provider_id"`
}
type InventoryResponse struct {
	Request         InventoryRequest    `json:"request"`
	BackendIdentity string              `json:"backend_identity"`
	Instances       []InventoryInstance `json:"instances"`
	Complete        bool                `json:"complete"`
	Reason          string              `json:"reason,omitempty"`
}

func (r InventoryRequest) Validate() error {
	if r.Version != 1 || r.RequestID == "" || r.ExpectedBackendIdentity == "" || r.Project == "" {
		return fmt.Errorf("invalid inventory request")
	}
	return nil
}

// Inventory lists all guests carrying this controller ID in the configured
// project. Other controllers and untagged guests are not included.
func (l *Incus) Inventory(ctx context.Context, req InventoryRequest) InventoryResponse {
	result := InventoryResponse{Request: req, Instances: []InventoryInstance{}, Reason: "inventory unavailable"}
	if req.Validate() != nil || req.Project != projectName(l.cfg) || l.controllerID == "" {
		return result
	}
	identity, err := l.admissionBackend(ctx)
	if err != nil || identity != req.ExpectedBackendIdentity {
		return result
	}
	result.BackendIdentity = identity
	cli, err := l.getCLI(ctx)
	if err != nil {
		return result
	}
	reader, ok := cli.(serverReader)
	if !ok {
		return result
	}
	server, _, err := reader.GetServer()
	if err != nil || server == nil || server.Auth != "trusted" || server.Public || server.Environment.ServerClustered || (server.Environment.ServerVersion != "6.0" && !strings.HasPrefix(server.Environment.ServerVersion, "6.0.")) {
		return result
	}
	guests, err := cli.GetInstancesFull(api.InstanceTypeAny)
	if err != nil || ctx.Err() != nil {
		return result
	}
	for _, guest := range guests {
		if guest.ExpandedConfig[controllerIDKeyName] != l.controllerID {
			continue
		}
		if guest.Name == "" {
			return result
		}
		result.Instances = append(result.Instances, InventoryInstance{Name: guest.Name, ProviderID: guest.Name})
	}
	result.Complete = true
	result.Reason = ""
	return result
}
