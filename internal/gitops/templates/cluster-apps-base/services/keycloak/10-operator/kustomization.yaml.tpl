---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
{{- if eq .OpenCenter.Meta.Region "ord1" }}
resources:
  - ./patch-subscription.yaml
{{- else }}
resources: []
{{- end }}
