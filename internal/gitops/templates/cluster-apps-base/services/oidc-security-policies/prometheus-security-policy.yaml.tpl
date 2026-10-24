{{- if and (index .OpenCenter.Services "keycloak").Enabled (index .OpenCenter.Services "kube-prometheus-stack").Enabled .OpenCenter.Identity.OIDC.Enabled oidcClientSecretsConfigured }}
---
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: prometheus-oidc
  namespace: {{ (kubePrometheusStack).Namespace }}
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: prometheus-gateway-route
  oidc:
    provider:
      issuer: "https://{{ if (index .OpenCenter.Services "keycloak") }}{{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" fqdn) }}{{ else }}{{ printf "auth.%s" fqdn }}{{ end }}/realms/opencenter"
    clientID: "opencenter"
    clientSecret:
      name: "gateway-oidc-secret"
    redirectURL: "https://{{ if (index .OpenCenter.Services "kube-prometheus-stack") }}{{ (index .OpenCenter.Services "kube-prometheus-stack").PrometheusHostname | default (printf "prometheus.%s" fqdn) }}{{ else }}{{ printf "prometheus.%s" fqdn }}{{ end }}/oauth2/callback"
    scopes:
      - openid
      - profile
      - email
      - roles
    logoutPath: "/logout"
{{- end }}
