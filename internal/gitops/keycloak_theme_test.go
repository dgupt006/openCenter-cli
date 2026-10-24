package gitops

import (
	"path/filepath"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

func TestKeycloakThemeEnabledByDefault(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("kc-theme-default")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	require.NoError(t, RenderClusterApps(cfg))

	keycloakDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "keycloak", "20-keycloak")

	// 1. Theme ConfigMap files are rendered in theme/
	require.FileExists(t, filepath.Join(keycloakDir, "theme", "configmap-login.yaml"))
	require.FileExists(t, filepath.Join(keycloakDir, "theme", "configmap-css.yaml"))
	require.FileExists(t, filepath.Join(keycloakDir, "theme", "configmap-img.yaml"))

	// 2. kustomization.yaml includes the theme ConfigMaps
	kustomizationPath := filepath.Join(keycloakDir, "kustomization.yaml")
	kustomizationDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, kustomizationPath)))
	require.NoError(t, err)
	require.Len(t, kustomizationDocs, 1)

	resources, ok := nestedValue(kustomizationDocs[0], "resources").([]any)
	require.True(t, ok)
	require.Contains(t, resources, "theme/configmap-login.yaml")
	require.Contains(t, resources, "theme/configmap-css.yaml")
	require.Contains(t, resources, "theme/configmap-img.yaml")

	// 3. keycloak-cr-patch.yaml mounts the theme volumes
	crPath := filepath.Join(keycloakDir, "keycloak-cr-patch.yaml")
	crDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, crPath)))
	require.NoError(t, err)
	require.Len(t, crDocs, 1)

	podTemplateSpec, ok := nestedValue(crDocs[0], "spec", "unsupported", "podTemplate", "spec").(map[string]any)
	require.True(t, ok, "spec.unsupported.podTemplate.spec must exist")

	containers, ok := podTemplateSpec["containers"].([]any)
	require.True(t, ok, "containers list must exist in podTemplate")
	require.NotEmpty(t, containers)
	kcContainer, ok := containers[0].(map[string]any)
	require.True(t, ok)

	volumeMounts, ok := kcContainer["volumeMounts"].([]any)
	require.True(t, ok, "volumeMounts must exist")
	mountMap := make(map[string]string)
	for _, rawMount := range volumeMounts {
		mount, ok := rawMount.(map[string]any)
		require.True(t, ok)
		mountMap[mount["name"].(string)] = mount["mountPath"].(string)
	}
	require.Equal(t, "/opt/keycloak/themes/opencenter/login", mountMap["theme-opencenter-login"])
	require.Equal(t, "/opt/keycloak/themes/opencenter/login/resources/css", mountMap["theme-opencenter-css"])
	require.Equal(t, "/opt/keycloak/themes/opencenter/login/resources/img", mountMap["theme-opencenter-img"])

	volumes, ok := podTemplateSpec["volumes"].([]any)
	require.True(t, ok, "volumes list must exist")
	volMap := make(map[string]string)
	for _, rawVol := range volumes {
		vol, ok := rawVol.(map[string]any)
		require.True(t, ok)
		cm, ok := vol["configMap"].(map[string]any)
		require.True(t, ok)
		volMap[vol["name"].(string)] = cm["name"].(string)
	}
	require.Equal(t, "keycloak-theme-opencenter-login", volMap["theme-opencenter-login"])
	require.Equal(t, "keycloak-theme-opencenter-css", volMap["theme-opencenter-css"])
	require.Equal(t, "keycloak-theme-opencenter-img", volMap["theme-opencenter-img"])

	// 4. opencenter-realm.yaml sets loginTheme: opencenter
	realmPath := filepath.Join(keycloakDir, "opencenter-realm.yaml")
	realmDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, realmPath)))
	require.NoError(t, err)
	require.Len(t, realmDocs, 1)

	loginTheme := nestedString(realmDocs[0], "spec", "realm", "loginTheme")
	require.Equal(t, "opencenter", loginTheme)
}

func TestKeycloakThemeCanBeDisabled(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("kc-theme-disabled")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	themeDisabled := false
	kcConfig := cfg.OpenCenter.Services["keycloak"].(*configservices.KeycloakConfig)
	kcConfig.ThemeEnabled = &themeDisabled

	require.NoError(t, RenderClusterApps(cfg))

	keycloakDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "keycloak", "20-keycloak")

	// 1. kustomization.yaml does not include theme ConfigMaps
	kustomizationPath := filepath.Join(keycloakDir, "kustomization.yaml")
	kustomizationDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, kustomizationPath)))
	require.NoError(t, err)

	resources, ok := nestedValue(kustomizationDocs[0], "resources").([]any)
	require.True(t, ok)
	require.NotContains(t, resources, "theme/configmap-login.yaml")
	require.NotContains(t, resources, "theme/configmap-css.yaml")
	require.NotContains(t, resources, "theme/configmap-img.yaml")

	// 2. keycloak-cr-patch.yaml does not mount theme volumes
	crPath := filepath.Join(keycloakDir, "keycloak-cr-patch.yaml")
	crStr := mustReadFile(t, crPath)
	require.NotContains(t, crStr, "theme-opencenter-login")
	require.NotContains(t, crStr, "theme-opencenter-css")
	require.NotContains(t, crStr, "theme-opencenter-img")

	// 3. opencenter-realm.yaml does not set loginTheme
	realmPath := filepath.Join(keycloakDir, "opencenter-realm.yaml")
	realmDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, realmPath)))
	require.NoError(t, err)

	loginTheme := nestedString(realmDocs[0], "spec", "realm", "loginTheme")
	require.Empty(t, loginTheme)
}
