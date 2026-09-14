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
            region: {{- if eq .OpenCenter.Infrastructure.Storage.Profile.ObjectStorageProvider "rustfs" }} us-east-1{{ else }} {{ .OpenCenter.Meta.Region }}{{ end }}
            bucket: {{- if eq .OpenCenter.Infrastructure.Storage.Profile.ObjectStorageProvider "rustfs" }} {{ printf "%s-harbor" .OpenCenter.Cluster.ClusterName }}{{ else }} {{ $harbor.S3Bucket | default (printf "%s-harbor" .OpenCenter.Cluster.ClusterName) }}{{ end }}
            accesskey: {{ .GetHarborS3AccessKey }}
            secretkey: {{ .GetHarborS3SecretKey }}
            regionendpoint: {{- if eq .OpenCenter.Infrastructure.Storage.Profile.ObjectStorageProvider "rustfs" }} http://rustfs.rustfs-system.svc.cluster.local:9000{{ else }} {{ $harbor.S3Endpoint }}{{ end }}
            v4auth: true
            secure: {{- if eq .OpenCenter.Infrastructure.Storage.Profile.ObjectStorageProvider "rustfs" }} false{{ else }} true{{ end }}
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
