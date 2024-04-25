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
	"fmt"
	"net/url"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

type IncusRemoteProtocol string
type IncusImageType string

func (l IncusImageType) String() string {
	return string(l)
}

const (
	SimpleStreams            IncusRemoteProtocol = "simplestreams"
	IncusImageVirtualMachine IncusImageType      = "virtual-machine"
	IncusImageContainer      IncusImageType      = "container"
)

// IncusImageRemote holds information about a remote server from which Incus can fetch
// OS images. Typically this will be a simplestreams server.
type IncusImageRemote struct {
	Address            string              `toml:"addr" json:"addr"`
	Public             bool                `toml:"public" json:"public"`
	Protocol           IncusRemoteProtocol `toml:"protocol" json:"protocol"`
	InsecureSkipVerify bool                `toml:"skip_verify" json:"skip-verify"`
}

func (l *IncusImageRemote) Validate() error {
	if l.Protocol != SimpleStreams {
		// Only supports simplestreams for now.
		return fmt.Errorf("invalid remote protocol %s. Supported protocols: %s", l.Protocol, SimpleStreams)
	}
	if l.Address == "" {
		return fmt.Errorf("missing address")
	}

	url, err := url.ParseRequestURI(l.Address)
	if err != nil {
		return errors.Wrap(err, "validating address")
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Errorf("address must be http or https")
	}

	return nil
}

// NewConfig returns a new Config
func NewConfig(cfgFile string) (*Incus, error) {
	var config Incus
	if _, err := toml.DecodeFile(cfgFile, &config); err != nil {
		return nil, fmt.Errorf("error decoding config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	return &config, nil
}

// Incus holds connection information for an Incus cluster.
type Incus struct {
	// UnixSocket is the path on disk to the Incus unix socket. If defined,
	// this is prefered over connecting via HTTPs.
	UnixSocket string `toml:"unix_socket_path" json:"unix-socket-path"`

	// Project name is the name of the project in which this runner will create
	// instances. If this option is not set, the default project will be used.
	// The project used here, must have all required profiles created by you
	// beforehand. For Incus, the "flavor" used in the runner definition for a pool
	// equates to a profile in the desired project.
	ProjectName string `toml:"project_name" json:"project-name"`

	// IncludeDefaultProfile specifies whether or not this provider will always add
	// the "default" profile to any newly created instance.
	IncludeDefaultProfile bool `toml:"include_default_profile" json:"include-default-profile"`

	// URL holds the URL of the remote Incus server.
	// example: https://10.10.10.1:8443/
	URL string `toml:"url" json:"url"`
	// ClientCertificate is the x509 client certificate path used for authentication.
	ClientCertificate string `toml:"client_certificate" json:"client_certificate"`
	// ClientKey is the key used for client certificate authentication.
	ClientKey string `toml:"client_key" json:"client-key"`
	// TLS certificate of the remote server. If not specified, the system CA is used.
	TLSServerCert string `toml:"tls_server_certificate" json:"tls-server-certificate"`
	// TLSCA is the TLS CA certificate when running Incus in PKI mode.
	TLSCA string `toml:"tls_ca" json:"tls-ca"`

	// ImageRemotes is a map to a set of remote image repositories we can use to
	// download images.
	ImageRemotes map[string]IncusImageRemote `toml:"image_remotes" json:"image-remotes"`

	// SecureBoot enables secure boot for VMs spun up using this provider.
	SecureBoot bool `toml:"secure_boot" json:"secure-boot"`

	// InstanceType allows you to choose between a virtual machine and a container
	InstanceType IncusImageType `toml:"instance_type" json:"instance-type"`
}

func (l *Incus) GetInstanceType() IncusImageType {
	switch l.InstanceType {
	case IncusImageVirtualMachine, IncusImageContainer:
		return l.InstanceType
	default:
		return IncusImageVirtualMachine
	}
}

func (l *Incus) Validate() error {
	if l.UnixSocket != "" {
		if _, err := os.Stat(l.UnixSocket); err != nil {
			return fmt.Errorf("could not access unix socket %s: %w", l.UnixSocket, err)
		}

		return nil
	}

	if l.URL == "" {
		return fmt.Errorf("unix_socket or address must be specified")
	}

	url, err := url.ParseRequestURI(l.URL)
	if err != nil {
		return fmt.Errorf("invalid Incus URL")
	}

	if url.Scheme != "https" {
		return fmt.Errorf("address must be https")
	}

	if l.ClientCertificate == "" || l.ClientKey == "" {
		return fmt.Errorf("client_certificate and client_key are mandatory")
	}

	if _, err := os.Stat(l.ClientCertificate); err != nil {
		return fmt.Errorf("failed to access client certificate %s: %w", l.ClientCertificate, err)
	}

	if _, err := os.Stat(l.ClientKey); err != nil {
		return fmt.Errorf("failed to access client key %s: %w", l.ClientKey, err)
	}

	if l.TLSServerCert != "" {
		if _, err := os.Stat(l.TLSServerCert); err != nil {
			return fmt.Errorf("failed to access tls_server_certificate %s: %w", l.TLSServerCert, err)
		}
	}

	for name, val := range l.ImageRemotes {
		if err := val.Validate(); err != nil {
			return fmt.Errorf("remote %s is invalid: %s", name, err)
		}
	}
	return nil
}
