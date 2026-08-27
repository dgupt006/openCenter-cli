// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitops

import (
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// templateRenderer creates a renderer that executes a Go template against the config.
func templateRenderer(tmpl string) OverrideValuesRenderer {
	return func(cfg v2.Config) (string, error) {
		funcMap := sprig.TxtFuncMap()
		t, err := template.New("override-values").Funcs(funcMap).Parse(tmpl)
		if err != nil {
			return "", err
		}
		var buf strings.Builder
		if err := t.Execute(&buf, cfg); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
}

// staticRenderer returns a renderer that always produces the same content.
func staticRenderer(content string) OverrideValuesRenderer {
	return func(_ v2.Config) (string, error) {
		return content, nil
	}
}

// --- Templates (moved from .tpl files) ---

const openstackCCMTemplate = `cloudConfig:
  global:
    auth-url: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}
    application-credential-id: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID }}
    application-credential-secret: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret }}
    domain-name: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Domain | default "default" }}
    region: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region }}
    tenant-name: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.TenantName }}
    tls-insecure: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Insecure | default false }}
  loadBalancer:
    floating-network-id: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingNetworkID }}
    subnet-id: {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.SubnetID }}
`

const openstackCSITemplate = `secret:
  enabled: true
  hostMount: false
  create: true
  filename: cloud.conf
  name: cinder-csi-cloud-config
  data:
    cloud.conf: |-
      [Global]
      auth-url = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}
      application-credential-id = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID }}
      application-credential-secret = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret }}
      domain-name = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Domain }}
      region = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region }}
      tenant-name = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.TenantName }}
      tls-insecure = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Insecure | default false }}
`

const vsphereCsiTemplate = `global:
  config:
    existingSecret: "vsphere-csi"
    global:
      cluster-id: "{{ .OpenCenter.Meta.Name }}"
    csidriver:
      enabled: true
    storageclass:
      enabled: true
      name: "{{ .OpenCenter.Infrastructure.Storage.DefaultStorageClass }}"
      storagepolicyname: ""
      expansion: true
      default: true
      reclaimPolicy: Delete
      volumebindingmode: "Immediate"
      datastoreurl: {{ .Secrets.VSphereCsi.Datastoreurl }}
vsphere-cpi:
  enabled: true
  global:
    config:
      existingConfig:
        enabled: true
        type: Secret
        name: "vsphere-cpi-secret"
      secretsInline: false
controller:
  config:
    block-volume-snapshot: true
  replicaCount: 3
  snapshotter:
    image:
      registry: {{ (index .OpenCenter.Services "vsphere-csi").Image.Repository | default "registry.k8s.io" }}
      repository: sig-storage/csi-snapshotter
      tag: {{ (index .OpenCenter.Services "vsphere-csi").Image.Tag | default "v8.2.0" }}
      pullPolicy: IfNotPresent
    args:
      - "--v=4"
      - "--kube-api-qps=100"
      - "--kube-api-burst=100"
      - "--timeout=300s"
      - "--csi-address=$(ADDRESS)"
      - "--leader-election"
      - "--leader-election-lease-duration=120s"
      - "--leader-election-renew-deadline=60s"
      - "--leader-election-retry-period=30s"
snapshot:
  controller:
    enabled: true
    replicaCount: 1
`

type veleroTemplateData struct {
	BackupStorageLocationName string
	Provider                  string
	Bucket                    string
	Region                    string
	PluginEnabled             bool
	PluginName                string
	PluginImage               string
	VSphereSnapshotClass      bool
}

func veleroRenderer(cfg v2.Config) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	storageType := ""
	bucket := ""
	region := ""
	if service, ok := cfg.OpenCenter.Services["velero"].(*services.VeleroConfig); ok && service != nil {
		storageType = strings.ToLower(strings.TrimSpace(service.StorageType))
		bucket = strings.TrimSpace(service.BackupBucket)
		region = strings.TrimSpace(service.Region)
	}

	if storageType == "" {
		switch provider {
		case "openstack":
			storageType = "swift"
		case "gcp":
			storageType = "gcs"
		case "azure":
			storageType = "azure"
		default:
			storageType = "s3"
		}
	}

	data := veleroTemplateData{
		BackupStorageLocationName: "default",
		Bucket:                    bucket,
		Region:                    region,
		VSphereSnapshotClass:      provider == "vmware" || provider == "vsphere",
	}

	switch storageType {
	case "swift":
		data.Provider = "community.openstack.org/openstack"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-openstack"
		data.PluginImage = "lirt/velero-plugin-for-openstack:v0.6.0"
	case "gcs":
		data.Provider = "velero.io/gcp"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-gcp"
		data.PluginImage = "velero/velero-plugin-for-gcp:v1.8.2"
	case "azure":
		data.Provider = "velero.io/azure"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-azure"
		data.PluginImage = "velero/velero-plugin-for-microsoft-azure:v1.10.1"
	default:
		data.Provider = "velero.io/aws"
		data.PluginEnabled = true
		data.PluginName = "velero-plugin-aws"
		data.PluginImage = "velero/velero-plugin-for-aws:v1.10.0"
	}

	if data.Region == "" {
		if provider == "openstack" && cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
			data.Region = strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
		}
		if data.Region == "" {
			data.Region = strings.TrimSpace(cfg.OpenCenter.Meta.Region)
		}
	}
	if data.Bucket == "" {
		data.Bucket = cfg.OpenCenter.Meta.Name + "-velero"
	}

	t, err := template.New("velero-values").Funcs(sprig.TxtFuncMap()).Parse(veleroTemplate)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const veleroTemplate = `---
configuration:
  features: EnableCSI
  defaultSnapshotMoveData: false
  defaultVolumesToFsBackup: false
  backupStorageLocation:
    - name: {{ .BackupStorageLocationName }}
      provider: {{ .Provider }}
      default: true
      bucket: {{ .Bucket }}
      config:
        region: {{ .Region }}
  volumeSnapshotLocation: []
{{- if .PluginEnabled }}
initContainers:
  - name: {{ .PluginName }}
    image: {{ .PluginImage }}
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins
{{- end }}
snapshotsEnabled: true
backupsEnabled: true
deployNodeAgent: false
{{- if .VSphereSnapshotClass }}
extraObjects:
  - apiVersion: snapshot.storage.k8s.io/v1
    kind: VolumeSnapshotClass
    metadata:
      name: velero-vsphere-snapshot-class
      labels:
        velero.io/csi-volumesnapshot-class: "true"
    driver: csi.vsphere.vmware.com
    deletionPolicy: Delete
{{- end }}
`

const lokiTemplate = `{{- $loki := index .OpenCenter.Services "loki" -}}
{{- $storageType := $loki.StorageType | default "s3" -}}
{{- $bucketName := $loki.BucketName | default (printf "%s-loki" .OpenCenter.Meta.Name) -}}
global:
    dnsService: coredns
loki:
    storage:
        bucketNames:
            chunks: {{ $bucketName }}
            ruler: {{ $bucketName }}
            admin: {{ $bucketName }}
        type: {{ $storageType }}
{{- if eq $storageType "swift" }}
        swift:
            auth_version: {{ $loki.SwiftAuthVersion | default 3 }}
            auth_url: {{ $loki.SwiftAuthURL }}
            region_name: {{ $loki.SwiftRegion | default .OpenCenter.Meta.Region }}
            application_credential_id: {{ $loki.SwiftApplicationCredentialID }}
            application_credential_secret: {{ .GetLokiSwiftApplicationCredentialSecret }}
            user_domain_name: {{ $loki.SwiftUserDomainName }}
            domain_name: {{ $loki.SwiftDomainName }}
            container_name: {{ $loki.SwiftContainerName | default $bucketName }}
{{- else }}
        s3:
            s3: null
            endpoint: {{ $loki.S3Endpoint | default (printf "https://swift.api.%s.rackspacecloud.com" .OpenCenter.Meta.Region) }}
            region: {{ $loki.S3Region | default .OpenCenter.Meta.Region }}
            secretAccessKey: {{ .GetLokiS3SecretKey }}
            accessKeyId: {{ .GetLokiS3AccessKey }}
            signatureVersion: null
            s3ForcePathStyle: {{ $loki.S3ForcePathStyle }}
            insecure: {{ $loki.S3Insecure }}
            http_config: {}
            backoff_config: {}
            disable_dualstack: false
{{- end }}
    schemaConfig:
        configs:
            - from: "2024-04-01"
              store: tsdb
              object_store: {{ $storageType }}
              schema: v13
              index:
                  prefix: loki_index_
                  period: 24h
`

const tempoTemplate = `{{- $tempo := index .OpenCenter.Services "tempo" -}}
{{- $storageType := $tempo.StorageType | default "s3" -}}
{{- $bucketName := $tempo.BucketName | default (printf "%s-tempo" .OpenCenter.Meta.Name) -}}
storage:
    trace:
        backend: {{ $storageType }}
{{- if eq $storageType "swift" }}
        swift:
            auth_version: {{ $tempo.SwiftAuthVersion | default 3 }}
            auth_url: {{ $tempo.SwiftAuthURL }}
            region: {{ $tempo.SwiftRegion | default .OpenCenter.Meta.Region }}
            application_credential_id: {{ $tempo.SwiftApplicationCredentialID }}
            application_credential_secret: {{ .GetTempoSwiftApplicationCredentialSecret }}
            user_domain_name: {{ $tempo.SwiftUserDomainName }}
            domain_name: {{ $tempo.SwiftDomainName }}
            container_name: {{ $tempo.SwiftContainerName | default $bucketName }}
{{- else }}
        s3:
            bucket: {{ $bucketName }}
            endpoint: {{ $tempo.S3Endpoint | default (printf "swift.api.%s.rackspacecloud.com" .OpenCenter.Meta.Region) }}
            access_key: {{ .GetTempoS3AccessKey }}
            secret_key: {{ .GetTempoS3SecretKey }}
            region: {{ $tempo.S3Region | default .OpenCenter.Meta.Region }}
            forcepathstyle: {{ $tempo.S3ForcePathStyle }}
            insecure: {{ $tempo.S3Insecure }}
{{- end }}
`

const mimirTemplate = `mimir:
    structuredConfig:
        blocks_storage:
            backend: s3
            s3:
                bucket_name: {{ .OpenCenter.Cluster.ClusterName }}-mimir
                endpoint: swift.api.{{ .OpenCenter.Meta.Region }}.rackspacecloud.com
                access_key_id: {{ .Secrets.Global.AWS.Application.AccessKey | default "PLACEHOLDER-MIMIR-ACCESS-KEY" }}
                secret_access_key: {{ .Secrets.Global.AWS.Application.SecretAccessKey | default "PLACEHOLDER-MIMIR-SECRET-KEY" }}
        ingest_storage:
            kafka:
                address: kafka-cluster-kafka-brokers.kafka-system.svc.cluster.local:9092
                topic: mimir-ingest
                auto_create_topic_enabled: true
                auto_create_topic_default_partitions: 1000
`

const otelTemplate = `collectors:
  daemon:
    config:
      exporters:
        otlphttp/loki:
          endpoint: http://observability-loki-gateway.observability.svc.cluster.local/otlp
          headers:
            X-Scope-OrgID: "default"
          compression: gzip
          timeout: 30s
          retry_on_failure:
            enabled: true
            initial_interval: 1s
            max_interval: 10s
            max_elapsed_time: 0s
          sending_queue:
            enabled: true
            num_consumers: 10
            queue_size: 2000
        otlp/tempo:
          endpoint: observability-tempo-distributor.observability.svc.cluster.local:4317
          headers:
            X-Scope-OrgID: "default"
          tls:
            insecure: true
          compression: gzip
          timeout: 30s
          retry_on_failure:
            enabled: true
            initial_interval: 1s
            max_interval: 10s
            max_elapsed_time: 0s
          sending_queue:
            enabled: true
            num_consumers: 10
            queue_size: 2000
`

const headlampTemplate = `config:
    oidc:
        enabled: true
        externalSecret:
            enabled: false
        secret:
            create: true
        clientID: opencenter
        clientSecret: {{ .Secrets.Headlamp.OIDCClientSecret }}
        issuerURL: https://{{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}/realms/opencenter
        scopes: openid profile email groups
        callbackURL: https://{{ (index .OpenCenter.Services "headlamp").Hostname | default (printf "headlamp.%s" .OpenCenter.Cluster.ClusterFQDN) }}/oidc-callback
    pluginsDir: /build/plugins
initContainers:
    - command:
        - /bin/sh
        - -c
        - mkdir -p /build/plugins && cp -r /plugins/* /build/plugins/ && chown -R 100:101 /build
      image: ghcr.io/headlamp-k8s/headlamp-plugin-flux:latest
      imagePullPolicy: Always
      name: headlamp-plugins
      securityContext:
        runAsNonRoot: false
        privileged: false
        runAsUser: 0
        runAsGroup: 0
      volumeMounts:
        - mountPath: /build/plugins
          name: headlamp-plugins
volumeMounts:
    - mountPath: /build/plugins
      name: headlamp-plugins
volumes:
    - name: headlamp-plugins
      emptyDir: {}
`

const harborTemplate = `{{- $harbor := index .OpenCenter.Services "harbor" -}}
externalURL: https://{{ $harbor.Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
logLevel: info
expose:
    type: clusterIP
persistence:
    enabled: true
    resourcePolicy: keep
    persistentVolumeClaim:
        registry:
            size: 100Gi
        jobservice:
            jobLog:
                size: 100Gi
        database:
            size: 100Gi
        redis:
            size: 100Gi
        trivy:
            size: 100Gi
    imageChartStorage:
        type: s3
        s3:
            region: {{ .OpenCenter.Meta.Region | upper }}
            bucket: {{ $harbor.S3Bucket | default (printf "%s-harbor" .OpenCenter.Cluster.ClusterName) }}
            accesskey: {{ .Secrets.Global.AWS.Application.AccessKey | default "PLACEHOLDER-HARBOR-ACCESS-KEY" }}
            secretkey: {{ .Secrets.Global.AWS.Application.SecretAccessKey | default "PLACEHOLDER-HARBOR-SECRET-KEY" }}
            regionendpoint: swift.api.{{ .OpenCenter.Meta.Region }}.rackspacecloud.com
            v4auth: true
            secure: true
            rootdirectory: images
harborAdminPassword: {{ $harbor.AdminPassword | default "PLACEHOLDER-HARBOR-ADMIN-PASSWORD" }}
metrics:
    enabled: true
    serviceMonitor:
        enabled: true
cache:
    enabled: true
    expireHours: 24
portal:
    replicas: 1
core:
    replicas: 1
jobservice:
    replicas: 1
registry:
    replicas: 1
    credentials:
        username: harbor-registry
        password: PLACEHOLDER-HARBOR-REGISTRY-PASSWORD
        htpasswdString: PLACEHOLDER-HARBOR-HTPASSWD
trivy:
    replicas: 1
database:
    internal:
        password: PLACEHOLDER-HARBOR-DATABASE-PASSWORD
exporter:
    replicas: 1
`

const kubePrometheusStackTemplate = `---
alertmanager:
  alertmanagerSpec:
    externalUrl: https://{{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "alertmanager.%s" .OpenCenter.Cluster.ClusterFQDN) }}
  config:
    global:
      resolve_timeout: 5m
    inhibit_rules:
      - source_matchers: [severity = critical]
        target_matchers: [severity =~ warning|info]
        equal: [namespace, alertname]
      - source_matchers: [severity = warning]
        target_matchers: [severity = info]
        equal: [namespace, alertname]
      - source_matchers: [alertname = InfoInhibitor]
        target_matchers: [severity = info]
        equal: [namespace]
      - target_matchers: [alertname = InfoInhibitor]
    route:
      group_by: [namespace, alertname]
      group_wait: 30s
      group_interval: 60s
      repeat_interval: 12h
      routes:
        - receiver: "null"
          matchers: [alertname = "Watchdog"]
        - receiver: warning_alerts_receiver
          continue: false
          matchers: [severity =~ "warning"]
        - receiver: alert_proxy_receiver
          continue: false
          matchers: [severity =~ "critical"]
    receivers:
      - name: "null"
      - name: warning_alerts_receiver
        msteamsv2_configs:
          - send_resolved: true
            webhook_url: {{ (index .OpenCenter.Services "kube-prometheus-stack").WebhookURL }}
      - name: alert_proxy_receiver
        webhook_configs:
          - url: http://rackspace-alert-proxy.rackspace.svc.cluster.local/alert/process
            send_resolved: true
prometheus:
  prometheusSpec:
    externalUrl: https://{{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "prometheus.%s" .OpenCenter.Cluster.ClusterFQDN) }}
    externalLabels:
      cluster: {{ .OpenCenter.Meta.Name }}
      region: {{ .OpenCenter.Meta.Region }}
      customer: {{ .OpenCenter.Meta.Organization }}
grafana:
  admin:
    existingSecret: "grafana-admin-password"
    userKey: admin-user
    passwordKey: admin-password
  datasources:
    datasources.yaml:
      apiVersion: 1
      datasources:
        - name: Loki
          uid: loki-default
          type: loki
          access: proxy
          url: http://observability-loki-gateway.observability.svc.cluster.local
          isDefault: false
          jsonData:
            httpHeaderName1: "X-Scope-OrgID"
            maxLines: 1000
          secureJsonData:
            httpHeaderValue1: "default"
          editable: true
        - name: Tempo
          uid: tempo-default
          type: tempo
          access: proxy
          url: http://observability-tempo-query-frontend.observability.svc.cluster.local:3200
          isDefault: false
          jsonData:
            httpHeaderName1: x-scope-orgid
            maxLines: 1000
            pdcInjected: false
            tracesToLogsV2:
              customQuery: false
              datasourceUid: loki-default
              filterBySpanID: true
              filterByTraceID: true
            tracesToMetrics:
              datasourceUid: prometheus
          secureJsonData:
            httpHeaderValue1: "default"
          editable: true
`
