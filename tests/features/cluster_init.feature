Feature: Cluster initialisation
  As a user, I want to initialise a new cluster configuration using
  the `init` command, so that I can start defining my cluster layout.

  # ---------------------------------------------------------------------------
  # Directory structure creation
  # ---------------------------------------------------------------------------

  @init @directory_structure
  Scenario: Init creates cluster-specific secrets directory structure
    When I run "opencenter cluster init secrets-test"
    Then a directory "~/.config/opencenter/clusters/secrets/opencenter/secrets-test" should exist
    And a directory "~/.config/opencenter/clusters/secrets/opencenter/secrets-test/age" should exist
    And a directory "~/.config/opencenter/clusters/secrets/opencenter/secrets-test/age/keys" should exist
    And a file "~/.config/opencenter/clusters/secrets/opencenter/secrets-test/age/keys/secrets-test-key.txt" should exist

  @init @directory_structure
  Scenario: Cluster directory creation with special characters in name
    When I run "opencenter cluster init test-cluster-123"
    Then a directory "~/.config/opencenter/clusters/gitops/opencenter/infrastructure/clusters/test-cluster-123" should exist
    And a file "~/.config/opencenter/clusters/blueprints/opencenter/test-cluster-123/test-cluster-123-config.yaml" should exist

  # ---------------------------------------------------------------------------
  # Configuration loading after init
  # ---------------------------------------------------------------------------

  @init @loading
  Scenario: Configuration loading works with new directory structure
    When I run "opencenter cluster init load-test"
    And I run "opencenter cluster use load-test"
    Then the active cluster should be "load-test"
    And the command should succeed

  # ---------------------------------------------------------------------------
  # Organization-based initialisation (--org flag)
  # ---------------------------------------------------------------------------

  @init @org
  Scenario: Init with organization creates config in correct location
    When I run "opencenter cluster init api-service --org prod-team"
    Then a file "~/.config/opencenter/clusters/blueprints/prod-team/api-service/api-service-config.yaml" should exist
    And the cluster configuration "api-service" should have "opencenter.meta.organization" set to "prod-team"
    And the cluster configuration "api-service" should have "opencenter.gitops.git_dir" containing "clusters/gitops/prod-team"

  @init @org @sops
  Scenario: Init with organization generates SOPS key in organization structure
    When I run "opencenter cluster init database --org data-team"
    Then a file "~/.config/opencenter/clusters/secrets/data-team/database/age/keys/database-key.txt" should exist
    And the file "~/.config/opencenter/clusters/secrets/data-team/database/age/keys/database-key.txt" should contain "AGE-SECRET-KEY-1"
    And a file "~/.config/opencenter/clusters/gitops/data-team/.sops.yaml" should exist
    And the file "~/.config/opencenter/clusters/gitops/data-team/.sops.yaml" should contain "creation_rules:"
    And the cluster configuration "database" should have "secrets.sops_age_key_file" containing "clusters/secrets/data-team/database/age/keys/database-key.txt"

  @init @org
  Scenario: Multiple clusters in same organization share GitOps root
    When I run "opencenter cluster init frontend --org web-team"
    And I run "opencenter cluster init backend --org web-team"
    Then a directory "~/.config/opencenter/clusters/gitops/web-team/infrastructure/clusters/frontend" should exist
    And a directory "~/.config/opencenter/clusters/gitops/web-team/infrastructure/clusters/backend" should exist
    And a file "~/.config/opencenter/clusters/blueprints/web-team/frontend/frontend-config.yaml" should exist
    And a file "~/.config/opencenter/clusters/blueprints/web-team/backend/backend-config.yaml" should exist
    And the cluster configuration "frontend" should have "opencenter.gitops.git_dir" containing "clusters/gitops/web-team"
    And the cluster configuration "backend" should have "opencenter.gitops.git_dir" containing "clusters/gitops/web-team"

  @init @org @sops
  Scenario: Init with organization creates separate SOPS keys per cluster
    When I run "opencenter cluster init service-a --org shared-team"
    And I run "opencenter cluster init service-b --org shared-team"
    Then a file "~/.config/opencenter/clusters/secrets/shared-team/service-a/age/keys/service-a-key.txt" should exist
    And a file "~/.config/opencenter/clusters/secrets/shared-team/service-b/age/keys/service-b-key.txt" should exist
    And the file "~/.config/opencenter/clusters/secrets/shared-team/service-a/age/keys/service-a-key.txt" should contain "AGE-SECRET-KEY-1"
    And the file "~/.config/opencenter/clusters/secrets/shared-team/service-b/age/keys/service-b-key.txt" should contain "AGE-SECRET-KEY-1"

  @init @org
  Scenario: Init with organization validates organization name in config
    When I run "opencenter cluster init validation-test --org validation-team --strict"
    Then the command should succeed
    And the cluster configuration "validation-test" should have "opencenter.meta.organization" set to "validation-team"

  # ---------------------------------------------------------------------------
  # Region-based initialisation (--region flag)
  # ---------------------------------------------------------------------------

  @init @region
  Scenario: Init with region creates config with region-derived values
    When I run "opencenter cluster init sjc-feature --type openstack --region sjc3"
    Then the command should succeed
    And the cluster configuration "sjc-feature" should have "opencenter.meta.region" set to "sjc3"
    And the cluster configuration "sjc-feature" should have "opencenter.cluster.cluster_fqdn" containing "sjc3"
