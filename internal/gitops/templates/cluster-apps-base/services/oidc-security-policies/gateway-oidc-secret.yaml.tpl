{{- if and (index .OpenCenter.Services "keycloak").Enabled (index .OpenCenter.Services "kube-prometheus-stack").Enabled .OpenCenter.Identity.OIDC.Enabled }}
apiVersion: v1
kind: Secret
metadata:
  name: gateway-oidc-secret
  namespace: {{ (kubePrometheusStack).Namespace }}
type: Opaque
stringData:
  client-secret: {{ .Secrets.Keycloak.ClientSecret | quote }}
{{- end }}
