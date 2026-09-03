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

// etcdBackupFluxTemplate is the top-level Flux Kustomization that reconciles the
// etcd-backup overlay. Without it (and its entry in the services/fluxcd
// aggregator) Flux never sees the service. etcd-backup ships no Helm release and
// no source of its own, so a single overlay-sourced Kustomization depending on
// sources is sufficient.
const etcdBackupFluxTemplate = `---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: etcd-backup
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .ClusterName }}/services/etcd-backup
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: etcd-backup
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
`

type etcdBackupKustomizationData struct {
	HasMaterializedSecret bool
}

type etcdBackupFluxData struct {
	ClusterName string
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

	fluxContent, err := renderInlineTemplateContent(
		etcdBackupFluxTemplate,
		"etcd-backup.yaml",
		etcdBackupFluxData{ClusterName: cfg.OpenCenter.Cluster.ClusterName},
	)
	if err != nil {
		return nil, err
	}

	return []clusterAppAction{
		{
			Owner:   "service-etcd-backup",
			Output:  filepath.ToSlash(filepath.Join("services/etcd-backup", "kustomization.yaml")),
			Content: content,
		},
		{
			Owner:   "service-etcd-backup",
			Output:  filepath.ToSlash(filepath.Join("services/fluxcd", "etcd-backup.yaml")),
			Content: fluxContent,
		},
	}, nil
}
