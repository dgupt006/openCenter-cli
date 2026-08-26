---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: observability-sources
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: observability-namespace
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-observability
    namespace: flux-system
  path: applications/base/services/observability/sources
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: opencenter-observability
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
