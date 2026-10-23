creation_rules:
{{- range .OpenCenter.GitOps.OverlayUnits.SOPS.Rules }}
  - path_regex: {{ .PathRegex | squote }}
    age: {{ join "," .AgeRecipients }}
    {{- $regex := .EncryptedRegex | default $.Secrets.SOPSConfig.EncryptedRegex }}
    {{- if $regex }}
    encrypted_regex: {{ $regex | quote }}
    {{- end }}
{{- end }}
