---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: [../../base/keycloak/10-operator]
patches:
  - path: patch-auto-approve.yaml
