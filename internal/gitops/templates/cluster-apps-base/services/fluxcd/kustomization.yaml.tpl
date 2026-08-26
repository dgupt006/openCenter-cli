---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
{{- if (index .OpenCenter.Services "sources").Enabled }}
  - ./sources.yaml
{{- end }}
{{- if or (index .OpenCenter.Services "kube-prometheus-stack").Enabled (index .OpenCenter.Services "loki").Enabled (index .OpenCenter.Services "tempo").Enabled (index .OpenCenter.Services "mimir").Enabled (index .OpenCenter.Services "opentelemetry-kube-stack").Enabled }}
  {{- if (index .OpenCenter.Services "kube-prometheus-stack").Enabled }}
  # PodMonitor lives under ./fluxcd-configs/ and needs monitoring.coreos.com
  # CRDs. Wrapped in flux-monitoring.yaml (Flux Kustomization) with dependsOn
  # kube-prometheus-stack-base so it waits for the CRDs to be installed.
  - ./flux-monitoring.yaml
  {{- end }}
  - ./observability-namespace.yaml
  - ./observability-sources.yaml
{{- end }}

{{- if (index .OpenCenter.Services "cert-manager").Enabled }}
  - ./cert-manager.yaml
{{- end }}


{{- if (index .OpenCenter.Services "harbor").Enabled }}
  - ./harbor-namespace.yaml
  - ./harbor.yaml
{{- end }}
{{- if (index .OpenCenter.Services "kafka-cluster").Enabled }}
  - ./strimzi-kafka-operator.yaml
  - ./kafka-cluster.yaml
{{- end }}
{{- if (index .OpenCenter.Services "keycloak").Enabled }}
  - ./keycloak.yaml
{{- end }}


{{- range autoServices }}
  - ./{{ . }}.yaml
{{- end }}
