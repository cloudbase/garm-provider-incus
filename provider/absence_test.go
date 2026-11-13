// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL
package provider

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudbase/garm-provider-incus/config"
	"github.com/lxc/incus/v7/shared/api"
	"github.com/stretchr/testify/require"
)

func TestAbsenceRequiresSpecificEvidence(t *testing.T) {
	req := AbsenceRequest{Version: 1, RequestID: "req", Generation: "g", Domain: "domain", Project: "p", InstanceName: "vm", ExpectedBackendIdentity: strings.Repeat("c", 64)}
	trusted := &api.Server{ServerUntrusted: api.ServerUntrusted{Auth: "trusted"}, Environment: api.ServerEnvironment{ServerVersion: "6.0.7"}}
	clustered := &api.Server{ServerUntrusted: api.ServerUntrusted{Auth: "trusted"}, Environment: api.ServerEnvironment{ServerClustered: true}}
	for _, tc := range []struct {
		name      string
		server    *api.Server
		serverErr error
		getErr    error
		query     bool
		absent    bool
	}{
		{name: "404", server: trusted, getErr: api.StatusErrorf(http.StatusNotFound, "missing"), query: true, absent: true},
		{name: "present", server: trusted, query: true},
		{name: "forbidden", server: trusted, getErr: api.StatusErrorf(http.StatusForbidden, "secret"), query: true},
		{name: "generic error", server: trusted, getErr: errors.New("not found"), query: true},
		{name: "clustered", server: clustered},
		{name: "unsupported version", server: &api.Server{ServerUntrusted: api.ServerUntrusted{Auth: "trusted"}, Environment: api.ServerEnvironment{ServerVersion: "7.0"}}},
		{name: "server unavailable", serverErr: errors.New("secret")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := new(MockIncusServer)
			cli := &readinessServer{MockIncusServer: base, server: tc.server, serverErr: tc.serverErr}
			if tc.query {
				base.On("GetInstanceFull", "vm").Return(&api.InstanceFull{}, "", tc.getErr).Once()
			}
			p := &Incus{cfg: &config.Incus{ProjectName: "p"}, cli: cli, admissionBackendIdentity: strings.Repeat("c", 64)}
			got := p.Absence(context.Background(), req)
			require.Equal(t, tc.absent, got.Absent)
			require.Equal(t, req, got.Request)
			require.Equal(t, AbsenceContract, got.Contract)
			require.LessOrEqual(t, got.ValidUntil.Sub(got.CheckedAt), 15*time.Second)
			require.NotContains(t, got.Reason, "secret")
			base.AssertExpectations(t)
		})
	}
	p := &Incus{cfg: &config.Incus{ProjectName: "different"}}
	require.False(t, p.Absence(context.Background(), req).Absent)
}
