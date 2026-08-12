// Package openstacksync reconciles OpenCenter's OpenStack settings with a
// selected clouds.yaml profile.  The OpenStack API is deliberately injected so
// the reconciliation and YAML changes remain deterministic and testable.
package openstacksync

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeS3    Mode = "s3"
	ModeSwift Mode = "swift"
)

type ServiceMode struct {
	Service string
	Mode    Mode
}

var SupportedServiceModes = map[string]map[Mode]bool{
	"loki": {ModeS3: true, ModeSwift: true}, "tempo": {ModeS3: true, ModeSwift: true},
	"etcd-backup": {ModeS3: true}, "velero": {ModeS3: true},
}

func ParseServiceModes(input string) ([]ServiceMode, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	var modes []ServiceMode
	for _, item := range strings.Split(input, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid service mapping %q", item)
		}
		service, mode := strings.TrimSpace(parts[0]), Mode(strings.TrimSpace(parts[1]))
		if service == "" || !SupportedServiceModes[service][mode] || seen[service] {
			return nil, fmt.Errorf("unsupported or duplicate service mapping %q", item)
		}
		seen[service] = true
		modes = append(modes, ServiceMode{service, mode})
	}
	return modes, nil
}

type Flavor struct {
	ID, Name         string
	VCPUs, RAM, Disk int
}
type VolumeType struct {
	ID, Name string
	IsPublic bool
}
type Network struct{ ID, Name string }
type Subnet struct{ ID, Name, NetworkID string }

type CoreDiscovery struct {
	AuthURL, Region, ProjectID, ProjectName                    string
	ApplicationCredentialID, ApplicationCredentialSecret       string
	UserDomainName, ProjectDomainName                          string
	ImageID, ImageIDWindows, ExternalNetworkID, FloatingIPPool string
	AvailabilityZones                                          []string
	InternalNetworks                                           []Network
	InternalSubnets                                            []Subnet
	SwiftEndpoint                                              string
	Flavors                                                    []Flavor
	VolumeTypes                                                []VolumeType
}

type ContainerRequest struct {
	Name, Region     string
	EnableVersioning bool
}
type EC2CredentialRequest struct{ ProjectID, ProjectName, UserName string }
type EC2Credentials struct{ AccessKeyID, SecretAccessKey, Endpoint, Region, ProjectID string }
type AccessRule struct{ Service, Method, Path string }
type AppCredentialRequest struct {
	Name, Description string
	AccessRules       []AccessRule
}
type AppCredential struct{ ID, Secret string }

type Dependencies struct {
	DiscoverCore         func(context.Context, string) (CoreDiscovery, error)
	EnsureContainer      func(context.Context, ContainerRequest) error
	CreateEC2Credentials func(context.Context, EC2CredentialRequest) (EC2Credentials, error)
	CreateAppCredential  func(context.Context, AppCredentialRequest) (AppCredential, error)
}

type Options struct {
	ConfigPath, OSCloud, SubnetID                                                      string
	ServiceModes                                                                       []ServiceMode
	DryRun, RotateCreds, NoScopeCreds, MatchFlavors, MatchVolumeType, ServicesExplicit bool
}

type FlavorMatch struct {
	Role, FlavorName string
	VCPUs, RAM       int
	RoundedUp        bool
}
type VolumeTypeMatch struct {
	Name           string
	TotalAvailable int
	AvailableNames []string
	IsDefault      bool
}
type Result struct {
	ConfigPath, OSCloud                                       string
	UpdatedYAML                                               []byte `json:"-" yaml:"-"`
	CoreChangedPaths                                          []string
	FlavorMatches                                             []FlavorMatch
	VolumeTypeMatch                                           *VolumeTypeMatch
	ServiceMappings, CredentialActions, SecretPaths, Warnings []string
}

type Service struct{ deps Dependencies }

func NewService(deps Dependencies) (*Service, error) {
	if deps.DiscoverCore == nil {
		return nil, fmt.Errorf("DiscoverCore dependency is required")
	}
	return &Service{deps: deps}, nil
}

// Sync produces an idempotent config update. In dry-run it never invokes
// mutation callbacks and never writes the target file.
func (s *Service) Sync(ctx context.Context, opts Options) (*Result, error) {
	input, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	root := map[string]any{}
	if err := yaml.Unmarshal(input, &root); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	d, err := s.deps.DiscoverCore(ctx, opts.OSCloud)
	if err != nil {
		return nil, fmt.Errorf("discover OpenStack: %w", err)
	}
	subnet, err := selectSubnet(opts.SubnetID, d.InternalSubnets)
	if err != nil {
		return nil, err
	}

	result := &Result{ConfigPath: opts.ConfigPath, OSCloud: opts.OSCloud}

	if subnet == "" && opts.SubnetID == "" && len(d.InternalSubnets) > 1 {
		// Multiple subnets discovered without an explicit override; skip
		// subnet selection so a new subnet will be created during deployment.
		result.Warnings = append(result.Warnings, "multiple subnets found; skipping subnet selection (a new subnet will be created during deployment)")
	} else if subnet == "" {
		// A missing inventory response must not erase a previously configured
		// subnet; it is common for restricted credentials to lack network list
		// visibility while still being able to manage existing resources.
		subnet = getString(root, "opencenter.infrastructure.cloud.openstack.subnet_id")
	}
	zone := getString(root, "opencenter.infrastructure.cloud.openstack.availability_zone")
	if zone == "" {
		zone = firstSorted(d.AvailabilityZones)
	}
	core := map[string]any{
		"auth_url": d.AuthURL, "region": d.Region, "project_id": d.ProjectID, "project_name": d.ProjectName,
		"application_credential_id": d.ApplicationCredentialID, "application_credential_secret": d.ApplicationCredentialSecret,
		"user_domain_name": d.UserDomainName, "project_domain_name": d.ProjectDomainName, "image_id": d.ImageID,
		"image_id_windows": d.ImageIDWindows, "subnet_id": subnet, "router_external_network_id": d.ExternalNetworkID,
		"floating_ip_pool": d.FloatingIPPool, "availability_zone": zone, "availability_zones": sorted(d.AvailabilityZones),
	}
	for key, value := range core {
		path := "opencenter.infrastructure.cloud.openstack." + key
		if !equalAt(root, path, value) {
			result.CoreChangedPaths = append(result.CoreChangedPaths, path)
		}
		set(root, path, value)
	}
	sort.Strings(result.CoreChangedPaths)
	for _, key := range []string{"subnet_id", "router_external_network_id", "floating_ip_pool"} {
		set(root, "opencenter.infrastructure.cloud.openstack.networking."+key, core[key])
	}

	if opts.MatchFlavors {
		result.FlavorMatches = applyFlavorMatches(root, d.Flavors)
	}
	if opts.MatchVolumeType {
		result.VolumeTypeMatch = applyVolumeType(root, d.VolumeTypes)
	}
	modes := opts.ServiceModes
	if !opts.ServicesExplicit && len(modes) == 0 {
		modes = detectModes(root)
	}
	for _, mode := range modes {
		name := storageName(root, mode)
		result.ServiceMappings = append(result.ServiceMappings, mode.Service+"="+string(mode.Mode))
		if opts.DryRun {
			result.CredentialActions = append(result.CredentialActions, "would ensure container "+name)
		} else {
			if s.deps.EnsureContainer == nil {
				return nil, fmt.Errorf("EnsureContainer dependency is required for service wiring")
			}
			if err := s.deps.EnsureContainer(ctx, ContainerRequest{Name: name, Region: d.Region}); err != nil {
				return nil, fmt.Errorf("ensure %s storage: %w", mode.Service, err)
			}
			result.CredentialActions = append(result.CredentialActions, "ensured container "+name)
		}
		if err := s.applyServiceCredentials(ctx, root, mode, name, d, opts, result); err != nil {
			return nil, err
		}
	}
	output, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	result.UpdatedYAML = output
	if !opts.DryRun {
		if err := writeAtomic(opts.ConfigPath, output); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Service) applyServiceCredentials(ctx context.Context, root map[string]any, mode ServiceMode, storage string, d CoreDiscovery, opts Options, result *Result) error {
	if mode.Mode == ModeSwift {
		idPath := fmt.Sprintf("opencenter.services.%s.swift_application_credential_id", mode.Service)
		secretPath := fmt.Sprintf("secrets.%s.swift_application_credential_secret", mode.Service)
		credentialID, secret := getString(root, idPath), getString(root, secretPath)
		if opts.RotateCreds || credentialID == "" || secret == "" {
			if opts.DryRun {
				credentialID, secret = "CHANGEME", "CHANGEME"
				result.CredentialActions = append(result.CredentialActions, plannedCredentialAction(mode.Service, ModeSwift, opts.RotateCreds))
			} else {
				if s.deps.CreateAppCredential == nil {
					return fmt.Errorf("CreateAppCredential dependency is required for %s=swift", mode.Service)
				}
				req := AppCredentialRequest{Name: storage + "-swift-appcred"}
				if !opts.NoScopeCreds && d.ProjectID != "" {
					req.AccessRules = buildContainerAccessRules(d.ProjectID, storage)
					req.Description = "container-scoped: " + storage
				} else if !opts.NoScopeCreds {
					result.Warnings = append(result.Warnings, fmt.Sprintf("cannot scope %s Swift credential to a container: project ID not available; use --no-scope-creds to suppress", mode.Service))
				}
				credential, err := s.deps.CreateAppCredential(ctx, req)
				if err != nil {
					return fmt.Errorf("create %s application credential: %w", mode.Service, err)
				}
				credentialID, secret = credential.ID, credential.Secret
				result.CredentialActions = append(result.CredentialActions, appliedCredentialAction(mode.Service, ModeSwift, opts.RotateCreds))
			}
		} else {
			result.CredentialActions = append(result.CredentialActions, "reused existing swift credentials for "+mode.Service)
		}
		base := "opencenter.services." + mode.Service
		set(root, base+".swift_auth_url", d.AuthURL)
		set(root, base+".swift_region", d.Region)
		set(root, base+".swift_auth_version", 3)
		set(root, idPath, credentialID)
		set(root, base+".swift_container_name", storage)
		set(root, base+".swift_user_domain_name", d.UserDomainName)
		set(root, base+".swift_domain_name", d.ProjectDomainName)
		set(root, secretPath, secret)
		if mode.Service == "loki" {
			set(root, base+".loki_storage_type", "swift")
		} else {
			set(root, base+".storage_type", "swift")
		}
		result.SecretPaths = append(result.SecretPaths, secretPath)
		return nil
	}
	accessPath, secretPath, err := s3SecretPaths(mode.Service)
	if err != nil {
		return err
	}
	accessKey, secretKey := getString(root, accessPath), getString(root, secretPath)
	endpoint, region := d.SwiftEndpoint, d.Region
	if opts.RotateCreds || accessKey == "" || secretKey == "" {
		if opts.DryRun {
			accessKey, secretKey = "CHANGEME", "CHANGEME"
			result.CredentialActions = append(result.CredentialActions, plannedCredentialAction(mode.Service, ModeS3, opts.RotateCreds))
		} else {
			if s.deps.CreateEC2Credentials == nil {
				return fmt.Errorf("CreateEC2Credentials dependency is required for %s=s3", mode.Service)
			}
			credential, err := s.deps.CreateEC2Credentials(ctx, EC2CredentialRequest{ProjectID: d.ProjectID, ProjectName: storage, UserName: storage + "-s3-user"})
			if err != nil {
				return fmt.Errorf("create %s S3 credentials: %w", mode.Service, err)
			}
			accessKey, secretKey = credential.AccessKeyID, credential.SecretAccessKey
			if credential.Endpoint != "" {
				endpoint = credential.Endpoint
			}
			if credential.Region != "" {
				region = credential.Region
			}
			result.CredentialActions = append(result.CredentialActions, appliedCredentialAction(mode.Service, ModeS3, opts.RotateCreds))
			result.Warnings = append(result.Warnings, fmt.Sprintf("EC2 credentials for %s=s3 are project-scoped; use %s=swift for container-level scoping", mode.Service, mode.Service))
		}
	} else {
		result.CredentialActions = append(result.CredentialActions, "reused existing s3 credentials for "+mode.Service)
	}
	set(root, accessPath, accessKey)
	set(root, secretPath, secretKey)
	base := "opencenter.services." + mode.Service
	switch mode.Service {
	case "etcd-backup":
		set(root, base+".s3_host", extractHost(endpoint))
		set(root, base+".s3_region", region)
	case "loki":
		set(root, base+".loki_storage_type", "s3")
		set(root, base+".loki_bucket_name", storage)
		set(root, base+".loki_s3_endpoint", endpoint)
		set(root, base+".loki_s3_region", region)
		set(root, base+".loki_s3_force_path_style", true)
		set(root, base+".loki_s3_insecure", false)
	case "tempo":
		set(root, base+".storage_type", "s3")
		set(root, base+".bucket_name", storage)
		set(root, base+".s3_endpoint", endpoint)
		set(root, base+".s3_region", region)
		set(root, base+".s3_force_path_style", true)
		set(root, base+".s3_insecure", false)
	case "velero":
		set(root, base+".storage_type", "s3")
		set(root, base+".velero_backup_bucket", storage)
		set(root, base+".velero_region", region)
	}
	result.SecretPaths = append(result.SecretPaths, accessPath, secretPath)
	return nil
}

func selectSubnet(override string, subnets []Subnet) (string, error) {
	if override != "" {
		return override, nil
	}
	if len(subnets) == 1 {
		return subnets[0].ID, nil
	}
	// Zero or multiple subnets: return empty so the caller falls back to the
	// existing config value or leaves it unset (new-cluster behaviour).
	return "", nil
}
func sorted(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
func firstSorted(values []string) string {
	values = sorted(values)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func applyFlavorMatches(root map[string]any, flavors []Flavor) []FlavorMatch {
	targets := []struct {
		role, path string
		cpu, ram   int
	}{{"bastion", "flavor_bastion", 2, 4096}, {"controller", "flavor_master", 4, 8192}, {"worker", "flavor_worker", 8, 16384}}
	var matches []FlavorMatch
	for _, target := range targets {
		if f, ok := pickFlavor(flavors, target.cpu, target.ram); ok {
			set(root, "opencenter.infrastructure.compute."+target.path, f.Name)
			matches = append(matches, FlavorMatch{target.role, f.Name, f.VCPUs, f.RAM, f.VCPUs > target.cpu || f.RAM > target.ram})
		}
	}
	return matches
}
func pickFlavor(flavors []Flavor, cpu, ram int) (Flavor, bool) {
	var candidates []Flavor
	for _, f := range flavors {
		if f.VCPUs >= cpu && f.RAM >= ram {
			candidates = append(candidates, f)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].VCPUs != candidates[j].VCPUs {
			return candidates[i].VCPUs < candidates[j].VCPUs
		}
		if candidates[i].RAM != candidates[j].RAM {
			return candidates[i].RAM < candidates[j].RAM
		}
		return candidates[i].Disk < candidates[j].Disk
	})
	if len(candidates) == 0 {
		return Flavor{}, false
	}
	return candidates[0], true
}
func applyVolumeType(root map[string]any, types []VolumeType) *VolumeTypeMatch {
	if len(types) == 0 {
		return nil
	}
	available := make([]string, 0, len(types))
	var public []VolumeType
	for _, t := range types {
		available = append(available, t.Name)
		if t.IsPublic {
			public = append(public, t)
		}
	}
	if len(public) == 0 {
		public = types
	}
	sort.Strings(available)
	sort.Slice(public, func(i, j int) bool { return public[i].Name < public[j].Name })
	chosen := public[0]
	for _, t := range public {
		if strings.EqualFold(t.Name, "Performance") {
			chosen = t
			break
		}
	}
	set(root, "opencenter.infrastructure.storage.worker_volume_type", chosen.Name)
	set(root, "opencenter.infrastructure.storage.master_volume_type", chosen.Name)
	return &VolumeTypeMatch{Name: chosen.Name, TotalAvailable: len(types), AvailableNames: available, IsDefault: strings.EqualFold(chosen.Name, "Performance")}
}
func detectModes(root map[string]any) []ServiceMode {
	var out []ServiceMode
	for service, modes := range SupportedServiceModes {
		if getBool(root, "opencenter.services."+service+".enabled") {
			mode := ModeS3
			if modes[ModeSwift] && (getString(root, "opencenter.services."+service+".storage_type") == "swift" || getString(root, "opencenter.services."+service+".loki_storage_type") == "swift") {
				mode = ModeSwift
			}
			out = append(out, ServiceMode{service, mode})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out
}
func storageName(root map[string]any, serviceMode ServiceMode) string {
	for _, p := range storageNamePaths(serviceMode) {
		if v := getString(root, p); v != "" {
			return v
		}
	}
	cluster := getString(root, "opencenter.cluster.cluster_name")
	if cluster == "" {
		cluster = "opencenter"
	}
	return cluster + "-" + serviceMode.Service
}
func storageNamePaths(serviceMode ServiceMode) []string {
	if serviceMode.Mode == ModeSwift {
		return []string{"opencenter.services." + serviceMode.Service + ".swift_container_name"}
	}
	switch serviceMode.Service {
	case "loki":
		return []string{"opencenter.services.loki.loki_bucket_name"}
	case "tempo":
		return []string{"opencenter.services.tempo.bucket_name"}
	case "velero":
		return []string{"opencenter.services.velero.velero_backup_bucket"}
	}
	return nil
}
func s3SecretPaths(service string) (string, string, error) {
	base := "secrets." + service
	switch service {
	case "loki", "tempo", "etcd-backup", "velero":
		return base + ".s3_access_key_id", base + ".s3_secret_access_key", nil
	}
	return "", "", fmt.Errorf("unsupported service %q", service)
}
func plannedCredentialAction(service string, mode Mode, rotate bool) string {
	verb := "would create"
	if rotate {
		verb = "would rotate"
	}
	return fmt.Sprintf("%s %s credentials for %s", verb, mode, service)
}
func appliedCredentialAction(service string, mode Mode, rotate bool) string {
	verb := "created"
	if rotate {
		verb = "rotated"
	}
	return fmt.Sprintf("%s %s credentials for %s", verb, mode, service)
}
func buildContainerAccessRules(projectID, containerName string) []AccessRule {
	containerPath := fmt.Sprintf("/v1/AUTH_%s/%s", projectID, containerName)
	segmentsPath := containerPath + "_segments"
	var rules []AccessRule
	for _, path := range []string{containerPath, segmentsPath} {
		rules = append(rules, AccessRule{Service: "object-store", Method: "GET", Path: path}, AccessRule{Service: "object-store", Method: "HEAD", Path: path})
		methods := []string{"GET", "HEAD", "PUT", "DELETE"}
		if path == containerPath {
			methods = append(methods, "POST")
		}
		for _, method := range methods {
			rules = append(rules, AccessRule{Service: "object-store", Method: method, Path: path + "/**"})
		}
	}
	return rules
}
func extractHost(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
}

func get(root map[string]any, path string) (any, bool) {
	var current any = root
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
func getString(root map[string]any, path string) string {
	v, _ := get(root, path)
	s, _ := v.(string)
	return s
}
func getBool(root map[string]any, path string) bool {
	v, _ := get(root, path)
	b, _ := v.(bool)
	return b
}
func equalAt(root map[string]any, path string, want any) bool {
	have, ok := get(root, path)
	return ok && fmt.Sprint(have) == fmt.Sprint(want)
}
func set(root map[string]any, path string, value any) {
	current := root
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}
func writeAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".opencenter-sync-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(info.Mode())
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
