// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL
package provider

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-incus/config"
	"github.com/stretchr/testify/require"
)

func TestBackendIdentityBindsCertificateEndpointProject(t *testing.T) {
	cert := func(serial int64) string {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		template := &x509.Certificate{SerialNumber: big.NewInt(serial)}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		require.NoError(t, err)
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	a, b := cert(1), cert(2)
	identity, err := backendIdentity(a, "https://example.invalid:8443", "project-a")
	require.NoError(t, err)
	require.Len(t, identity, 64)
	for _, v := range []struct{ cert, endpoint, project string }{{b, "https://example.invalid:8443", "project-a"}, {a, "https://other.invalid:8443", "project-a"}, {a, "https://example.invalid:8443", "project-b"}} {
		other, err := backendIdentity(v.cert, v.endpoint, v.project)
		require.NoError(t, err)
		require.NotEqual(t, identity, other)
	}
	_, err = backendIdentity("invalid", "https://example.invalid", "p")
	require.Error(t, err)
}

func TestMismatchedBackendCannotCreateOrReportAbsent(t *testing.T) {
	p := &Incus{cfg: &config.Incus{ProjectName: "p"}, cli: new(MockIncusServer), admissionBackendIdentity: strings.Repeat("a", 64)}
	request := AbsenceRequest{Version: 1, RequestID: "r", Generation: "g", Domain: "d", Project: "p", InstanceName: "vm", ExpectedBackendIdentity: strings.Repeat("b", 64)}
	require.False(t, p.Absence(context.Background(), request).Absent)
	del := AdmittedDeleteRequest{Version: 1, RequestID: "r", Project: "p", InstanceName: "vm", ExpectedBackendIdentity: strings.Repeat("b", 64)}
	require.False(t, p.AdmittedDelete(context.Background(), del).Deleted)
	_, err := p.getCreateInstanceArgs(context.Background(), commonParams.BootstrapInstance{Name: "vm"}, extraSpecs{AdmissionResources: &AdmittedResourceSpec{BackendIdentity: strings.Repeat("b", 64)}})
	require.Error(t, err)
}
