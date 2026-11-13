// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudbase/garm-provider-incus/provider"
	"github.com/stretchr/testify/require"
)

func TestReadinessWire(t *testing.T) {
	req := `{"version":1,"request_id":"r","generation":"g","domain":"d","project":"p","image":"i","flavor":"f","os_arch":"amd64","instance_type":"virtual-machine","resources":{"cpu":1,"memory_bytes":1024,"disk_bytes":1024,"vm_slots":1}}`
	for _, tc := range []struct {
		name, input string
		valid       bool
	}{
		{"missing config", req, true},
		{"unknown field", strings.Replace(req, `"version":1`, `"version":1,"unknown":1`, 1), false},
		{"unknown nested field", strings.Replace(req, `"cpu":1`, `"cpu":1,"unknown":1`, 1), false},
		{"unsupported version", strings.Replace(req, `"version":1`, `"version":2`, 1), false},
		{"trailing data", req + ` {}`, false},
		{"oversize", req + strings.Repeat(" ", 65537), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GARM_PROVIDER_CONFIG_FILE", "")
			var out bytes.Buffer
			err := runReadiness(context.Background(), strings.NewReader(tc.input), &out)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var got provider.ReadinessResponse
			require.NoError(t, json.Unmarshal(out.Bytes(), &got))
			require.False(t, got.Ready)
			require.Equal(t, "r", got.Request.RequestID)
			require.NotEmpty(t, got.Reason)
		})
	}
}

func TestReadinessCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	require.Error(t, runReadiness(ctx, strings.NewReader(""), &out))
	require.Empty(t, out.String())
}

func TestAbsenceWire(t *testing.T) {
	req := `{"version":1,"request_id":"r","generation":"g","domain":"d","project":"p","instance_name":"vm","expected_backend_identity":"identity"}`
	for _, tc := range []struct {
		name, input string
		valid       bool
	}{
		{"missing config", req, true},
		{"unknown field", strings.Replace(req, `"version":1`, `"version":1,"unknown":1`, 1), false},
		{"wrong version", strings.Replace(req, `"version":1`, `"version":2`, 1), false},
		{"trailing", req + ` {}`, false},
		{"oversize", req + strings.Repeat(" ", 65537), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GARM_PROVIDER_CONFIG_FILE", "")
			var out bytes.Buffer
			err := runAbsence(context.Background(), strings.NewReader(tc.input), &out)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var got provider.AbsenceResponse
			require.NoError(t, json.Unmarshal(out.Bytes(), &got))
			require.False(t, got.Absent)
			require.Equal(t, "r", got.Request.RequestID)
			require.Equal(t, provider.AbsenceContract, got.Contract)
		})
	}
}

func TestAdmittedDeleteWireFailsClosed(t *testing.T) {
	request := `{"version":1,"request_id":"r","project":"p","instance_name":"vm","expected_backend_identity":"identity"}`
	t.Setenv("GARM_PROVIDER_CONFIG_FILE", "")
	var out bytes.Buffer
	require.NoError(t, runAdmittedDelete(context.Background(), strings.NewReader(request), &out))
	var result provider.AdmittedDeleteResponse
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	require.False(t, result.Deleted)
	require.Equal(t, "r", result.Request.RequestID)
	out.Reset()
	require.Error(t, runAdmittedDelete(context.Background(), strings.NewReader(strings.Replace(request, `"version":1`, `"version":1,"bad":1`, 1)), &out))
}
