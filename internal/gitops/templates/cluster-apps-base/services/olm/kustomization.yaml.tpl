---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../base/olm
  - bundle-unpack-netpol.yaml
patches:
  - target:
      group: networking.k8s.io
      version: v1
      kind: NetworkPolicy
      name: olm-operator
      namespace: olm
    patch: |-
      - op: replace
        path: /spec/egress/0
        value:
          ports:
            - protocol: TCP
              port: {{ .OpenCenter.Cluster.Kubernetes.APIPort }}
  - target:
      group: networking.k8s.io
      version: v1
      kind: NetworkPolicy
      name: catalog-operator
      namespace: olm
    patch: |-
      - op: replace
        path: /spec/egress/0
        value:
          ports:
            - protocol: TCP
              port: {{ .OpenCenter.Cluster.Kubernetes.APIPort }}
  - target:
      group: networking.k8s.io
      version: v1
      kind: NetworkPolicy
      name: packageserver
      namespace: olm
    patch: |-
      - op: replace
        path: /spec/egress/0
        value:
          ports:
            - protocol: TCP
              port: {{ .OpenCenter.Cluster.Kubernetes.APIPort }}
