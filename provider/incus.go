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

package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/execution"
	"github.com/cloudbase/garm-provider-incus/config"

	incus "github.com/lxc/incus/client"
	"github.com/lxc/incus/shared/api"
	"github.com/pkg/errors"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
)

var _ execution.ExternalProvider = &Incus{}

const (
	// We look for this key in the config of the instances to determine if they are
	// created by us or not.
	controllerIDKeyName = "user.runner-controller-id"
	poolIDKey           = "user.runner-pool-id"

	// osTypeKeyName is the key we use in the instance config to indicate the OS
	// platform a runner is supposed to have. This value is defined in the pool and
	// passed into the provider as bootstrap params.
	osTypeKeyName = "user.os-type"

	// osArchKeyNAme is the key we use in the instance config to indicate the OS
	// architecture a runner is supposed to have. This value is defined in the pool and
	// passed into the provider as bootstrap params.
	osArchKeyNAme = "user.os-arch"
)

var (
	configToIncusArchMap map[commonParams.OSArch]string = map[commonParams.OSArch]string{
		commonParams.Amd64: "x86_64",
		commonParams.Arm64: "aarch64",
		commonParams.Arm:   "armv7l",
	}

	incusToConfigArch map[string]commonParams.OSArch = map[string]commonParams.OSArch{
		"x86_64":  commonParams.Amd64,
		"aarch64": commonParams.Arm64,
		"armv7l":  commonParams.Arm,
	}
)

const (
	DefaultProjectDescription = "This project was created automatically by garm to be used for github ephemeral action runners."
	DefaultProjectName        = "garm-project"
)

type ToolFetchFunc func(osType commonParams.OSType, osArch commonParams.OSArch, tools []commonParams.RunnerApplicationDownload) (commonParams.RunnerApplicationDownload, error)

type GetCloudConfigFunc func(bootstrapParams commonParams.BootstrapInstance, tools commonParams.RunnerApplicationDownload, runnerName string) (string, error)

var (
	DefaultToolFetch      ToolFetchFunc      = util.GetTools
	DefaultGetCloudconfig GetCloudConfigFunc = cloudconfig.GetCloudConfig
)

func NewIncusProvider(configFile, controllerID string) (execution.ExternalProvider, error) {
	cfg, err := config.NewConfig(configFile)
	if err != nil {
		return nil, errors.Wrap(err, "parsing config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating provider config")
	}

	if len(cfg.ImageRemotes) == 0 {
		return nil, fmt.Errorf("no image remotes configured")
	}

	provider := &Incus{
		cfg:          cfg,
		controllerID: controllerID,
		imageManager: &image{
			remotes: cfg.ImageRemotes,
		},
	}

	return provider, nil
}

type InstanceServerInterface interface {
	GetProject(string) (*api.Project, string, error)
	UseProject(string) incus.InstanceServer
	GetProfileNames() ([]string, error)
	CreateInstance(api.InstancesPost) (incus.Operation, error)
	UpdateInstanceState(string, api.InstanceStatePut, string) (incus.Operation, error)
	GetInstanceFull(string) (*api.InstanceFull, string, error)
	DeleteInstance(string) (incus.Operation, error)
	GetInstancesFull(api.InstanceType) ([]api.InstanceFull, error)
	GetImageAliasArchitectures(string, string) (map[string]*api.ImageAliasesEntry, error)
	GetImage(string) (*api.Image, string, error)
}

type Incus struct {
	// cfg is the provider config for this provider.
	cfg *config.Incus
	// cli is the Incus client.
	cli InstanceServerInterface
	// imageManager downloads images from remotes
	imageManager *image
	// controllerID is the ID of this controller
	controllerID string

	mux sync.Mutex
}

func (l *Incus) getCLI(ctx context.Context) (InstanceServerInterface, error) {
	l.mux.Lock()
	defer l.mux.Unlock()

	if l.cli != nil {
		return l.cli, nil
	}
	cli, err := getClientFromConfig(ctx, l.cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating Incus client")
	}

	_, _, err = cli.GetProject(projectName(l.cfg))
	if err != nil {
		return nil, errors.Wrapf(err, "fetching project name: %s", projectName(l.cfg))
	}
	cli = cli.UseProject(projectName(l.cfg))
	l.cli = cli

	return cli, nil
}

func (l *Incus) getProfiles(ctx context.Context, flavor string) ([]string, error) {
	ret := []string{}
	if l.cfg.IncludeDefaultProfile {
		ret = append(ret, "default")
	}

	set := map[string]struct{}{}

	cli, err := l.getCLI(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching client")
	}

	profiles, err := cli.GetProfileNames()
	if err != nil {
		return nil, errors.Wrap(err, "fetching profile names")
	}
	for _, profile := range profiles {
		set[profile] = struct{}{}
	}

	if _, ok := set[flavor]; !ok {
		return nil, errors.Wrapf(runnerErrors.ErrNotFound, "looking for profile %s", flavor)
	}

	ret = append(ret, flavor)
	return ret, nil
}

// sadly, the security.secureboot flag is a string encoded boolean.
func (l *Incus) secureBootEnabled() string {
	if l.cfg.SecureBoot {
		return "true"
	}
	return "false"
}

func (l *Incus) getCreateInstanceArgs(ctx context.Context, bootstrapParams commonParams.BootstrapInstance, specs extraSpecs) (api.InstancesPost, error) {
	if bootstrapParams.Name == "" {
		return api.InstancesPost{}, runnerErrors.NewBadRequestError("missing name")
	}
	profiles, err := l.getProfiles(ctx, bootstrapParams.Flavor)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "fetching profiles")
	}

	arch, err := resolveArchitecture(bootstrapParams.OSArch)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "fetching archictecture")
	}

	instanceType := l.cfg.GetInstanceType()
	instanceSource, err := l.imageManager.getInstanceSource(bootstrapParams.Image, instanceType, arch, l.cli)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting instance source")
	}

	tools, err := DefaultToolFetch(bootstrapParams.OSType, bootstrapParams.OSArch, bootstrapParams.Tools)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting tools")
	}

	bootstrapParams.UserDataOptions.DisableUpdatesOnBoot = specs.DisableUpdates
	bootstrapParams.UserDataOptions.ExtraPackages = specs.ExtraPackages
	bootstrapParams.UserDataOptions.EnableBootDebug = specs.EnableBootDebug
	cloudCfg, err := DefaultGetCloudconfig(bootstrapParams, tools, bootstrapParams.Name)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "generating cloud-config")
	}

	if bootstrapParams.OSType == commonParams.Windows {
		cloudCfg = fmt.Sprintf("#ps1_sysnative\n%s", cloudCfg)
	}

	configMap := map[string]string{
		"user.user-data":    cloudCfg,
		osTypeKeyName:       string(bootstrapParams.OSType),
		osArchKeyNAme:       string(bootstrapParams.OSArch),
		controllerIDKeyName: l.controllerID,
		poolIDKey:           bootstrapParams.PoolID,
	}

	if instanceType == config.IncusImageVirtualMachine {
		configMap["security.secureboot"] = l.secureBootEnabled()
	}

	args := api.InstancesPost{
		InstancePut: api.InstancePut{
			Architecture: arch,
			Profiles:     profiles,
			Description:  "Github runner provisioned by garm",
			Config:       configMap,
		},
		Source: instanceSource,
		Name:   bootstrapParams.Name,
		Type:   api.InstanceType(instanceType),
	}
	return args, nil
}

func (l *Incus) launchInstance(ctx context.Context, createArgs api.InstancesPost) error {
	cli, err := l.getCLI(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching client")
	}
	// Get Incus to create the instance (background operation)
	op, err := cli.CreateInstance(createArgs)
	if err != nil {
		return errors.Wrap(err, "creating instance")
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for instance creation")
	}

	// Get Incus to start the instance (background operation)
	reqState := api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}

	op, err = cli.UpdateInstanceState(createArgs.Name, reqState, "")
	if err != nil {
		return errors.Wrap(err, "starting instance")
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for instance to start")
	}
	return nil
}

// CreateInstance creates a new compute instance in the provider.
func (l *Incus) CreateInstance(ctx context.Context, bootstrapParams commonParams.BootstrapInstance) (commonParams.ProviderInstance, error) {
	extraSpecs, err := parseExtraSpecsFromBootstrapParams(bootstrapParams)
	if err != nil {
		return commonParams.ProviderInstance{}, errors.Wrap(err, "parsing extra specs")
	}
	args, err := l.getCreateInstanceArgs(ctx, bootstrapParams, extraSpecs)
	if err != nil {
		return commonParams.ProviderInstance{}, errors.Wrap(err, "fetching create args")
	}

	if err := l.launchInstance(ctx, args); err != nil {
		return commonParams.ProviderInstance{}, errors.Wrap(err, "creating instance")
	}

	ret, err := l.waitInstanceHasIP(ctx, args.Name)
	if err != nil {
		return commonParams.ProviderInstance{}, errors.Wrap(err, "fetching instance")
	}

	return ret, nil
}

// GetInstance will return details about one instance.
func (l *Incus) GetInstance(ctx context.Context, instanceName string) (commonParams.ProviderInstance, error) {
	cli, err := l.getCLI(ctx)
	if err != nil {
		return commonParams.ProviderInstance{}, errors.Wrap(err, "fetching client")
	}
	instance, _, err := cli.GetInstanceFull(instanceName)
	if err != nil {
		if isNotFoundError(err) {
			return commonParams.ProviderInstance{}, errors.Wrapf(runnerErrors.ErrNotFound, "fetching instance: %q", err)
		}
		return commonParams.ProviderInstance{}, errors.Wrap(err, "fetching instance")
	}

	return incusInstanceToAPIInstance(instance), nil
}

// Delete instance will delete the instance in a provider.
func (l *Incus) DeleteInstance(ctx context.Context, instance string) error {
	cli, err := l.getCLI(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching client")
	}

	if err := l.setState(ctx, instance, "stop", true); err != nil {
		if isNotFoundError(err) {
			return nil
		}
		// I am not proud of this, but the drivers.ErrInstanceIsStopped from Incus pulls in
		// a ton of CGO, linux specific dependencies, that don't make sense having
		// in garm.
		if !(errors.Cause(err).Error() == errInstanceIsStopped.Error()) {
			return errors.Wrap(err, "stopping instance")
		}
	}

	opResponse := make(chan struct {
		op  incus.Operation
		err error
	})
	var op incus.Operation
	go func() {
		op, err := cli.DeleteInstance(instance)
		opResponse <- struct {
			op  incus.Operation
			err error
		}{op: op, err: err}
	}()

	select {
	case resp := <-opResponse:
		if resp.err != nil {
			if isNotFoundError(resp.err) {
				return nil
			}
			return errors.Wrap(resp.err, "removing instance")
		}
		op = resp.op
	case <-time.After(time.Second * 60):
		return errors.Wrapf(runnerErrors.ErrTimeout, "removing instance %s", instance)
	}

	opTimeout, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err = op.WaitContext(opTimeout)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return errors.Wrap(err, "waiting for instance deletion")
	}
	return nil
}

type listResponse struct {
	instances []api.InstanceFull
	err       error
}

// ListInstances will list all instances for a provider.
func (l *Incus) ListInstances(ctx context.Context, poolID string) ([]commonParams.ProviderInstance, error) {
	cli, err := l.getCLI(ctx)
	if err != nil {
		return []commonParams.ProviderInstance{}, errors.Wrap(err, "fetching client")
	}

	result := make(chan listResponse, 1)

	go func() {
		// TODO(gabriel-samfira): if this blocks indefinitely, we will leak a goroutine.
		// Convert the internal provider to an external one. Running the provider as an
		// external process will allow us to not care if a goroutine leaks. Once a timeout
		// is reached, the provider can just exit with an error. Something we can't do with
		// internal providers.
		instances, err := cli.GetInstancesFull(api.InstanceTypeAny)
		result <- listResponse{
			instances: instances,
			err:       err,
		}
	}()

	var instances []api.InstanceFull
	select {
	case res := <-result:
		if res.err != nil {
			return []commonParams.ProviderInstance{}, errors.Wrap(res.err, "fetching instances")
		}
		instances = res.instances
	case <-time.After(time.Second * 60):
		return []commonParams.ProviderInstance{}, errors.Wrap(runnerErrors.ErrTimeout, "fetching instances from provider")
	}

	ret := []commonParams.ProviderInstance{}

	for _, instance := range instances {
		if id, ok := instance.ExpandedConfig[controllerIDKeyName]; ok && id == l.controllerID {
			if poolID != "" {
				id := instance.ExpandedConfig[poolIDKey]
				if id != poolID {
					// Pool ID was specified. Filter out instances belonging to other pools.
					continue
				}
			}
			ret = append(ret, incusInstanceToAPIInstance(&instance))
		}
	}

	return ret, nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (l *Incus) RemoveAllInstances(ctx context.Context) error {
	instances, err := l.ListInstances(ctx, "")
	if err != nil {
		return errors.Wrap(err, "fetching instance list")
	}

	for _, instance := range instances {
		// TODO: remove in parallel
		if err := l.DeleteInstance(ctx, instance.Name); err != nil {
			return errors.Wrapf(err, "removing instance %s", instance.Name)
		}
	}

	return nil
}

func (l *Incus) setState(ctx context.Context, instance, state string, force bool) error {
	reqState := api.InstanceStatePut{
		Action:  state,
		Timeout: -1,
		Force:   force,
	}

	cli, err := l.getCLI(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching client")
	}

	op, err := cli.UpdateInstanceState(instance, reqState, "")
	if err != nil {
		return errors.Wrapf(err, "setting state to %s", state)
	}
	ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err = op.WaitContext(ctxTimeout)
	if err != nil {
		return errors.Wrapf(err, "waiting for instance to transition to state %s", state)
	}
	return nil
}

// Stop shuts down the instance.
func (l *Incus) Stop(ctx context.Context, instance string, force bool) error {
	return l.setState(ctx, instance, "stop", force)
}

// Start boots up an instance.
func (l *Incus) Start(ctx context.Context, instance string) error {
	return l.setState(ctx, instance, "start", false)
}
