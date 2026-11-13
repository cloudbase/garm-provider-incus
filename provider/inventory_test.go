// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL
package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudbase/garm-provider-incus/config"
	"github.com/lxc/incus/v7/shared/api"
	"github.com/stretchr/testify/require"
)

func TestInventoryBoundToBackendAndController(t *testing.T) {
	identity := strings.Repeat("a", 64)
	req := InventoryRequest{Version: 1, RequestID: "r", ExpectedBackendIdentity: identity, Project: "p"}
	for _, tc := range []struct {
		name     string
		backend  string
		err      error
		guests   []api.InstanceFull
		complete bool
		count    int
	}{
		{name: "includes unknown pool", backend: identity, guests: []api.InstanceFull{{Instance: api.Instance{Name: "one", ExpandedConfig: map[string]string{controllerIDKeyName: "c"}}}, {Instance: api.Instance{Name: "unowned", ExpandedConfig: map[string]string{controllerIDKeyName: "other"}}}}, complete: true, count: 1},
		{name: "empty complete", backend: identity, complete: true},
		{name: "list failed", backend: identity, err: errors.New("secret")},
		{name: "different backend", backend: strings.Repeat("b", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := new(MockIncusServer)
			cli := &readinessServer{MockIncusServer: base, server: &api.Server{ServerUntrusted: api.ServerUntrusted{Auth: "trusted"}, Environment: api.ServerEnvironment{ServerVersion: "6.0.9"}}}
			if tc.backend == identity {
				base.On("GetInstancesFull", api.InstanceTypeAny).Return(tc.guests, tc.err).Once()
			}
			p := &Incus{cfg: &config.Incus{ProjectName: "p"}, controllerID: "c", cli: cli, admissionBackendIdentity: tc.backend}
			got := p.Inventory(context.Background(), req)
			require.Equal(t, tc.complete, got.Complete)
			require.Len(t, got.Instances, tc.count)
			require.NotContains(t, got.Reason, "secret")
			if tc.complete && tc.count > 0 {
				require.Equal(t, "one", got.Instances[0].Name)
			}
			base.AssertExpectations(t)
		})
	}
}
