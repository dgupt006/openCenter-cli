---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: flux-system
resources:
{{- $services := .OpenCenter.Services }}

{{- if (index $services "cert-manager").Enabled }}
  - "opencenter-cert-manager.yaml"
{{- end }}
{{- if (index $services "harbor").Enabled }}
  - "opencenter-harbor.yaml"
{{- end }}

{{- if (index $services "kafka-cluster").Enabled }}
  - "opencenter-strimzi-kafka-operator.yaml"
{{- end }}

{{- if (index $services "keycloak").Enabled }}
  - "opencenter-keycloak.yaml"
  - "opencenter-keycloak-config.yaml"
{{- end }}
{{- if (index $services "olm").Enabled }}
  - "opencenter-olm.yaml"
  - "opencenter-olm-config.yaml"
{{- end }}
{{- if or (index $services "kube-prometheus-stack").Enabled (index $services "loki").Enabled (index $services "tempo").Enabled (index $services "mimir").Enabled (index $services "opentelemetry-kube-stack").Enabled }}
  - "opencenter-observability.yaml"
{{- end }}
{{- range autoServices }}
{{- $srcName := autoServiceSourceName . }}
{{- if eq $srcName (printf "opencenter-%s" .) }}
  - "{{ $srcName }}.yaml"
{{- end }}
{{- end }}
