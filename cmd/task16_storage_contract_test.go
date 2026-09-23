package cmd

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestValidateServiceUsesProviderAwareLokiTempoStorageDefaults(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		service  string
		wantErr  string
	}{
		{name: "OpenStack omitted Loki uses Swift", provider: "openstack", service: "loki"},
		{name: "generic omitted Loki uses S3", provider: "kind", service: "loki", wantErr: "s3_endpoint"},
		{name: "generic omitted Tempo uses S3", provider: "kind", service: "tempo", wantErr: "s3_endpoint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := v2.NewV2Default("task16", tt.provider)
			if err != nil {
				t.Fatal(err)
			}
			cfg.OpenCenter.Infrastructure.Provider = tt.provider
			var service any
			switch tt.service {
			case "loki":
				service = &services.LokiConfig{BaseConfig: services.BaseConfig{Enabled: true}, SwiftApplicationCredentialID: "swift-id"}
				cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "swift-secret"
			case "tempo":
				service = &services.TempoConfig{BaseConfig: services.BaseConfig{Enabled: true}}
				cfg.Secrets.Tempo.AccessKey = "tempo-access"
				cfg.Secrets.Tempo.SecretKey = "tempo-secret"
			}
			cfg.OpenCenter.Services[tt.service] = service

			err = validateServiceWithConfig(tt.service, service, &cfg.Secrets, cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateServiceWithConfig() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateServiceWithConfig() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
