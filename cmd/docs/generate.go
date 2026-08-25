//go:build tools

// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const defaultDocsAudience = "operators, developers"

func main() {
	if err := GenerateDocs("docs/reference/opencenter", cmd.NewBuiltinRootCmd()); err != nil {
		panic(fmt.Errorf("generate documentation: %w", err))
	}
}

// GenerateDocs writes the visible built-in Cobra command tree to outputDir.
// The old directory is retained until generation succeeds, then replaced as a
// whole so stale generated pages cannot survive and failed generation is safe.
func GenerateDocs(outputDir string, root *cobra.Command) error {
	if root == nil {
		return fmt.Errorf("root command is nil")
	}

	parent := filepath.Dir(outputDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create documentation parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".opencenter-reference-")
	if err != nil {
		return fmt.Errorf("create documentation staging directory: %w", err)
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
		}
	}()

	preserved, err := existingFrontmatter(outputDir)
	if err != nil {
		return err
	}
	metadata := commandMetadata(root)
	prepender := func(filename string) string {
		base := filepath.Base(filename)
		if frontmatter, ok := preserved[base]; ok {
			return frontmatter
		}
		return metadata[base]
	}
	identity := func(link string) string { return link }
	if err := doc.GenMarkdownTreeCustom(root, stage, prepender, identity); err != nil {
		return fmt.Errorf("generate markdown tree: %w", err)
	}
	if err := normalizeGeneratedPages(stage); err != nil {
		return err
	}

	if err := replaceGeneratedDirectory(stage, outputDir); err != nil {
		return err
	}
	keepStage = true
	return nil
}

func commandMetadata(root *cobra.Command) map[string]string {
	metadata := make(map[string]string)
	walkVisibleCommands(root, func(command *cobra.Command) {
		filename := commandFilename(command)
		id := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
		id = strings.ReplaceAll(id, "_", "-")
		displayName := commandDisplayName(command.CommandPath())
		metadata[filename] = fmt.Sprintf(
			"---\nid: %s\ntitle: %s\nsidebar_label: %s\ndescription: %s\ndoc_type: reference\naudience: %s\ntags: [cli, reference]\n---\n",
			yamlQuote(id), yamlQuote(displayName), yamlQuote(displayName), yamlQuote(command.Short), yamlQuote(defaultDocsAudience),
		)
	})
	return metadata
}

func walkVisibleCommands(root *cobra.Command, visit func(*cobra.Command)) {
	if root.IsAvailableCommand() || root.Parent() == nil {
		visit(root)
	}
	for _, child := range root.Commands() {
		if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		walkVisibleCommands(child, visit)
	}
}

func commandFilename(command *cobra.Command) string {
	return strings.ReplaceAll(command.CommandPath(), " ", "_") + ".md"
}

func commandDisplayName(commandPath string) string {
	parts := strings.Fields(commandPath)
	for i, part := range parts {
		words := strings.Fields(strings.ReplaceAll(part, "-", " "))
		for j, word := range words {
			if len(word) == 0 {
				continue
			}
			words[j] = strings.ToUpper(word[:1]) + word[1:]
		}
		parts[i] = strings.Join(words, " ")
	}
	return strings.Join(parts, "_")
}

func yamlQuote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return "\"" + value + "\""
}

func existingFrontmatter(outputDir string) (map[string]string, error) {
	result := make(map[string]string)
	entries, err := os.ReadDir(outputDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read existing documentation directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(outputDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read existing documentation page %s: %w", entry.Name(), err)
		}
		if frontmatter, ok := extractFrontmatter(data); ok {
			result[entry.Name()] = frontmatter
		}
	}
	return result, nil
}

func extractFrontmatter(data []byte) (string, bool) {
	if !strings.HasPrefix(string(data), "---") {
		return "", false
	}
	separator := strings.Index(string(data[3:]), "\n---")
	if separator < 0 {
		return "", false
	}
	end := 3 + separator + len("\n---")
	if end < len(data) && data[end] == '\r' {
		end++
	}
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return string(data[:end]), true
}

func replaceGeneratedDirectory(stage, outputDir string) error {
	backup, err := os.MkdirTemp(filepath.Dir(outputDir), ".opencenter-reference-backup-")
	if err != nil {
		return fmt.Errorf("create documentation backup: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("prepare documentation backup: %w", err)
	}
	oldMoved := false
	if _, err := os.Stat(outputDir); err == nil {
		if err := os.Rename(outputDir, backup); err != nil {
			return fmt.Errorf("move existing documentation directory: %w", err)
		}
		oldMoved = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing documentation directory: %w", err)
	}

	if err := os.Rename(stage, outputDir); err != nil {
		if oldMoved {
			_ = os.Rename(backup, outputDir)
		}
		return fmt.Errorf("install generated documentation directory: %w", err)
	}
	if oldMoved {
		_ = os.RemoveAll(backup)
	}
	return nil
}

func normalizeGeneratedPages(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read staged documentation: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read staged documentation page %s: %w", entry.Name(), err)
		}
		trimmed := bytes.TrimRight(data, "\r\n")
		trimmed = append(trimmed, '\n')
		if err := os.WriteFile(path, trimmed, 0o644); err != nil {
			return fmt.Errorf("normalize staged documentation page %s: %w", entry.Name(), err)
		}
	}
	return nil
}
