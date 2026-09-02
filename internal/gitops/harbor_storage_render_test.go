package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

func TestRenderHarborPVCDefaultsAndInfrastructureStorageClass(t *testing.T) {
	cfg := newDefault("harbor-storage-defaults")
	values := renderHarborValues(t, cfg)

	pvc := harborPVCValues(t, values)
	for component, want := range map[string]string{
		"registry":   "100Gi",
		"jobservice": "10Gi",
		"database":   "10Gi",
		"redis":      "10Gi",
		"trivy":      "10Gi",
	} {
		got := harborPVCSize(t, pvc, component)
		if got != want {
			t.Errorf("%s PVC size = %q, want %q", component, got, want)
		}
		if got := harborPVCStorageClass(t, pvc, component); got != cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass {
			t.Errorf("%s PVC storageClass = %q, want infrastructure default %q", component, got, cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass)
		}
	}

	registry := pvc["registry"].(map[string]any)
	if values["persistence"].(map[string]any)["imageChartStorage"] == nil {
		t.Fatal("Harbor imageChartStorage must remain configured for object storage")
	}
	if registry["size"] == nil {
		t.Fatal("Harbor registry PVC must remain present as chart-required cache/state")
	}
}

func TestRenderHarborPVCExplicitSizesAndStorageClass(t *testing.T) {
	cfg := newDefault("harbor-storage-custom")
	harbor := cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig)
	harbor.RegistryVolumeSize = 40
	harbor.JobserviceVolumeSize = 2
	harbor.DatabaseVolumeSize = 8
	harbor.RedisVolumeSize = 2
	harbor.TrivyVolumeSize = 3
	harbor.StorageClass = "harbor-fast"

	pvc := harborPVCValues(t, renderHarborValues(t, cfg))
	for component, want := range map[string]string{
		"registry":   "40Gi",
		"jobservice": "2Gi",
		"database":   "8Gi",
		"redis":      "2Gi",
		"trivy":      "3Gi",
	} {
		if got := harborPVCSize(t, pvc, component); got != want {
			t.Errorf("%s PVC size = %q, want %q", component, got, want)
		}
		if got := harborPVCStorageClass(t, pvc, component); got != harbor.StorageClass {
			t.Errorf("%s PVC storageClass = %q, want explicit Harbor storage class %q", component, got, harbor.StorageClass)
		}
	}
}

func TestHarborTemplatesContainNoComponentWide100GiPolicy(t *testing.T) {
	path := filepath.Join("templates", "cluster-apps-base", "services", "harbor", "helm-values", "override-values.yaml.tpl")
	templateData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Harbor template: %v", err)
	}
	if strings.Contains(string(templateData), "size: 100Gi") {
		t.Fatalf("Harbor template still contains a hard-coded component-wide 100Gi policy")
	}
	if strings.Contains(string(templateData), "rackspacecloud.com") {
		t.Fatal("filesystem Harbor template still contains a hard-coded Rackspace endpoint")
	}
	if strings.Contains(harborTemplate, "size: 100Gi") {
		t.Fatalf("embedded Harbor renderer still contains a hard-coded component-wide 100Gi policy")
	}
	if strings.Contains(harborTemplate, "rackspacecloud.com") {
		t.Fatal("embedded Harbor renderer still contains a hard-coded Rackspace endpoint")
	}
}

func TestHarborEmbeddedRendererMatchesFilesystemTemplate(t *testing.T) {
	path := filepath.Join("templates", "cluster-apps-base", "services", "harbor", "helm-values", "override-values.yaml.tpl")
	templateData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Harbor template: %v", err)
	}

	cfg := newDefault("harbor-template-parity")
	harbor := cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig)
	harbor.RegistryVolumeSize = 40
	harbor.JobserviceVolumeSize = 2
	harbor.DatabaseVolumeSize = 8
	harbor.RedisVolumeSize = 2
	harbor.TrivyVolumeSize = 3
	harbor.StorageClass = "harbor-fast"

	embedded, err := templateRenderer(harborTemplate)(cfg)
	if err != nil {
		t.Fatalf("render embedded Harbor template: %v", err)
	}
	filesystem, err := templateRenderer(string(templateData))(cfg)
	if err != nil {
		t.Fatalf("render filesystem Harbor template: %v", err)
	}
	if embedded != filesystem {
		t.Fatalf("embedded and filesystem Harbor templates diverged:\nembedded:\n%s\nfilesystem:\n%s", embedded, filesystem)
	}
}

func TestRenderHarborUsesConfiguredS3Endpoint(t *testing.T) {
	cfg := newDefault("harbor-endpoint")
	harbor := cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig)
	harbor.S3Endpoint = "https://s3.catalog.example/v1"
	values := renderHarborValues(t, cfg)
	storage := values["persistence"].(map[string]any)["imageChartStorage"].(map[string]any)
	s3 := storage["s3"].(map[string]any)
	if got := s3["regionendpoint"]; got != harbor.S3Endpoint {
		t.Fatalf("Harbor regionendpoint = %v, want %q", got, harbor.S3Endpoint)
	}
	if strings.Contains(string(mustRenderHarborTemplate(t, cfg)), "rackspacecloud.com") {
		t.Fatal("Harbor renderer still contains a hard-coded Rackspace endpoint")
	}
}

func mustRenderHarborTemplate(t *testing.T, cfg v2.Config) string {
	t.Helper()
	values, err := templateRenderer(harborTemplate)(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return values
}

func renderHarborValues(t *testing.T, cfg v2.Config) map[string]any {
	t.Helper()
	values, err := templateRenderer(harborTemplate)(cfg)
	if err != nil {
		t.Fatalf("render Harbor values: %v", err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal([]byte(values), &decoded); err != nil {
		t.Fatalf("parse Harbor values: %v\n%s", err, values)
	}
	return decoded
}

func harborPVCValues(t *testing.T, values map[string]any) map[string]any {
	t.Helper()
	persistence := values["persistence"].(map[string]any)
	return persistence["persistentVolumeClaim"].(map[string]any)
}

func harborPVCSize(t *testing.T, pvc map[string]any, component string) string {
	t.Helper()
	return harborPVCField(t, pvc, component, "size")
}

func harborPVCStorageClass(t *testing.T, pvc map[string]any, component string) string {
	t.Helper()
	return harborPVCField(t, pvc, component, "storageClass")
}

func harborPVCField(t *testing.T, pvc map[string]any, component, field string) string {
	t.Helper()
	var componentValues map[string]any
	if component == "jobservice" {
		componentValues = pvc[component].(map[string]any)["jobLog"].(map[string]any)
	} else {
		componentValues = pvc[component].(map[string]any)
	}
	got, ok := componentValues[field].(string)
	if !ok {
		t.Fatalf("%s PVC %s = %#v, want string", component, field, componentValues[field])
	}
	return got
}
