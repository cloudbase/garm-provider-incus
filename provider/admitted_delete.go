// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL
package provider

import (
	"context"
	"fmt"
	"strings"
)

type AdmittedDeleteRequest struct {
	Version                 int    `json:"version"`
	RequestID               string `json:"request_id"`
	ExpectedBackendIdentity string `json:"expected_backend_identity"`
	Project                 string `json:"project"`
	InstanceName            string `json:"instance_name"`
}
type AdmittedDeleteResponse struct {
	Request         AdmittedDeleteRequest `json:"request"`
	Deleted         bool                  `json:"deleted"`
	BackendIdentity string                `json:"backend_identity"`
	Reason          string                `json:"reason,omitempty"`
}

func (r AdmittedDeleteRequest) Validate() error {
	if r.Version != 1 || r.RequestID == "" || r.ExpectedBackendIdentity == "" || r.Project == "" || r.InstanceName == "" {
		return fmt.Errorf("invalid admitted delete request")
	}
	return nil
}

// AdmittedDelete performs the deletion on the same exact pinned connection
// whose identity was reserved. A failure is uncertain, not proof of absence.
func (l *Incus) AdmittedDelete(ctx context.Context, req AdmittedDeleteRequest) AdmittedDeleteResponse {
	result := AdmittedDeleteResponse{Request: req, Reason: "delete unavailable"}
	if req.Validate() != nil || req.Project != projectName(l.cfg) {
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
	if err := l.DeleteInstance(ctx, req.InstanceName); err != nil || ctx.Err() != nil {
		return result
	}
	result.Deleted = true
	result.Reason = ""
	return result
}
