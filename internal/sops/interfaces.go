/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sops

import (
	"context"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
)

// SOPSManager interface for managing SOPS operations
type SOPSManager interface {
	// Key management operations
	GetKeyManager() crypto.KeyManager

	// Encryption operations
	GetEncryptor() Encryptor

	// High-level operations
	EncryptOverlayFiles(ctx context.Context, overlayPath string, cfg *v2.Config) error
	CreateSOPSConfig(overlayPath string, cfg *v2.Config) error
	ValidateEncryption(overlayPath string, cfg *v2.Config) error
	CreateSampleEncryptedSecrets(ctx context.Context, repoPath string, ageKey string) error
	EncryptRepositorySecrets(ctx context.Context, repoPath string, ageKey string) error
	CheckSOPSVersion(ctx context.Context) (string, error)
}

// Encryptor interface for SOPS encryption/decryption operations
type Encryptor interface {
	EncryptFile(ctx context.Context, filePath string, config EncryptionConfig) error
	EncryptFiles(ctx context.Context, filePaths []string, config EncryptionConfig) error
	DecryptFile(ctx context.Context, filePath string, outputPath string) error
	IsFileEncrypted(filePath string) (bool, error)
	RotateKeys(ctx context.Context, filePath string, newAgeKeys, newPGPKeys []string) error
	GetEncryptedContent(filePath string) (string, error)
	EditEncryptedFile(ctx context.Context, filePath string) error
}

// EncryptionConfig represents SOPS encryption configuration
type EncryptionConfig struct {
	AgeKeys          []string
	PGPKeys          []string
	ConfigFile       string
	FilenameOverride string
	EncryptedRegex   string
	InPlace          bool
	DryRun           bool
	Verbose          bool
}
