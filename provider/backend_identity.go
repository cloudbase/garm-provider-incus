// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL
package provider

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"
)

// admissionBackend binds all operations to a project on an exact TLS leaf and
// endpoint. Unix sockets and CA-only authentication lack this identity contract.
func (l *Incus) admissionBackend(ctx context.Context) (string, error) {
	l.mux.Lock()
	defer l.mux.Unlock()
	if l.admissionBackendIdentity != "" {
		return l.admissionBackendIdentity, nil
	}
	if l.cli != nil {
		return "", fmt.Errorf("admission client is not pinned")
	}
	u, err := url.Parse(l.cfg.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid admission endpoint")
	}
	cli, err := getClientFromConfigWithPin(ctx, l.cfg, true)
	if err != nil {
		return "", err
	}
	info, err := cli.GetConnectionInfo()
	if err != nil || info == nil {
		return "", fmt.Errorf("missing connection identity")
	}
	identity, err := backendIdentity(info.Certificate, "https://"+strings.ToLower(u.Host), projectName(l.cfg))
	if err != nil {
		return "", err
	}
	if _, _, err = cli.GetProject(projectName(l.cfg)); err != nil {
		return "", err
	}
	l.admissionBackendIdentity = identity
	l.cli = cli.UseProject(projectName(l.cfg))
	return l.admissionBackendIdentity, nil
}

func backendIdentity(certificate, endpoint, project string) (string, error) {
	block, rest := pem.Decode([]byte(certificate))
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return "", fmt.Errorf("invalid pinned certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("invalid pinned certificate")
	}
	h := sha256.New()
	for _, part := range [][]byte{cert.Raw, []byte(endpoint), []byte(project)} {
		_, _ = fmt.Fprintf(h, "%d:", len(part))
		_, _ = h.Write(part)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
