package gitops

import (
	"fmt"
	"path/filepath"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

const rustFSNamespace = "rustfs-system"

// RustFS is an internal component of the managed object-storage profile. Its
// credentials and consuming-service configuration are deliberately rendered
// separately once those generated values exist (OCTR-742 item 7).
const rustFSKustomizationTemplate = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace.yaml
  - service.yaml
  - statefulset.yaml
`

const rustFSNamespaceTemplate = `---
apiVersion: v1
kind: Namespace
metadata:
  name: rustfs-system
  labels:
    app.kubernetes.io/part-of: rustfs
    opencenter/managed-by: opencenter
`

const rustFSServiceTemplate = `---
apiVersion: v1
kind: Service
metadata:
  name: rustfs
  namespace: rustfs-system
  labels:
    app.kubernetes.io/name: rustfs
    app.kubernetes.io/part-of: rustfs
spec:
  selector:
    app.kubernetes.io/name: rustfs
  ports:
    - name: s3
      port: 9000
      targetPort: s3
    - name: console
      port: 9001
      targetPort: console
`

const rustFSStatefulSetTemplate = `---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rustfs
  namespace: rustfs-system
  labels:
    app.kubernetes.io/name: rustfs
    app.kubernetes.io/part-of: rustfs
spec:
  serviceName: rustfs
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: rustfs
  template:
    metadata:
      labels:
        app.kubernetes.io/name: rustfs
        app.kubernetes.io/part-of: rustfs
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: rustfs
          image: rustfs/rustfs:1.0.0-rc.6
          args: ["/data"]
          ports:
            - name: s3
              containerPort: 9000
            - name: console
              containerPort: 9001
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: false
          readinessProbe:
            tcpSocket:
              port: s3
            initialDelaySeconds: 5
          livenessProbe:
            tcpSocket:
              port: s3
            initialDelaySeconds: 15
          volumeMounts:
            - name: data
              mountPath: /data
  volumeClaimTemplates:
    - metadata:
        name: data
        labels:
          app.kubernetes.io/name: rustfs
          app.kubernetes.io/part-of: rustfs
      spec:
        accessModes: ["ReadWriteOnce"]
        storageClassName: longhorn
        resources:
          requests:
            storage: 50Gi
`

const rustFSFluxTemplate = `---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: rustfs
  namespace: flux-system
spec:
  dependsOn:
    - name: longhorn-base
      namespace: flux-system
  interval: {{ .FluxInterval }}
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .ClusterName }}/services/rustfs
  targetNamespace: rustfs-system
  prune: true
  wait: true
  force: true
  suspend: false
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: rustfs
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
`

type rustFSFluxData struct {
	ClusterName  string
	FluxInterval string
}

// planRustFSActions materializes the Longhorn-backed RustFS object store only
// for the managed, non-production storage profile. It has no public service
// configuration surface: the storage profile is the sole activation boundary.
func planRustFSActions(cfg v2.Config, _ []secretartifacts.Artifact) ([]clusterAppAction, error) {
	profile := v2.EffectiveStorageProfile(&cfg)
	if profile.ObjectStorageProvider != v2.StorageObjectProviderRustFS {
		return nil, nil
	}
	if profile.Lifecycle != v2.StorageLifecycleNonProduction {
		return nil, fmt.Errorf("RustFS rendering requires the non-production storage lifecycle")
	}
	if profile.PVCProvider != v2.StoragePVCProviderLonghorn {
		return nil, fmt.Errorf("RustFS rendering requires pvc_provider: longhorn")
	}
	longhorn, exists := cfg.OpenCenter.Services["longhorn"]
	if !exists || IsServiceDisabled(longhorn) {
		return nil, fmt.Errorf("RustFS rendering requires the Longhorn service to be enabled")
	}

	flux, err := renderInlineTemplateContent(rustFSFluxTemplate, "rustfs.yaml", rustFSFluxData{
		ClusterName:  cfg.ClusterName(),
		FluxInterval: cfg.OpenCenter.GitOps.Flux.Interval,
	})
	if err != nil {
		return nil, err
	}

	return []clusterAppAction{
		{Owner: "storage-rustfs", Output: filepath.ToSlash(filepath.Join("services", "rustfs", "kustomization.yaml")), Content: rustFSKustomizationTemplate},
		{Owner: "storage-rustfs", Output: filepath.ToSlash(filepath.Join("services", "rustfs", "namespace.yaml")), Content: rustFSNamespaceTemplate},
		{Owner: "storage-rustfs", Output: filepath.ToSlash(filepath.Join("services", "rustfs", "service.yaml")), Content: rustFSServiceTemplate},
		{Owner: "storage-rustfs", Output: filepath.ToSlash(filepath.Join("services", "rustfs", "statefulset.yaml")), Content: rustFSStatefulSetTemplate},
		{Owner: "storage-rustfs", Output: filepath.ToSlash(filepath.Join("services", "fluxcd", "rustfs.yaml")), Content: flux},
	}, nil
}
