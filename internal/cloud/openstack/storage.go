package openstack

import (
	"context"
	"fmt"
	"strings"

	tokens3 "github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"

	"github.com/gophercloud/gophercloud"
	gopenstack "github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/applicationcredentials"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/extensions/ec2credentials"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/containers"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

// StorageAdapter is the only OpenStack surface used by the one-service storage flow.
type StorageAdapter interface {
	Preflight(context.Context, string, string, bool) (StoragePreflight, error)
	EnsureContainer(context.Context, ContainerRequest) error
	CreateAppCredential(context.Context, AppCredentialRequest) (AppCredential, error)
	DeleteAppCredential(context.Context, string, string) error
	CreateEC2Credentials(context.Context, EC2CredentialRequest) (EC2Credentials, error)
	DeleteEC2Credentials(context.Context, string, string) error
}

type StoragePreflight struct {
	Endpoint          string `json:"endpoint" yaml:"endpoint"`
	S3Endpoint        string `json:"s3_endpoint,omitempty" yaml:"s3_endpoint,omitempty"`
	AuthURL           string `json:"auth_url" yaml:"auth_url"`
	Region            string `json:"region" yaml:"region"`
	ProjectID         string `json:"project_id" yaml:"project_id"`
	CredentialOwnerID string `json:"-" yaml:"-"`
}

type ContainerRequest struct {
	Name   string
	Region string
}

type AccessRule struct {
	Service string
	Method  string
	Path    string
}

type AppCredentialRequest struct {
	UserID      string
	Name        string
	Description string
	AccessRules []AccessRule
}

type AppCredential struct {
	ID     string
	Secret string
}

type EC2CredentialRequest struct {
	UserID      string
	ProjectID   string
	ProjectName string
	UserName    string
}

type EC2Credentials struct {
	ID          string
	AccessKeyID string
	Secret      string
	Endpoint    string
	Region      string
	ProjectID   string
}

type GophercloudStorageAdapter struct{ Profile Profile }

func NewStorageAdapter(profile Profile) *GophercloudStorageAdapter {
	return &GophercloudStorageAdapter{Profile: profile}
}

func (a *GophercloudStorageAdapter) Preflight(ctx context.Context, backend, explicitS3Endpoint string, resolveOwner bool) (StoragePreflight, error) {
	if backend != "swift" && backend != "s3" {
		return StoragePreflight{}, fmt.Errorf("unsupported OpenStack storage backend %q", backend)
	}
	provider, err := a.Profile.provider(ctx)
	if err != nil {
		return StoragePreflight{}, a.maskError(err)
	}
	client, err := gopenstack.NewObjectStorageV1(provider, a.endpointOpts())
	if err != nil {
		return StoragePreflight{}, a.maskError(fmt.Errorf("create object-store client: %w", err))
	}
	projectID := strings.TrimSpace(a.Profile.ProjectID)
	if discoveredID, _, _, scopeErr := authenticatedProjectScope(provider); scopeErr == nil && strings.TrimSpace(discoveredID) != "" {
		projectID = discoveredID
	}
	result := StoragePreflight{Endpoint: client.Endpoint, AuthURL: a.Profile.AuthURL, Region: a.Profile.Region, ProjectID: projectID}
	if resolveOwner {
		result.CredentialOwnerID = strings.TrimSpace(a.Profile.UserID)
		if result.CredentialOwnerID == "" {
			result.CredentialOwnerID, err = authenticatedUserID(provider)
			if err != nil {
				return StoragePreflight{}, fmt.Errorf("resolve storage credential owner: %w", err)
			}
		}
	}
	if backend == "s3" {
		result.S3Endpoint = strings.TrimSpace(explicitS3Endpoint)
		if result.S3Endpoint == "" {
			result.S3Endpoint = catalogS3Endpoint(provider, a.endpointOpts())
		}
		if result.S3Endpoint == "" {
			result.S3Endpoint = strings.TrimSpace(a.Profile.S3Endpoint)
		}
		if result.S3Endpoint == "" {
			return StoragePreflight{}, fmt.Errorf("no distinct S3-compatible endpoint found; specify --s3-endpoint")
		}
	}
	return result, nil
}

func catalogS3Endpoint(provider *gophercloud.ProviderClient, opts gophercloud.EndpointOpts) string {
	if provider == nil || provider.GetAuthResult() == nil {
		return ""
	}
	var catalog *tokens3.ServiceCatalog
	switch result := provider.GetAuthResult().(type) {
	case tokens3.CreateResult:
		catalog, _ = result.ExtractServiceCatalog()
	case tokens3.GetResult:
		catalog, _ = result.ExtractServiceCatalog()
	}
	if catalog == nil {
		return ""
	}
	for _, entry := range catalog.Entries {
		if !strings.EqualFold(strings.TrimSpace(entry.Type), "s3") && !strings.EqualFold(strings.TrimSpace(entry.Name), "s3") {
			continue
		}
		for _, endpoint := range entry.Endpoints {
			if strings.TrimSpace(endpoint.URL) == "" {
				continue
			}
			if opts.Region != "" && endpoint.Region != "" && endpoint.Region != opts.Region {
				continue
			}
			if opts.Availability != "" && endpoint.Interface != "" && !strings.EqualFold(endpoint.Interface, string(opts.Availability)) {
				continue
			}
			return strings.TrimSpace(endpoint.URL)
		}
	}
	return ""
}

func (a *GophercloudStorageAdapter) EnsureContainer(ctx context.Context, req ContainerRequest) error {
	provider, err := a.Profile.provider(ctx)
	if err != nil {
		return a.maskError(err)
	}
	client, err := gopenstack.NewObjectStorageV1(provider, a.endpointOptsFor(req.Region))
	if err != nil {
		return a.maskError(fmt.Errorf("create object-store client: %w", err))
	}
	_, err = containers.Create(client, req.Name, containers.CreateOpts{}).Extract()
	if err != nil && !strings.Contains(err.Error(), "409") {
		return a.maskError(fmt.Errorf("ensure Swift container: %w", err))
	}
	return nil
}

func (a *GophercloudStorageAdapter) CreateAppCredential(ctx context.Context, req AppCredentialRequest) (AppCredential, error) {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return AppCredential{}, fmt.Errorf("credential owner user ID is required for application credential creation")
	}
	provider, err := a.Profile.provider(ctx)
	if err != nil {
		return AppCredential{}, a.maskError(err)
	}
	identity, err := gopenstack.NewIdentityV3(provider, a.endpointOpts())
	if err != nil {
		return AppCredential{}, a.maskError(fmt.Errorf("create identity client: %w", err))
	}
	rules := make([]applicationcredentials.AccessRule, 0, len(req.AccessRules))
	for _, rule := range req.AccessRules {
		rules = append(rules, applicationcredentials.AccessRule{Service: rule.Service, Method: rule.Method, Path: rule.Path})
	}
	credential, err := applicationcredentials.Create(identity, userID, applicationcredentials.CreateOpts{
		Name: req.Name, Description: req.Description, AccessRules: rules,
	}).Extract()
	if err != nil {
		return AppCredential{}, a.maskError(fmt.Errorf("create Swift application credential: %w", err))
	}
	return AppCredential{ID: credential.ID, Secret: credential.Secret}, nil
}

func (a *GophercloudStorageAdapter) DeleteAppCredential(ctx context.Context, id, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("credential owner user ID is required for application credential deletion")
	}
	provider, err := a.Profile.provider(ctx)
	if err != nil {
		return a.maskError(err)
	}
	identity, err := gopenstack.NewIdentityV3(provider, a.endpointOpts())
	if err != nil {
		return a.maskError(fmt.Errorf("create identity client: %w", err))
	}
	return a.maskError(applicationcredentials.Delete(identity, userID, id).ExtractErr())
}

func (a *GophercloudStorageAdapter) CreateEC2Credentials(ctx context.Context, req EC2CredentialRequest) (EC2Credentials, error) {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return EC2Credentials{}, fmt.Errorf("credential owner user ID is required for EC2 credential creation")
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		return EC2Credentials{}, fmt.Errorf("project ID is required for EC2 credential creation")
	}
	provider, err := a.Profile.provider(ctx)
	if err != nil {
		return EC2Credentials{}, a.maskError(err)
	}
	identity, err := gopenstack.NewIdentityV3(provider, a.endpointOpts())
	if err != nil {
		return EC2Credentials{}, a.maskError(fmt.Errorf("create identity client: %w", err))
	}
	credential, err := ec2credentials.Create(identity, userID, ec2credentials.CreateOpts{TenantID: req.ProjectID}).Extract()
	if err != nil {
		return EC2Credentials{}, a.maskError(fmt.Errorf("create EC2 credential: %w", err))
	}
	s3Endpoint := strings.TrimSpace(a.Profile.S3Endpoint)
	if s3Endpoint == "" {
		s3Endpoint = catalogS3Endpoint(provider, a.endpointOpts())
	}
	return EC2Credentials{ID: credential.Access, AccessKeyID: credential.Access, Secret: credential.Secret, Endpoint: s3Endpoint, Region: a.Profile.Region, ProjectID: req.ProjectID}, nil
}

func (a *GophercloudStorageAdapter) DeleteEC2Credentials(ctx context.Context, id, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("credential owner user ID is required for EC2 credential deletion")
	}
	provider, err := a.Profile.provider(ctx)
	if err != nil {
		return a.maskError(err)
	}
	identity, err := gopenstack.NewIdentityV3(provider, a.endpointOpts())
	if err != nil {
		return a.maskError(fmt.Errorf("create identity client: %w", err))
	}
	return a.maskError(ec2credentials.Delete(identity, userID, id).ExtractErr())
}

func (a *GophercloudStorageAdapter) maskError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", security.MaskSecrets(err.Error(), a.Profile.Password, a.Profile.AppCredSecret))
}

func (a *GophercloudStorageAdapter) endpointOpts() gophercloud.EndpointOpts {
	return a.endpointOptsFor(a.Profile.Region)
}

func (a *GophercloudStorageAdapter) endpointOptsFor(region string) gophercloud.EndpointOpts {
	region = strings.TrimSpace(region)
	if region == "" {
		region = strings.TrimSpace(a.Profile.Region)
	}
	opts := gophercloud.EndpointOpts{Region: region}
	if strings.TrimSpace(a.Profile.Interface) != "" {
		opts.Availability = gophercloud.Availability(strings.TrimSpace(a.Profile.Interface))
	}
	return opts
}
