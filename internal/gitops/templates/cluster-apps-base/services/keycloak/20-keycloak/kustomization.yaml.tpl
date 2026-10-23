apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - keycloak-cr-patch.yaml
  {{- if .OpenCenter.Services.keycloak.InternalTLSEnabled }}
  - keycloak-certificate.yaml
  {{- end }}
  - httproute.yaml
  - opencenter-realm.yaml
  {{- if and (.OpenCenter.Services.keycloak.MinReplicas) (.OpenCenter.Services.keycloak.MaxReplicas) }}
  - keycloak-hpa.yaml
  {{- end }}
  {{- if .OpenCenter.Services.keycloak.BackupEnabled }}
  - keycloak-backup-cronjob.yaml
  {{- end }}
