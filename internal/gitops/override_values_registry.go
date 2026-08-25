package gitops

import v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"

// OverrideValuesRenderer produces dynamic override-values.yaml content from cluster config.
type OverrideValuesRenderer func(cfg v2.Config) (string, error)

// OverlayFilesRenderer produces additional overlay files from cluster config.
// Returns a map of filename → content.
type OverlayFilesRenderer func(cfg v2.Config) (map[string]string, error)
