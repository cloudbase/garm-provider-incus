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

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func getDefaultIncusImageRemoteConfig() IncusImageRemote {
	return IncusImageRemote{
		Address:            "https://cloud-images.ubuntu.com/releases",
		Public:             true,
		Protocol:           SimpleStreams,
		InsecureSkipVerify: false,
	}
}

func getDefaultIncusConfig() Incus {
	remote := getDefaultIncusImageRemoteConfig()
	return Incus{
		URL:                   "https://example.com:8443",
		ProjectName:           "default",
		IncludeDefaultProfile: false,
		ClientCertificate:     "../testdata/incus/certs/client.crt",
		ClientKey:             "../testdata/incus/certs/client.key",
		TLSServerCert:         "../testdata/incus/certs/servercert.crt",
		ImageRemotes: map[string]IncusImageRemote{
			"default": remote,
		},
		SecureBoot: false,
	}
}

func TestIncusRemote(t *testing.T) {
	cfg := getDefaultIncusImageRemoteConfig()

	err := cfg.Validate()
	require.Nil(t, err)
}

func TestIncusRemoteEmptyAddress(t *testing.T) {
	cfg := getDefaultIncusImageRemoteConfig()

	cfg.Address = ""

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "missing address")
}

func TestIncusRemoteInvalidAddress(t *testing.T) {
	cfg := getDefaultIncusImageRemoteConfig()

	cfg.Address = "bogus address"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "validating address: parse \"bogus address\": invalid URI for request")
}

func TestIncusRemoteIvalidAddressScheme(t *testing.T) {
	cfg := getDefaultIncusImageRemoteConfig()

	cfg.Address = "ftp://whatever"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "address must be http or https")
}

func TestIncusConfig(t *testing.T) {
	cfg := getDefaultIncusConfig()
	err := cfg.Validate()
	require.Nil(t, err)
}

func TestIncusWithInvalidUnixSocket(t *testing.T) {
	cfg := getDefaultIncusConfig()

	cfg.UnixSocket = "bogus unix socket"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestMissingUnixSocketAndMissingURL(t *testing.T) {
	cfg := getDefaultIncusConfig()

	cfg.URL = ""
	cfg.UnixSocket = ""

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "unix_socket or address must be specified")
}

func TestInvalidIncusURL(t *testing.T) {
	cfg := getDefaultIncusConfig()
	cfg.URL = "bogus"

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "invalid Incus URL")
}

func TestIncusURLIsHTTPS(t *testing.T) {
	cfg := getDefaultIncusConfig()
	cfg.URL = "http://example.com"

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "address must be https")
}

func TestMissingClientCertOrKey(t *testing.T) {
	cfg := getDefaultIncusConfig()
	cfg.ClientKey = ""
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "client_certificate and client_key are mandatory")

	cfg = getDefaultIncusConfig()
	cfg.ClientCertificate = ""
	err = cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "client_certificate and client_key are mandatory")
}

func TestIncusIvalidCertOrKeyPaths(t *testing.T) {
	cfg := getDefaultIncusConfig()
	cfg.ClientCertificate = "/i/am/not/here"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)

	cfg.ClientCertificate = "../testdata/incus/certs/client.crt"
	cfg.ClientKey = "/me/neither"

	err = cfg.Validate()
	require.NotNil(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestIncusInvalidServerCertPath(t *testing.T) {
	cfg := getDefaultIncusConfig()
	cfg.TLSServerCert = "/not/a/valid/server/cert/path"

	err := cfg.Validate()
	require.NotNil(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestInvalidIncusImageRemotes(t *testing.T) {
	cfg := getDefaultIncusConfig()

	cfg.ImageRemotes["default"] = IncusImageRemote{
		Protocol: IncusRemoteProtocol("bogus"),
	}

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "remote default is invalid: invalid remote protocol bogus. Supported protocols: simplestreams")
}
