// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2schema"
	"github.com/spf13/cobra"
)

const defaultV2SchemaPath = "schema/opencenter-v2.schema.json"

var configIDESchemaPatterns = []string{
	"**/.opencenter-v2.yaml",
	"**/.opencenter-v2.yml",
	"**/*opencenter*v2*.yaml",
	"**/*opencenter*v2*.yml",
	"**/*-config.yaml",
	"**/*-config.yml",
}

var configIDEFileAssociations = []string{
	".opencenter-v2.yaml",
	".opencenter-v2.yml",
	"*opencenter*v2*.yaml",
	"*opencenter*v2*.yml",
	"*-config.yaml",
	"*-config.yml",
}

func newConfigIDECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ide",
		Short: "Generate v2 schema and editor setup for cluster configuration files",
		Long: `Generate the current v2 JSON Schema and print setup instructions for IDEs
and YAML Language Server clients.

The generated schema is an editor aid for autocomplete, hover, and early shape
validation. Runtime validation remains owned by "opencenter cluster validate".`,
		Example: `  # Generate schema and print generic YAML Language Server instructions
  opencenter settings ide

  # Generate schema and merge VS Code workspace settings
  opencenter settings ide --ide vscode --write

  # Print schema to stdout for external tooling
  opencenter settings ide --print

  # CI check for checked-in schema drift
  opencenter settings ide --check`,
		Args: cobra.NoArgs,
		RunE: runConfigIDE,
	}

	cmd.Flags().String("ide", "auto", "target IDE (auto, vscode, jetbrains, yaml-language-server, none)")
	cmd.Flags().Bool("write", false, "write supported editor configuration files")
	cmd.Flags().Bool("schema-only", false, "write schema and skip editor instructions")
	cmd.Flags().String("schema-path", defaultV2SchemaPath, "path to write or check the generated v2 JSON Schema")
	cmd.Flags().Bool("check", false, "fail if the schema file is missing or stale")
	cmd.Flags().Bool("print", false, "print schema to stdout and write no files")

	return cmd
}

func runConfigIDE(cmd *cobra.Command, args []string) error {
	ide, _ := cmd.Flags().GetString("ide")
	writeEditorConfig, _ := cmd.Flags().GetBool("write")
	schemaOnly, _ := cmd.Flags().GetBool("schema-only")
	schemaPath, _ := cmd.Flags().GetString("schema-path")
	check, _ := cmd.Flags().GetBool("check")
	printSchema, _ := cmd.Flags().GetBool("print")

	ide = strings.ToLower(strings.TrimSpace(ide))
	if err := validateConfigIDEName(ide); err != nil {
		return err
	}
	if strings.TrimSpace(schemaPath) == "" {
		return fmt.Errorf("--schema-path cannot be empty")
	}

	if printSchema {
		data, err := v2schema.Generate(v2schema.Options{})
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}

	if check {
		if err := v2schema.CheckFile(schemaPath, v2schema.Options{}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Schema is current: %s\n", filepath.ToSlash(schemaPath))
		return nil
	}

	data, err := v2schema.Generate(v2schema.Options{})
	if err != nil {
		return err
	}
	if err := writeSchemaFile(schemaPath, data); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Schema written: %s\n", filepath.ToSlash(schemaPath))

	if schemaOnly {
		return nil
	}

	targetIDE := resolveConfigIDE(ide)
	if writeEditorConfig {
		if targetIDE != "vscode" {
			return fmt.Errorf("automatic editor config writes are only supported for vscode")
		}
		if err := writeVSCodeSettings(schemaPath); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "VS Code settings updated: .vscode/settings.json")
	}

	printConfigIDEInstructions(cmd, targetIDE, schemaPath)
	return nil
}

func validateConfigIDEName(ide string) error {
	switch ide {
	case "auto", "vscode", "jetbrains", "yaml-language-server", "none":
		return nil
	default:
		return fmt.Errorf("unsupported IDE %q; use auto, vscode, jetbrains, yaml-language-server, or none", ide)
	}
}

func resolveConfigIDE(ide string) string {
	if ide != "auto" {
		return ide
	}
	if info, err := os.Stat(".vscode"); err == nil && info.IsDir() {
		return "vscode"
	}
	return "yaml-language-server"
}

func writeSchemaFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create schema directory: %w", err)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write schema file: %w", err)
	}
	return nil
}

func writeVSCodeSettings(schemaPath string) error {
	settingsPath := filepath.Join(".vscode", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create .vscode directory: %w", err)
	}

	settings := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse existing VS Code settings: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read VS Code settings: %w", err)
	}

	schemas, err := objectSetting(settings, "yaml.schemas")
	if err != nil {
		return err
	}
	schemas[vscodeSchemaPath(schemaPath)] = configIDESchemaPatterns
	settings["yaml.schemas"] = schemas

	associations, err := objectSetting(settings, "files.associations")
	if err != nil {
		return err
	}
	for _, pattern := range configIDEFileAssociations {
		associations[pattern] = "yaml"
	}
	settings["files.associations"] = associations

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal VS Code settings: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("write VS Code settings: %w", err)
	}
	return nil
}

func objectSetting(settings map[string]any, key string) (map[string]any, error) {
	raw, ok := settings[key]
	if !ok {
		return map[string]any{}, nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s in VS Code settings must be an object", key)
	}
	return object, nil
}

func vscodeSchemaPath(schemaPath string) string {
	path := filepath.ToSlash(schemaPath)
	if filepath.IsAbs(schemaPath) || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return path
	}
	return "./" + path
}

func printConfigIDEInstructions(cmd *cobra.Command, ide, schemaPath string) {
	switch ide {
	case "none":
		return
	case "vscode":
		fmt.Fprintf(cmd.OutOrStdout(), `
VS Code
Add this to .vscode/settings.json, or run with --ide vscode --write:

  "yaml.schemas": {
    "%s": [
      "**/.opencenter-v2.yaml",
      "**/.opencenter-v2.yml",
      "**/*opencenter*v2*.yaml",
      "**/*opencenter*v2*.yml",
      "**/*-config.yaml",
      "**/*-config.yml"
    ]
  }
`, vscodeSchemaPath(schemaPath))
	case "jetbrains":
		fmt.Fprintf(cmd.OutOrStdout(), `
JetBrains
Open Settings > Languages & Frameworks > Schemas and DTDs > JSON Schema Mappings.
Add %s and map it to openCenter v2 YAML configuration files.
`, filepath.ToSlash(schemaPath))
	default:
		fmt.Fprintf(cmd.OutOrStdout(), `
yaml-language-server
Add this schema association to your YAML Language Server configuration:

  yaml.schemas:
    %s:
      - "**/.opencenter-v2.yaml"
      - "**/.opencenter-v2.yml"
      - "**/*opencenter*v2*.yaml"
      - "**/*opencenter*v2*.yml"
      - "**/*-config.yaml"
      - "**/*-config.yml"
`, vscodeSchemaPath(schemaPath))
	}
}
