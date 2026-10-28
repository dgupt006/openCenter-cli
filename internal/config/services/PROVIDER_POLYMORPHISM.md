# Service-provider resolution

`provider_registry.go` and `provider_validator.go` resolve and validate the
provider settings of enabled platform services. The registry is process-wide
and keeps compatibility and default-provider tables under a read/write lock.

## Current behavior

- `cert-manager` uses DNS providers. AWS defaults to `route53`, GCP to
  `clouddns`, Azure to `azuredns`, and Baremetal/VMware to `cloudflare`.
- OpenStack `cert-manager` uses `designate` when Designate is marked available
  for the cluster; otherwise it uses `cloudflare`.
- `AutoSelectDNSProvider` is the pure form of that cert-manager/dns selection and
  is the one production uses: `v2.Config.ResolvedDNSProvider` falls back to it
  whenever `dns_provider` is unset, so the renderers and readiness agree on the
  provider in force without mutating the registry.
- `loki`, `tempo`, and `velero` use the portable `s3` storage contract by
  default, regardless of infrastructure provider. `none` is accepted for Loki
  and Velero, and `filesystem` is accepted for Tempo.
- Legacy Swift, GCS, and Azure storage selections are rejected with migration
  guidance. Mimir separately permits `swift` only on OpenStack and otherwise
  accepts `s3`.

An explicit provider is validated instead of replaced. The validator returns
the compatible choices when a configured provider is unsupported.

## Current repository integration status

`ServiceProviderValidator` currently has no production callers in this repository. `NewServiceProviderValidator`, `ApplyDefaultProviders`, and `ValidateServiceProviders` are exercised by the package's tests, but no production package constructs or invokes the validator. The cert-manager DNS default is the exception: it is live through `AutoSelectDNSProvider`, called by `v2.Config.ResolvedDNSProvider` in `internal/config/v2/helpers.go`. The rest of the behavior above is not currently part of the live cluster enable/disable or generate pipeline described in [the platform-services reference](../../../docs/reference/platform-services.md).

## Intended integration order (not currently wired)

Call provider defaulting during configuration hydration, then validate the
resolved service settings:

```go
validator := services.NewServiceProviderValidator()

if err := validator.ApplyDefaultProviders(services, infraProvider,
    useDesignate, clusterName); err != nil {
    return err
}
if errs := validator.ValidateServiceProviders(services, infraProvider,
    useDesignate, clusterName); len(errs) != 0 {
    return errors.Join(errs...)
}
```

The compatibility matrix and default-selection behavior are covered by
`provider_registry_test.go`, `provider_validator_test.go`, and
`provider_polymorphism_property_test.go`.
