package secrets

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/stretchr/testify/require"
)

func TestWriteEncryptedManifestUsesSecretScopeAndFinalFilename(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	recorder := &recordingEncryptor{}
	manager.sopsManager = sops.NewDefaultSOPSManager(nil, recorder, nil)
	manifestPath := filepath.Join(tmpDir, "services", "api", "secret.yaml")
	ageKey := "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"

	changed, err := manager.writeEncryptedManifest(context.Background(), "api", map[string]interface{}{"token": "value"}, manifestPath, ageKey, nil)
	require.NoError(t, err)
	require.True(t, changed)
	require.FileExists(t, manifestPath)
	require.Equal(t, manifestPath, recorder.config.FilenameOverride)
	require.Equal(t, "^(data|stringData)$", recorder.config.EncryptedRegex)
	require.NotEqual(t, manifestPath, recorder.filePath)
}

type recordingEncryptor struct {
	config   sops.EncryptionConfig
	filePath string
}

func (r *recordingEncryptor) EncryptFile(_ context.Context, filePath string, config sops.EncryptionConfig) error {
	r.filePath = filePath
	r.config = config
	return nil
}
func (r *recordingEncryptor) EncryptFiles(context.Context, []string, sops.EncryptionConfig) error {
	return nil
}
func (r *recordingEncryptor) DecryptFile(context.Context, string, string) error { return nil }
func (r *recordingEncryptor) IsFileEncrypted(string) (bool, error)              { return false, nil }
func (r *recordingEncryptor) RotateKeys(context.Context, string, []string, []string) error {
	return nil
}
func (r *recordingEncryptor) GetEncryptedContent(string) (string, error)      { return "", nil }
func (r *recordingEncryptor) EditEncryptedFile(context.Context, string) error { return nil }
