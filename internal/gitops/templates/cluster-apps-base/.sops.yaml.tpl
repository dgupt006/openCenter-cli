creation_rules:
{{- range .OpenCenter.GitOps.OverlayUnits.SOPS.Rules }}
  - path_regex: {{ .PathRegex | squote }}
    age: {{ join "," .AgeRecipients }}
    encrypted_regex: {{ (.EncryptedRegex | default "^(data|stringData|credentials|password|secret|key|token|cert|ca|crt|tls|clientSecret|accessKeyId|secretAccessKey|secret_key|access_key|secret_access_key|access_key_id|application-credential-secret)$") | quote }}
{{- end }}
