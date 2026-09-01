{{- $harbor := index .OpenCenter.Services "harbor" -}}
externalURL: https://{{ $harbor.Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
logLevel: info
expose:
    type: clusterIP
persistence:
    enabled: true
    resourcePolicy: keep
    persistentVolumeClaim:
        registry:
            size: 100Gi
        jobservice:
            jobLog:
                size: 100Gi
        database:
            size: 100Gi
        redis:
            size: 100Gi
        trivy:
            size: 100Gi
    imageChartStorage:
        type: s3
        s3:
            region: {{ .OpenCenter.Meta.Region }}
            bucket: {{ $harbor.S3Bucket | default (printf "%s-harbor" .OpenCenter.Cluster.ClusterName) }}
            accesskey: {{ .GetHarborS3AccessKey }}
            secretkey: {{ .GetHarborS3SecretKey }}
            regionendpoint: swift.api.{{ .OpenCenter.Meta.Region }}.rackspacecloud.com
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
