{{- $harbor := index .OpenCenter.Services "harbor" -}}
{{- $storageClass := $harbor.StorageClass | default .OpenCenter.Infrastructure.Storage.DefaultStorageClass -}}
externalURL: https://{{ $harbor.Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
logLevel: info
expose:
    type: clusterIP
persistence:
    enabled: true
    resourcePolicy: keep
    persistentVolumeClaim:
        # Harbor requires registry PVC cache/state even when image blobs use object storage.
        registry:
            size: {{ $harbor.RegistryVolumeSize | default 100 }}Gi
            storageClass: {{ $storageClass }}
        jobservice:
            jobLog:
                size: {{ $harbor.JobserviceVolumeSize | default 5 }}Gi
                storageClass: {{ $storageClass }}
        database:
            size: {{ $harbor.DatabaseVolumeSize | default 10 }}Gi
            storageClass: {{ $storageClass }}
        redis:
            size: {{ $harbor.RedisVolumeSize | default 5 }}Gi
            storageClass: {{ $storageClass }}
        trivy:
            size: {{ $harbor.TrivyVolumeSize | default 5 }}Gi
            storageClass: {{ $storageClass }}
    # Primary image blobs use object storage; registry PVC is cache/state, not blob storage.
    imageChartStorage:
        type: s3
        s3:
            region: {{ .OpenCenter.Meta.Region }}
            bucket: {{ $harbor.S3Bucket | default (printf "%s-harbor" .OpenCenter.Cluster.ClusterName) }}
            accesskey: {{ .GetHarborS3AccessKey }}
            secretkey: {{ .GetHarborS3SecretKey }}
            regionendpoint: {{ $harbor.S3Endpoint }}
            v4auth: true
            secure: true
            rootdirectory: images
harborAdminPassword: {{ .Secrets.Harbor.AdminPassword | quote }}
metrics:
    enabled: true
    serviceMonitor:
        enabled: true
cache:
    enabled: true
    expireHours: 24
portal:
    replicas: 1
core:
    replicas: 1
jobservice:
    replicas: 1
registry:
    replicas: 1
    credentials:
        username: harbor-registry
        password: {{ .Secrets.Harbor.RegistryPassword | quote }}
        htpasswdString: ""
trivy:
    replicas: 1
database:
    internal:
        password: {{ .Secrets.Harbor.DatabasePassword | quote }}
exporter:
    replicas: 1
