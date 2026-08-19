package sops

import (
	"reflect"
	"testing"
)

func TestOverlayFilesToEncrypt(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     []string
	}{
		{
			name:     "provider without additional secret files",
			provider: "baremetal",
			want: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				// Service override-values with credentials (always included)
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
			},
		},
		{
			name:     "OpenStack",
			provider: "openstack",
			want: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/openstack-credentials.yaml",
				// OpenStack-specific service override-values with credentials
				"services/openstack-ccm/helm-values/override-values.yaml",
				"services/openstack-csi/helm-values/override-values.yaml",
				// Service override-values with credentials (always included)
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
			},
		},
		{
			name:     "vSphere",
			provider: "vsphere",
			want: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/vsphere-credentials.yaml",
				"customer-managed/services/cloud-provider-vsphere/secret.yaml",
				// Service override-values with credentials (always included)
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newSOPSTestConfig("test-cluster", "baremetal", "")
			cfg.OpenCenter.Infrastructure.Provider = tt.provider

			if got := overlayFilesToEncrypt(cfg); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("overlayFilesToEncrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceOverrideValuesFilesToEncrypt(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     []string
	}{
		{
			name:     "OpenStack includes openstack-ccm and openstack-csi",
			provider: "openstack",
			want: []string{
				"services/openstack-ccm/helm-values/override-values.yaml",
				"services/openstack-csi/helm-values/override-values.yaml",
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
			},
		},
		{
			name:     "non-OpenStack excludes openstack-ccm and openstack-csi",
			provider: "baremetal",
			want: []string{
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
			},
		},
		{
			name:     "vSphere excludes openstack-ccm and openstack-csi",
			provider: "vsphere",
			want: []string{
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newSOPSTestConfig("test-cluster", tt.provider, "")

			got := serviceOverrideValuesFilesToEncrypt(cfg)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("serviceOverrideValuesFilesToEncrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}
