// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Cloudbase Solutions SRL

package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/lxc/incus/v7/shared/api"
	"github.com/lxc/incus/v7/shared/units"
)

// ReadinessRequest and ReadinessResponse implement the experimental readiness_v1
// wire protocol. Resources are whole CPUs, bytes and VM slots.
type ReadinessRequest struct {
	Version                 int                `json:"version"`
	RequestID               string             `json:"request_id"`
	Generation              string             `json:"generation"`
	Domain                  string             `json:"domain"`
	Project                 string             `json:"project"`
	Image                   string             `json:"image"`
	Flavor                  string             `json:"flavor"`
	OSArch                  string             `json:"os_arch"`
	InstanceType            string             `json:"instance_type"`
	Resources               ReadinessResources `json:"resources"`
	ExpectedBackendIdentity string             `json:"expected_backend_identity"`
}

type ReadinessResources struct {
	CPU         int64 `json:"cpu"`
	MemoryBytes int64 `json:"memory_bytes"`
	DiskBytes   int64 `json:"disk_bytes"`
	VMSlots     int64 `json:"vm_slots"`
}

type AdmittedResourceSpec struct {
	CPU             int64  `json:"cpu"`
	MemoryBytes     int64  `json:"memory_bytes"`
	DiskBytes       int64  `json:"disk_bytes"`
	VMSlots         int64  `json:"vm_slots"`
	BackendIdentity string `json:"backend_identity"`
}

func (r AdmittedResourceSpec) vector() ReadinessResources {
	return ReadinessResources{CPU: r.CPU, MemoryBytes: r.MemoryBytes, DiskBytes: r.DiskBytes, VMSlots: r.VMSlots}
}

type ReadinessResponse struct {
	Request              ReadinessRequest `json:"request"`
	Ready                bool             `json:"ready"`
	CheckedAt            time.Time        `json:"checked_at"`
	ValidUntil           time.Time        `json:"valid_until"`
	ImageFingerprint     string           `json:"image_fingerprint"`
	Reason               string           `json:"reason,omitempty"`
	Checks               ReadinessChecks  `json:"checks"`
	BackendIdentity      string           `json:"backend_identity"`
	ProvisioningContract string           `json:"provisioning_contract"`
}

type ReadinessChecks struct {
	Project               bool `json:"project"`
	Profile               bool `json:"profile"`
	Image                 bool `json:"image"`
	Architecture          bool `json:"architecture"`
	Resources             bool `json:"resources"`
	Server                bool `json:"server"`
	ProjectLimits         bool `json:"project_limits"`
	ResourcesEnforced     bool `json:"resources_enforced"`
	CreateWaitsCompletion bool `json:"create_waits_completion"`
}

type hostReader interface {
	GetServer() (*api.Server, string, error)
	GetProjectState(string) (*api.ProjectState, error)
}

type profileReader interface {
	GetProfile(string) (*api.Profile, string, error)
}

// Readiness checks only the documented read-only criteria. It does not prove
// create permission, KVM health, future availability, or host-wide capacity.
func (l *Incus) Readiness(ctx context.Context, req ReadinessRequest) ReadinessResponse {
	now := time.Now().UTC()
	result := ReadinessResponse{Request: req, CheckedAt: now, ValidUntil: now.Add(15 * time.Second), Reason: "configuration check failed"}
	fail := func(reason string) ReadinessResponse { result.Reason = reason; return result }
	if req.Version != 1 || req.RequestID == "" || req.Generation == "" || req.Domain == "" || req.Project == "" || req.Image == "" || req.Flavor == "" || req.OSArch == "" || req.InstanceType == "" || req.Resources.CPU <= 0 || req.Resources.MemoryBytes <= 0 || req.Resources.DiskBytes <= 0 || req.Resources.VMSlots != 1 {
		return fail("invalid request")
	}
	if req.Project != projectName(l.cfg) || req.InstanceType != l.cfg.GetInstanceType().String() {
		return fail("configuration mismatch")
	}
	if !isExactFingerprint(req.Image) {
		return fail("image fingerprint required")
	}
	arch, err := resolveArchitecture(commonParams.OSArch(req.OSArch))
	if err != nil {
		return fail("unsupported architecture")
	}
	identity, err := l.admissionBackend(ctx)
	if err != nil || (req.ExpectedBackendIdentity != "" && req.ExpectedBackendIdentity != identity) {
		return fail("backend identity unavailable or changed")
	}
	result.BackendIdentity = identity
	cli, err := l.getCLI(ctx)
	if err != nil {
		return fail("project unavailable")
	}
	host, ok := cli.(hostReader)
	if !ok {
		return fail("server inspection unsupported")
	}
	server, _, err := host.GetServer()
	if err != nil || server == nil || server.Auth != "trusted" || server.Public || server.Environment.ServerClustered || (server.Environment.ServerVersion != "6.0" && !strings.HasPrefix(server.Environment.ServerVersion, "6.0.")) {
		return fail("server unavailable or unsupported")
	}
	driver := "qemu"
	if req.InstanceType == "container" {
		driver = "lxc"
	}
	found := false
	for _, d := range strings.Split(server.Environment.Driver, "|") {
		if strings.TrimSpace(d) == driver {
			found = true
		}
	}
	if !found || server.Environment.KernelArchitecture != arch {
		return fail("server driver or architecture unavailable")
	}
	result.Checks.Server = true
	project, _, err := cli.GetProject(req.Project)
	if err != nil || project == nil || project.Name != req.Project {
		return fail("project unavailable")
	}
	result.Checks.Project = true
	state, err := host.GetProjectState(req.Project)
	if err != nil || state == nil {
		return fail("project limits unavailable")
	}
	// Missing quota entries are unknown, not evidence of unlimited capacity.
	resourceNeeds := map[string]int64{"cpu": req.Resources.CPU, "memory": req.Resources.MemoryBytes, "disk": req.Resources.DiskBytes, "instances": 1}
	if req.InstanceType == "virtual-machine" {
		resourceNeeds["virtual-machines"] = 1
	} else {
		resourceNeeds["containers"] = 1
	}
	for key, needed := range resourceNeeds {
		quota, ok := state.Resources[key]
		if !ok || quota.Usage < 0 || quota.Limit < -1 || (quota.Limit >= 0 && (quota.Usage > quota.Limit || needed > quota.Limit-quota.Usage)) {
			return fail("project quota unavailable or full")
		}
	}
	result.Checks.ProjectLimits = true
	_, _, err = l.admissionProfile(ctx, req.Flavor, req.Resources)
	if err != nil {
		return fail("profile or resources unavailable or mismatched")
	}
	result.Checks.Profile = true
	result.Checks.Resources = true
	result.Checks.ResourcesEnforced = true
	result.Checks.CreateWaitsCompletion = true
	img, _, err := cli.GetImage(req.Image)
	if err != nil || img == nil || img.Fingerprint != req.Image {
		return fail("image unavailable")
	}
	if img.Architecture != arch || img.Type != req.InstanceType {
		return fail("image architecture or type mismatch")
	}
	result.Checks.Image = true
	result.Checks.Architecture = true
	if ctx.Err() != nil {
		return fail("check timed out")
	}
	result.ImageFingerprint = img.Fingerprint
	result.ProvisioningContract = "incus-single-node-sync-v1"
	result.Ready = true
	result.Reason = "create permission, KVM health, and host-wide capacity are not verified"
	return result
}

// admissionProfile snapshots the exact effective profile used for an admitted VM.
// No profile references may be sent with the create: they could change later.
func (l *Incus) admissionProfile(ctx context.Context, flavor string, resources ReadinessResources) (map[string]string, map[string]map[string]string, error) {
	if resources.CPU <= 0 || resources.MemoryBytes <= 0 || resources.DiskBytes <= 0 || resources.VMSlots != 1 {
		return nil, nil, fmt.Errorf("invalid admitted resource vector")
	}
	cli, err := l.getCLI(ctx)
	if err != nil {
		return nil, nil, err
	}
	reader, ok := cli.(profileReader)
	if !ok {
		return nil, nil, fmt.Errorf("profile inspection unsupported")
	}
	names, err := l.getProfiles(ctx, flavor)
	if err != nil {
		return nil, nil, err
	}
	config := map[string]string{}
	devices := map[string]map[string]string{}
	for _, name := range names {
		p, _, err := reader.GetProfile(name)
		if err != nil || p == nil {
			return nil, nil, fmt.Errorf("profile unavailable")
		}
		for k, v := range p.Config {
			config[k] = v
		}
		for k, v := range p.Devices {
			copied := map[string]string{}
			for key, value := range v {
				copied[key] = value
			}
			devices[k] = copied
		}
	}
	for key := range config {
		if strings.HasPrefix(key, "raw.") {
			return nil, nil, fmt.Errorf("raw instance settings unsupported")
		}
	}
	cpu, err := strconv.ParseInt(config["limits.cpu"], 10, 64)
	if err != nil || cpu != resources.CPU {
		return nil, nil, fmt.Errorf("CPU allocation mismatch")
	}
	memory, err := units.ParseByteSizeString(config["limits.memory"])
	if err != nil || memory != resources.MemoryBytes {
		return nil, nil, fmt.Errorf("memory allocation mismatch")
	}
	var root map[string]string
	for _, device := range devices {
		switch device["type"] {
		case "nic":
			if device["nictype"] == "physical" || device["nictype"] == "sriov" {
				return nil, nil, fmt.Errorf("physical NIC unsupported")
			}
			continue
		case "disk":
		default:
			return nil, nil, fmt.Errorf("unsupported admitted device type")
		}
		if device["path"] != "/" || root != nil {
			return nil, nil, fmt.Errorf("additional or ambiguous disk unsupported")
		}
		root = device
	}
	if root == nil || root["pool"] == "" {
		return nil, nil, fmt.Errorf("root storage unavailable")
	}
	disk, err := units.ParseByteSizeString(root["size"])
	if err != nil || disk != resources.DiskBytes {
		return nil, nil, fmt.Errorf("disk allocation mismatch")
	}
	config["limits.cpu"] = strconv.FormatInt(resources.CPU, 10)
	config["limits.memory"] = strconv.FormatInt(resources.MemoryBytes, 10)
	root["size"] = strconv.FormatInt(resources.DiskBytes, 10)
	return config, devices, nil
}

// ParseReadinessRequest validates protocol identity and bounded input separately
// from the provider probe.
func (r ReadinessRequest) Validate() error {
	if r.Version != 1 || r.RequestID == "" || r.Generation == "" || r.Domain == "" {
		return fmt.Errorf("invalid readiness request")
	}
	return nil
}
