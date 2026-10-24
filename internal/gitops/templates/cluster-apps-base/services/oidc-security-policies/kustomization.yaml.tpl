{{- if and (index .OpenCenter.Services "keycloak").Enabled (index .OpenCenter.Services "kube-prometheus-stack").Enabled .OpenCenter.Identity.OIDC.Enabled }}
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - alertmanager-security-policy.yaml
  - prometheus-security-policy.yaml
  - gateway-oidc-secret.yaml
{{- end }}
