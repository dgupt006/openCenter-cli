---
# Supplemental NetworkPolicy for OLM bundle-unpack Job pods (OCTR-705).
# catalog-operator auto-generates a per-CatalogSource NetworkPolicy
# (operatorhubio-catalog-unpack-bundles) that hardcodes API egress to port 6443.
# On clusters whose API port differs (e.g. 443), that policy blocks the
# bundle-unpack pods from reaching the API server, so the unpack Job fails and
# no InstallPlan is ever created. NetworkPolicies are additive, so this policy
# coexists with the auto-generated one and permits egress to the real API port.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: olm-bundle-unpack-api-egress
  namespace: olm
spec:
  podSelector:
    matchLabels:
      olm.managed: "true"
  policyTypes:
    - Egress
  egress:
    - ports:
        - protocol: TCP
          port: {{ .OpenCenter.Cluster.Kubernetes.APIPort }}
        - protocol: TCP
          port: 50051
        - protocol: TCP
          port: 53
        - protocol: UDP
          port: 53
