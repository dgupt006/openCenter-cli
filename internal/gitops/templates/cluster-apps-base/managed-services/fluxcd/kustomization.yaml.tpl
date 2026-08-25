---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
{{- $hasManagedServices := false }}
{{- range $name, $service := .OpenCenter.ManagedServices }}
  {{- if $service.Enabled }}
  {{- $hasManagedServices = true }}
  {{- end }}
{{- end }}
resources:
{{- if $hasManagedServices }}
  - ./sources.yaml
{{- with (index .OpenCenter.ManagedServices "alert-proxy") }}
{{- if .Enabled }}
  - ./alert-proxy.yaml
{{- end }}
{{- end }}
{{- else }} []
{{- end }}
