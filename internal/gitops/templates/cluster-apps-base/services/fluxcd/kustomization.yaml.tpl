---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
{{- if (index .OpenCenter.Services "sources").Enabled }}
  - ./sources.yaml
{{- end }}
{{- if (index .OpenCenter.Services "kube-prometheus-stack").Enabled }}
  - ./fluxcd-configs/podmonitor.yaml
  - ./observability-namespace.yaml
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
