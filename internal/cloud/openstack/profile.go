package openstack

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gophercloud/gophercloud"
	gopenstack "github.com/gophercloud/gophercloud/openstack"
	"gopkg.in/yaml.v3"
)

// Profile is the read-only representation of one clouds.yaml profile. Secret
// fields are retained for authentication but are never included in results.
type Profile struct {
	Name          string
	AuthURL       string
	Username      string
	UserID        string
	Password      string
	ProjectID     string
	ProjectName   string
	Region        string
	S3Endpoint    string
	Interface     string
	DomainName    string
	UserDomain    string
	ProjectDomain string
	AppCredID     string
	AppCredName   string
	AppCredSecret string
	CACert        string
	Cert          string
	Key           string
	Insecure      bool
	Verify        *bool
}

type providerCloudsFile struct {
	Clouds map[string]providerCloudProfile `yaml:"clouds"`
}

type providerCloudProfile struct {
	Auth       providerCloudAuth `yaml:"auth"`
	RegionName string            `yaml:"region_name"`
	S3Endpoint string            `yaml:"s3_endpoint"`
	Interface  string            `yaml:"interface"`
	CACert     string            `yaml:"cacert"`
	Cert       string            `yaml:"cert"`
	Key        string            `yaml:"key"`
	Insecure   bool              `yaml:"insecure"`
	Verify     *bool             `yaml:"verify"`
}

type providerCloudAuth struct {
	AuthURL                     string `yaml:"auth_url"`
	Username                    string `yaml:"username"`
	UserID                      string `yaml:"user_id"`
	Password                    string `yaml:"password"`
	ProjectID                   string `yaml:"project_id"`
	ProjectName                 string `yaml:"project_name"`
	ProjectDomainName           string `yaml:"project_domain_name"`
	UserDomainName              string `yaml:"user_domain_name"`
	DomainName                  string `yaml:"domain_name"`
	ApplicationCredentialID     string `yaml:"application_credential_id"`
	ApplicationCredentialName   string `yaml:"application_credential_name"`
	ApplicationCredentialSecret string `yaml:"application_credential_secret"`
}

// DefaultCloudsYAMLPath returns the standard OpenStack client configuration
// path, respecting the conventional override used by openstacksdk and OSC.
func DefaultCloudsYAMLPath() string {
	if path := strings.TrimSpace(os.Getenv("OS_CLIENT_CONFIG_FILE")); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "clouds.yaml"
	}
	return filepath.Join(home, ".config", "openstack", "clouds.yaml")
}

// LoadProfile reads a selected profile without authenticating or mutating a
// remote resource.
func LoadProfile(path, name string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read clouds.yaml: %w", err)
	}
	var file providerCloudsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return Profile{}, fmt.Errorf("parse clouds.yaml: %w", err)
	}
	selected, ok := file.Clouds[name]
	if !ok {
		return Profile{}, fmt.Errorf("cloud profile %q not found in %s", name, path)
	}
	if strings.TrimSpace(selected.Auth.AuthURL) == "" {
		return Profile{}, fmt.Errorf("cloud profile %q has no auth.auth_url", name)
	}
	return Profile{
		Name: name, AuthURL: strings.TrimSpace(selected.Auth.AuthURL), Username: selected.Auth.Username,
		UserID: selected.Auth.UserID, Password: selected.Auth.Password, ProjectID: selected.Auth.ProjectID,
		ProjectName: selected.Auth.ProjectName, Region: selected.RegionName, S3Endpoint: selected.S3Endpoint, Interface: selected.Interface,
		DomainName: selected.Auth.DomainName, UserDomain: selected.Auth.UserDomainName,
		ProjectDomain: selected.Auth.ProjectDomainName, AppCredID: selected.Auth.ApplicationCredentialID,
		AppCredName: selected.Auth.ApplicationCredentialName, AppCredSecret: selected.Auth.ApplicationCredentialSecret,
		CACert: selected.CACert, Cert: selected.Cert, Key: selected.Key, Insecure: selected.Insecure, Verify: selected.Verify,
	}, nil
}

func (p Profile) AuthOptions() gophercloud.AuthOptions {
	domain := firstProfileValue(p.UserDomain, p.DomainName)
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: p.AuthURL, Username: p.Username, UserID: p.UserID, Password: p.Password,
		DomainName: domain, TenantID: p.ProjectID, TenantName: p.ProjectName,
		ApplicationCredentialID: p.AppCredID, ApplicationCredentialName: p.AppCredName,
		ApplicationCredentialSecret: p.AppCredSecret, AllowReauth: true,
	}
	if opts.TenantID == "" && opts.TenantName != "" && p.ProjectDomain != "" {
		opts.Scope = &gophercloud.AuthScope{ProjectName: opts.TenantName, DomainName: p.ProjectDomain}
	}
	return opts
}

func (p Profile) httpClient() (http.Client, error) {
	verify := true
	if p.Verify != nil {
		verify = *p.Verify
	}
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if strings.TrimSpace(p.CACert) != "" {
		caPath := p.CACert
		if expanded, expandErr := filepath.Abs(caPath); expandErr == nil {
			caPath = expanded
		}
		ca, readErr := os.ReadFile(caPath)
		if readErr != nil {
			return http.Client{}, fmt.Errorf("read OpenStack CA certificate: %w", readErr)
		}
		if !rootCAs.AppendCertsFromPEM(ca) {
			return http.Client{}, fmt.Errorf("parse OpenStack CA certificate %s", caPath)
		}
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs: rootCAs, InsecureSkipVerify: p.Insecure || !verify, //nolint:gosec // explicitly configured by the selected profile
	}}
	if p.Cert != "" || p.Key != "" {
		cert, certErr := tls.LoadX509KeyPair(p.Cert, p.Key)
		if certErr != nil {
			return http.Client{}, fmt.Errorf("load OpenStack client certificate: %w", certErr)
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}
	return http.Client{Transport: transport}, nil
}

func (p Profile) provider(ctx context.Context) (*gophercloud.ProviderClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client, err := gopenstack.NewClient(p.AuthURL)
	if err != nil {
		return nil, fmt.Errorf("create OpenStack client: %w", err)
	}
	client.Context = ctx
	client.HTTPClient, err = p.httpClient()
	if err != nil {
		return nil, err
	}
	if err := gopenstack.Authenticate(client, p.AuthOptions()); err != nil {
		return nil, fmt.Errorf("authenticate OpenStack client: %w", err)
	}
	return client, nil
}

func firstProfileValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
