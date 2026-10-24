---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - "../base/olm/"
  - "./netpol.yaml"
# Each document in patches.yaml matches its upstream NetworkPolicy by
# group/version/kind/name/namespace, which is how kustomize v5 resolves
# inline patches.
patches:
  - path: "./patches.yaml"
