{{- $kps := index .OpenCenter.Services "kube-prometheus-stack" -}}
{{- $grafanaHost := printf "grafana.%s" .OpenCenter.Cluster.ClusterFQDN -}}
{{- if $kps -}}
  {{- if $kps.GrafanaHostname -}}
    {{- $grafanaHost = $kps.GrafanaHostname -}}
  {{- else if $kps.Hostname -}}
    {{- $grafanaHost = $kps.Hostname -}}
  {{- end -}}
{{- end -}}
apiVersion: k8s.keycloak.org/v2alpha1
kind: KeycloakRealmImport
metadata:
  name: opencenter-realm
  namespace: keycloak
spec:
  keycloakCRName: keycloak
  realm:
    id: opencenter
    realm: opencenter
    enabled: true
    displayName: opencenter
    {{- if .OpenCenter.Services.keycloak.IsThemeEnabled }}
    loginTheme: opencenter
    {{- end }}
    sslRequired: external
    registrationAllowed: false
    loginWithEmailAllowed: true
    duplicateEmailsAllowed: false
    resetPasswordAllowed: false
    editUsernameAllowed: false
    bruteForceProtected: true
    permanentLockout: false
    maxFailureWaitSeconds: 900
    minimumQuickLoginWaitSeconds: 60
    waitIncrementSeconds: 60
    quickLoginCheckMilliSeconds: 1000
    maxDeltaTimeSeconds: 43200
    failureFactor: 30
    passwordPolicy: length(12) and upperCase(1) and lowerCase(1) and digits(1) and specialChars(1)
    defaultSignatureAlgorithm: RS256
    accessTokenLifespan: 28800
    ssoSessionIdleTimeout: 28800
    ssoSessionMaxLifespan: 3600
    eventsEnabled: true
    eventsListeners:
      - jboss-logging
    adminEventsEnabled: false
    adminEventsDetailsEnabled: false
    attributes:
      frontendUrl: https://{{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}
    roles:
      realm:
        - name: default-roles-opencenter
          description: ${role_default-roles}
          composite: true
          composites:
            realm:
              - offline_access
              - uma_authorization
            client:
              account:
                - manage-account
                - view-profile
          clientRole: false
        - name: offline_access
          description: ${role_offline-access}
          composite: false
          clientRole: false
        - name: admin
          description: ""
          composite: true
          composites:
            realm:
              - admin
            client:
              realm-management:
                - manage-identity-providers
                - query-clients
                - manage-clients
                - manage-users
                - create-client
                - impersonation
                - query-groups
                - manage-events
                - manage-realm
                - manage-authorization
                - query-realms
              broker:
                - read-token
              account:
                - manage-consent
                - manage-account
                - delete-account
                - view-profile
                - view-consent
                - manage-account-links
                - view-groups
                - view-applications
          clientRole: false
        - name: uma_authorization
          description: ${role_uma_authorization}
          composite: false
          clientRole: false
      client:
        grafana-oauth:
          - name: grafana-admin
            description: ""
            composite: false
            clientRole: true
          - name: grafana-editor
            description: ""
            composite: false
            clientRole: true
          - name: grafana-viewer
            description: ""
            composite: false
            clientRole: true
    defaultRole:
      name: default-roles-opencenter
      description: ${role_default-roles}
      composite: true
      clientRole: false
    groups:
      - name: cluster-admins
        path: /cluster-admins
      - name: k8s-ops
        path: /k8s-ops
      - name: namespace-admins
        path: /namespace-admins
      - name: observability
        path: /observability
      - name: platform-team
        path: /platform-team
      - name: read-only
        path: /read-only
      - name: security-team
        path: /security-team
    defaultGroups:
      - /read-only
    scopeMappings:
      - clientScope: offline_access
        roles:
          - offline_access
    clientScopes:
      - name: profile
        description: 'OpenID Connect built-in scope: profile'
        protocol: openid-connect
        attributes:
          include.in.token.scope: "true"
          display.on.consent.screen: "true"
          consent.screen.text: ${profileScopeConsentText}
        protocolMappers:
          - name: username
            protocol: openid-connect
            protocolMapper: oidc-usermodel-attribute-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              userinfo.token.claim: "true"
              user.attribute: username
              id.token.claim: "true"
              access.token.claim: "true"
              claim.name: preferred_username
              jsonType.label: String
          - name: given name
            protocol: openid-connect
            protocolMapper: oidc-usermodel-attribute-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              userinfo.token.claim: "true"
              user.attribute: firstName
              id.token.claim: "true"
              access.token.claim: "true"
              claim.name: given_name
              jsonType.label: String
          - name: family name
            protocol: openid-connect
            protocolMapper: oidc-usermodel-attribute-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              userinfo.token.claim: "true"
              user.attribute: lastName
              id.token.claim: "true"
              access.token.claim: "true"
              claim.name: family_name
              jsonType.label: String
          - name: full name
            protocol: openid-connect
            protocolMapper: oidc-full-name-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              id.token.claim: "true"
              access.token.claim: "true"
              userinfo.token.claim: "true"
      - name: email
        description: 'OpenID Connect built-in scope: email'
        protocol: openid-connect
        attributes:
          include.in.token.scope: "true"
          display.on.consent.screen: "true"
          consent.screen.text: ${emailScopeConsentText}
        protocolMappers:
          - name: email
            protocol: openid-connect
            protocolMapper: oidc-usermodel-attribute-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              userinfo.token.claim: "true"
              user.attribute: email
              id.token.claim: "true"
              access.token.claim: "true"
              claim.name: email
              jsonType.label: String
          - name: email verified
            protocol: openid-connect
            protocolMapper: oidc-usermodel-property-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              userinfo.token.claim: "true"
              user.attribute: emailVerified
              id.token.claim: "true"
              access.token.claim: "true"
              claim.name: email_verified
              jsonType.label: boolean
      - name: roles
        description: OpenID Connect scope for add user roles to the access token
        protocol: openid-connect
        attributes:
          include.in.token.scope: "false"
          display.on.consent.screen: "true"
          consent.screen.text: ${rolesScopeConsentText}
        protocolMappers:
          - name: realm roles
            protocol: openid-connect
            protocolMapper: oidc-usermodel-realm-role-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              multivalued: "true"
              user.attribute: foo
              access.token.claim: "true"
              claim.name: realm_access.roles
              jsonType.label: String
          - name: client roles
            protocol: openid-connect
            protocolMapper: oidc-usermodel-client-role-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              multivalued: "true"
              user.attribute: foo
              access.token.claim: "true"
              claim.name: resource_access.${client_id}.roles
              jsonType.label: String
          - name: audience resolve
            protocol: openid-connect
            protocolMapper: oidc-audience-resolve-mapper
            consentRequired: false
      - name: web-origins
        description: OpenID Connect scope for add allowed web origins to the access token
        protocol: openid-connect
        attributes:
          include.in.token.scope: "false"
          display.on.consent.screen: "false"
        protocolMappers:
          - name: allowed web origins
            protocol: openid-connect
            protocolMapper: oidc-allowed-origins-mapper
            consentRequired: false
      - name: acr
        description: OpenID Connect scope for add acr (authentication context class reference) to the token
        protocol: openid-connect
        attributes:
          include.in.token.scope: "false"
          display.on.consent.screen: "false"
        protocolMappers:
          - name: acr loa level
            protocol: openid-connect
            protocolMapper: oidc-acr-mapper
            consentRequired: false
            config:
              id.token.claim: "true"
              introspection.token.claim: "true"
              access.token.claim: "true"
              userinfo.token.claim: "true"
      - name: basic
        description: OpenID Connect scope for add all basic claims to the token
        protocol: openid-connect
        attributes:
          include.in.token.scope: "false"
          display.on.consent.screen: "false"
        protocolMappers:
          - name: sub
            protocol: openid-connect
            protocolMapper: oidc-sub-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              access.token.claim: "true"
      - name: groups
        description: ""
        protocol: openid-connect
        attributes:
          include.in.token.scope: "false"
          display.on.consent.screen: "true"
        protocolMappers:
          - name: name
            protocol: openid-connect
            protocolMapper: oidc-usermodel-attribute-mapper
            consentRequired: false
            config:
              aggregate.attrs: "false"
              introspection.token.claim: "true"
              multivalued: "false"
              userinfo.token.claim: "true"
              user.attribute: username
              id.token.claim: "true"
              lightweight.claim: "false"
              access.token.claim: "true"
              claim.name: name
              jsonType.label: String
          - name: groups
            protocol: openid-connect
            protocolMapper: oidc-group-membership-mapper
            consentRequired: false
            config:
              full.path: "false"
              introspection.token.claim: "true"
              userinfo.token.claim: "true"
              multivalued: "true"
              id.token.claim: "true"
              lightweight.claim: "false"
              access.token.claim: "true"
              claim.name: groups
    defaultDefaultClientScopes:
      - role_list
      - saml_organization
      - profile
      - email
      - roles
      - web-origins
      - acr
      - basic
      - groups
    defaultOptionalClientScopes:
      - offline_access
      - address
      - phone
      - microprofile-jwt
      - organization
    clients:
      - clientId: opencenter
        name: openCenter Cluster
        description: OIDC client for Kubernetes API authentication
        enabled: true
        publicClient: false
        {{- if .Secrets.Keycloak.ClientSecret }}
        secret: {{ .Secrets.Keycloak.ClientSecret | quote }}
        {{- end }}
        standardFlowEnabled: true
        directAccessGrantsEnabled: false
        serviceAccountsEnabled: true
        protocol: openid-connect
        frontchannelLogout: true
        attributes:
          post.logout.redirect.uris: +
          oauth2.device.authorization.grant.enabled: "true"
          use.refresh.tokens: "true"
        redirectUris:
          - http://*
          - https://*
        webOrigins:
          - http://*
          - https://*
        defaultClientScopes:
          - service_account
          - web-origins
          - acr
          - roles
          - profile
          - groups
          - basic
          - email
        optionalClientScopes:
          - address
          - phone
          - organization
          - offline_access
          - microprofile-jwt
        protocolMappers:
          - name: groups
            protocol: openid-connect
            protocolMapper: oidc-group-membership-mapper
            consentRequired: false
            config:
              full.path: "false"
              id.token.claim: "true"
              access.token.claim: "true"
              claim.name: groups
              userinfo.token.claim: "true"
              multivalued: "true"
          - name: audience
            protocol: openid-connect
            protocolMapper: oidc-audience-mapper
            consentRequired: false
            config:
              included.client.audience: kubernetes
              id.token.claim: "true"
              access.token.claim: "true"
              userinfo.token.claim: "true"
          - name: email
            protocol: openid-connect
            protocolMapper: oidc-usermodel-property-mapper
            consentRequired: false
            config:
              usermodel.attribute: email
              claim.name: email
              id.token.claim: "true"
              access.token.claim: "true"
              userinfo.token.claim: "true"
      - clientId: grafana-oauth
        name: grafana
        description: ""
        rootUrl: https://{{ $grafanaHost }}
        adminUrl: https://{{ $grafanaHost }}
        baseUrl: https://{{ $grafanaHost }}
        enabled: true
        publicClient: false
        standardFlowEnabled: true
        implicitFlowEnabled: false
        directAccessGrantsEnabled: true
        serviceAccountsEnabled: false
        frontchannelLogout: true
        protocol: openid-connect
        attributes:
          oidc.ciba.grant.enabled: "false"
          backchannel.logout.session.required: "true"
          standard.token.exchange.enabled: "false"
          oauth2.device.authorization.grant.enabled: "false"
          backchannel.logout.revoke.offline.tokens: "false"
        redirectUris:
          - https://{{ $grafanaHost }}/login/generic_oauth/*
          - https://{{ $grafanaHost }}/*
        webOrigins:
          - https://{{ $grafanaHost }}
        defaultClientScopes:
          - web-origins
          - acr
          - roles
          - profile
          - groups
          - basic
          - email
        optionalClientScopes:
          - address
          - phone
          - organization
          - offline_access
          - microprofile-jwt
        protocolMappers:
          - name: client-roles
            protocol: openid-connect
            protocolMapper: oidc-usermodel-client-role-mapper
            consentRequired: false
            config:
              introspection.token.claim: "true"
              multivalued: "true"
              userinfo.token.claim: "true"
              id.token.claim: "true"
              lightweight.claim: "false"
              access.token.claim: "true"
              claim.name: roles
              jsonType.label: JSON
              usermodel.clientRoleMapping.clientId: grafana-oauth
    users:
      - username: service-account-opencenter
        enabled: true
        serviceAccountClientId: opencenter
        realmRoles:
          - default-roles-opencenter
        groups:
          - /read-only
      - username: oc.admin
        enabled: true
        email: mpk-support@rackspace.com
        emailVerified: true
        firstName: Admin
        lastName: User
        groups:
          - /cluster-admins
        credentials:
          - type: password
            temporary: true
            value: "ChangeMeImmediately123!"
    browserSecurityHeaders:
      contentSecurityPolicyReportOnly: ""
      xContentTypeOptions: nosniff
      referrerPolicy: no-referrer
      xRobotsTag: none
      xFrameOptions: SAMEORIGIN
      contentSecurityPolicy: frame-src 'self'; frame-ancestors 'self'; object-src 'none';
      strictTransportSecurity: max-age=31536000; includeSubDomains
