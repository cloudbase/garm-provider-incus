// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL

package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lxc/incus/v7/shared/api"
)

const AbsenceContract = "incus-single-node-absence-v1"

type AbsenceRequest struct {
	Version                 int    `json:"version"`
	RequestID               string `json:"request_id"`
	Generation              string `json:"generation"`
	Domain                  string `json:"domain"`
	Project                 string `json:"project"`
	InstanceName            string `json:"instance_name"`
	ExpectedBackendIdentity string `json:"expected_backend_identity"`
}

type AbsenceResponse struct {
	Request         AbsenceRequest `json:"request"`
	Absent          bool           `json:"absent"`
	CheckedAt       time.Time      `json:"checked_at"`
	ValidUntil      time.Time      `json:"valid_until"`
	Contract        string         `json:"contract"`
	BackendIdentity string         `json:"backend_identity"`
	Reason          string         `json:"reason,omitempty"`
}

type serverReader interface {
	GetServer() (*api.Server, string, error)
}

func (r AbsenceRequest) Validate() error {
	if r.Version != 1 || r.RequestID == "" || r.Generation == "" || r.Domain == "" || r.Project == "" || r.InstanceName == "" || r.ExpectedBackendIdentity == "" {
		return fmt.Errorf("invalid absence request")
	}
	return nil
}

// Absence observes only an exact 404 on a trusted, non-clustered Incus server
// in the configured project. The caller must prove the create completed and no
// create/retry is in flight and must serialize delete and name reuse.
func (l *Incus) Absence(ctx context.Context, req AbsenceRequest) AbsenceResponse {
	now := time.Now().UTC()
	result := AbsenceResponse{Request: req, CheckedAt: now, ValidUntil: now.Add(15 * time.Second), Contract: AbsenceContract, Reason: "absence unverified"}
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
	_, _, err = cli.GetInstanceFull(req.InstanceName)
	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) && ctx.Err() == nil {
		result.Absent = true
		result.Reason = ""
	}
	return result
}
