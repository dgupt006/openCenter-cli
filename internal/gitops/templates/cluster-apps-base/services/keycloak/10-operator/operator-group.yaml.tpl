apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: keycloak-operator-group
  namespace: operators
spec:
  targetNamespaces:
    - operators
