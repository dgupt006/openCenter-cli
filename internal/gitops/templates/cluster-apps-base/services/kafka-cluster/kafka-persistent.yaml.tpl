apiVersion: kafka.strimzi.io/v1
kind: KafkaNodePool
metadata:
  name: controller
  labels:
    strimzi.io/cluster: kafka-cluster
spec:
  replicas: 3
  roles:
    - controller
  storage:
    type: jbod
    volumes:
      - id: 0
        type: persistent-claim
        size: 10Gi
        # Pin the storage class so the PVC never relies on the ambiguous cluster
        # default during the bootstrap window (transient Longhorn default / Cinder
        # SC not yet created), which would bind it to the wrong backend permanently.
        class: {{ .OpenCenter.Infrastructure.Storage.DefaultStorageClass }}
        kraftMetadata: shared
        deleteClaim: false
---
apiVersion: kafka.strimzi.io/v1
kind: KafkaNodePool
metadata:
  name: broker
  labels:
    strimzi.io/cluster: kafka-cluster
spec:
  replicas: 3
  roles:
    - broker
  storage:
    type: jbod
    volumes:
      - id: 0
        type: persistent-claim
        size: 20Gi
        class: {{ .OpenCenter.Infrastructure.Storage.DefaultStorageClass }}
        kraftMetadata: shared
        deleteClaim: false
---
apiVersion: kafka.strimzi.io/v1
kind: Kafka
metadata:
  name: kafka-cluster
spec:
  kafka:
    version: 4.1.1
    metadataVersion: 4.1-IV1
    listeners:
      - name: plain
        port: 9092
        type: internal
        tls: false
      - name: tls
        port: 9093
        type: internal
        tls: true
    config:
      offsets.topic.replication.factor: 3
      transaction.state.log.replication.factor: 3
      transaction.state.log.min.isr: 2
      default.replication.factor: 3
      min.insync.replicas: 2
      log.retention.hours: 6
      log.segment.bytes: 268435456
      log.roll.ms: 3600000
      log.retention.bytes: 1073741824
  entityOperator:
    topicOperator: {}
    userOperator: {}
