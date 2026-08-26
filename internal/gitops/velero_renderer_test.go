package gitops

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestVeleroRendererOpenStackUsesProviderSafeValues(t *testing.T) {
	cfg := mustNewGitOpsTestConfig("velero-render", "openstack")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "DFW3"
	cfg.OpenCenter.Services["velero"] = &services.VeleroConfig{
		BackupBucket: "custom-backups",
		Region:       "DFW3",
		StorageType:  "swift",
	}

	spec, ok := newBuiltInRenderCatalog().Lookup("velero")
	require.True(t, ok)
	require.NotNil(t, spec.OverrideValuesRenderer)

	rendered, err := spec.OverrideValuesRenderer(cfg)
	require.NoError(t, err)

	var values struct {
		Configuration struct {
			BackupStorageLocation []struct {
				Name   string `yaml:"name"`
				Bucket string `yaml:"bucket"`
				Config struct {
					Region string `yaml:"region"`
				} `yaml:"config"`
			} `yaml:"backupStorageLocation"`
		} `yaml:"configuration"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &values))
	require.Len(t, values.Configuration.BackupStorageLocation, 1)
	require.Equal(t, "default", values.Configuration.BackupStorageLocation[0].Name)
	require.Equal(t, "custom-backups", values.Configuration.BackupStorageLocation[0].Bucket)
	require.Equal(t, "DFW3", values.Configuration.BackupStorageLocation[0].Config.Region)

	require.Contains(t, rendered, "- name: default")
	require.Contains(t, rendered, "bucket: custom-backups")
	require.Contains(t, rendered, "region: DFW3")
	require.NotContains(t, strings.ToLower(rendered), "cloud-credentials")
	require.NotContains(t, rendered, "csi.vsphere.vmware.com/velero-vsphere-snapshot-class")
	require.NotContains(t, rendered, "driver: csi.vsphere.vmware.com")
}
