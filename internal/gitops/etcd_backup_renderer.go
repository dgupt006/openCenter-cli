package gitops

import (
	"path/filepath"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

const etcdBackupKustomizationTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - cronjob.yaml
{{- if .HasMaterializedSecret }}
  - secret.yaml
{{- end }}

configMapGenerator:
  - name: etcd-backup-upload-script
    files:
      - backup.py
`

type etcdBackupKustomizationData struct {
	HasMaterializedSecret bool
}

// planEtcdBackupDynamicActions emits the service kustomization separately from
// the descriptor-owned CronJob and uploader files. This keeps full and
// single-service plans on the same ownership path and only references a Secret
// after its owned artifact has been materialized.
func planEtcdBackupDynamicActions(cfg v2.Config, artifacts []secretartifacts.Artifact) ([]clusterAppAction, error) {
	service, exists := cfg.OpenCenter.Services["etcd-backup"]
	if !exists || IsServiceDisabled(service) {
		return nil, nil
	}

	content, err := renderInlineTemplateContent(
		etcdBackupKustomizationTemplate,
		"kustomization.yaml",
		etcdBackupKustomizationData{HasMaterializedSecret: secretArtifactTargetMaterialized(cfg, "etcd-backup", artifacts)},
	)
	if err != nil {
		return nil, err
	}
	return []clusterAppAction{{
		Owner:   "service-etcd-backup",
		Output:  filepath.ToSlash(filepath.Join("services/etcd-backup", "kustomization.yaml")),
		Content: content,
	}}, nil
}
