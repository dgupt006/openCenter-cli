package gitops

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// The base-stage Flux Kustomization healthChecks reference the HelmRelease by
// metadata.name. Three services ship a HelmRelease whose name differs from the
// service key (openstack-ccm -> openstack-cloud-controller-manager,
// openstack-csi -> openstack-cinder-csi, gateway-api -> envoy-gateway-api);
// watching the service key instead targets a resource that never exists, so
// the Kustomization never reports Ready.

func TestCatalogHelmReleaseNamesMatchBaseRepo(t *testing.T) {
	cases := map[string]string{
		"openstack-ccm": "openstack-cloud-controller-manager",
		"openstack-csi": "openstack-cinder-csi",
		"gateway-api":   "envoy-gateway-api",
	}
	catalog := newBuiltInRenderCatalog()
	for service, want := range cases {
		spec, ok := catalog.Lookup(service)
		if !ok {
			t.Fatalf("%s missing from render catalog", service)
		}
		if spec.HelmReleaseName != want {
			t.Errorf("%s: HelmReleaseName = %q, want %q", service, spec.HelmReleaseName, want)
		}
	}
}

// Every other catalog service must leave HelmReleaseName empty so the context
// builder falls back to the service name; a stray value silently renames its
// health check.
func TestCatalogHelmReleaseNameDefaultsToServiceName(t *testing.T) {
	catalog := newBuiltInRenderCatalog()
	overrides := map[string]bool{
		"openstack-ccm": true, "openstack-csi": true, "gateway-api": true,
	}
	for _, spec := range catalog.specs {
		if overrides[spec.ServiceName] {
			continue
		}
		if spec.HelmReleaseName != "" {
			t.Errorf("%s: unexpected HelmReleaseName %q", spec.ServiceName, spec.HelmReleaseName)
		}
	}
}

func TestAutoContextHelmReleaseNameFallback(t *testing.T) {
	cfg, err := v2.NewV2Default("hc-fallback", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	ccm := buildAutoServiceContext("openstack-ccm", extractBaseConfig(cfg.OpenCenter.Services["openstack-ccm"]), *cfg)
	if ccm.HelmReleaseName != "openstack-cloud-controller-manager" {
		t.Errorf("openstack-ccm context HelmReleaseName = %q", ccm.HelmReleaseName)
	}
	velero := buildAutoServiceContext("velero", extractBaseConfig(cfg.OpenCenter.Services["velero"]), *cfg)
	if velero.HelmReleaseName != "velero" {
		t.Errorf("velero context HelmReleaseName = %q, want service-name fallback", velero.HelmReleaseName)
	}
}

func TestTwoStageTemplateRendersHelmReleaseName(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName: "openstack-ccm", Namespace: "openstack-ccm",
		SourceName:   "opencenter-openstack-ccm",
		BasePath:     "applications/base/services/openstack-ccm",
		FluxInterval: "5m",
	}
	ctx.HelmReleaseName = kustomizationName(ctx.ServiceName, "openstack-cloud-controller-manager")
	out, err := renderInlineAutoTemplate(autoFluxTwoStageTemplate, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "kind: HelmRelease\n      name: openstack-cloud-controller-manager\n") {
		t.Errorf("healthChecks must target the real HelmRelease name:\n%s", out)
	}
	if strings.Contains(out, "kind: HelmRelease\n      name: openstack-ccm\n") {
		t.Error("healthChecks still target the service key")
	}
}
