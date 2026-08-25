package v2schema

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Place this exact HTML comment immediately before an intentionally invalid
// configuration fence to opt that fence out of the documentation drift check.
const schemaIgnoreComment = "<!-- opencenter:schema-ignore -->"

type docsDriftAllowlist struct {
	entries map[string]struct{}
	seen    map[string]struct{}
}

func loadDocsDriftAllowlist(path string) (*docsDriftAllowlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read documentation drift allowlist: %w", err)
	}

	allowlist := &docsDriftAllowlist{
		entries: make(map[string]struct{}),
		seen:    make(map[string]struct{}),
	}
	for lineNumber, line := range strings.Split(string(data), "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("documentation drift allowlist line %d must be relative/path.md:property.path", lineNumber+1)
		}
		docPath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(parts[0])))
		if filepath.IsAbs(docPath) || docPath == ".." || strings.HasPrefix(docPath, "../") {
			return nil, fmt.Errorf("documentation drift allowlist line %d must use a repository-relative path", lineNumber+1)
		}
		allowlist.entries[docPath+":"+strings.TrimSpace(parts[1])] = struct{}{}
	}
	return allowlist, nil
}

func (a *docsDriftAllowlist) record(docPath, propertyPath string) bool {
	key := docPath + ":" + propertyPath
	if _, ok := a.entries[key]; !ok {
		return false
	}
	a.seen[key] = struct{}{}
	return true
}

// TestDocumentationYAMLSnippetsMatchSchema protects configuration examples from
// drifting away from the checked-in v2 schema. Snippets are checked structurally
// so they may omit the schema's required top-level fields.
func TestDocumentationYAMLSnippetsMatchSchema(t *testing.T) {
	root := repoRoot(t)
	schemaData, err := os.ReadFile(filepath.Join(root, "schema", "opencenter-v2.schema.json"))
	if err != nil {
		t.Fatalf("read checked-in schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatalf("parse checked-in schema: %v", err)
	}

	allowlist, err := loadDocsDriftAllowlist(filepath.Join(root, "internal", "config", "v2schema", "testdata", "docs_drift_allowlist.txt"))
	if err != nil {
		t.Fatal(err)
	}

	docsRoot := filepath.Join(root, "docs")
	err = filepath.WalkDir(docsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return checkDocumentationFile(t, path, filepath.ToSlash(relativePath), schema, allowlist)
	})
	if err != nil {
		t.Fatalf("walk documentation: %v", err)
	}

	stale := make([]string, 0)
	for entry := range allowlist.entries {
		if _, ok := allowlist.seen[entry]; !ok {
			stale = append(stale, entry)
		}
	}
	sort.Strings(stale)
	for _, entry := range stale {
		t.Errorf("stale documentation drift allowlist entry %s; delete it", entry)
	}
}

func checkDocumentationFile(t *testing.T, path, docPath string, schema map[string]any, allowlist *docsDriftAllowlist) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	for opening := 0; opening < len(lines); opening++ {
		if strings.TrimSpace(lines[opening]) != "```yaml" {
			continue
		}
		closing := opening + 1
		for closing < len(lines) && strings.TrimSpace(lines[closing]) != "```" {
			closing++
		}
		if closing == len(lines) {
			return fmt.Errorf("%s:%d: yaml fence has no closing fence", path, opening+1)
		}
		if opening > 0 && strings.TrimSpace(lines[opening-1]) == schemaIgnoreComment {
			opening = closing
			continue
		}

		block := strings.Join(lines[opening+1:closing], "\n")
		if !containsConfigurationRoot(block) {
			opening = closing
			continue
		}
		decoder := yaml.NewDecoder(strings.NewReader(block))
		for document := 1; ; document++ {
			var node yaml.Node
			err := decoder.Decode(&node)
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("%s:%d: parse yaml snippet: %w", path, opening+1, err)
			}
			checkDocumentationDocument(t, docPath, opening+1, document, &node, schema, allowlist)
		}
	}
	return nil
}

func checkDocumentationDocument(t *testing.T, docPath string, fenceLine, document int, node *yaml.Node, schema map[string]any, allowlist *docsDriftAllowlist) {
	root := unwrapYAMLNode(node)
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}

	// Kubernetes manifests, Helm values, and CI examples have none of the
	// openCenter configuration roots below. Explicitly skip manifests even if a
	// resource happens to contain a similarly named nested field.
	if hasTopLevelKey(root, "apiVersion") || hasTopLevelKey(root, "kind") {
		return
	}

	rootProperties := schemaMap(schema["properties"])
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		var propertySchema map[string]any
		var pathPrefix string
		switch key.Value {
		case "opencenter":
			propertySchema = schemaMap(rootProperties["opencenter"])
			pathPrefix = "opencenter"
		case "services":
			opencenterSchema := schemaMap(rootProperties["opencenter"])
			propertySchema = schemaMap(schemaMap(opencenterSchema["properties"])["services"])
			pathPrefix = "services"
		case "schema_version", "secrets":
			propertySchema = schemaMap(rootProperties[key.Value])
			pathPrefix = key.Value
		default:
			continue
		}
		if propertySchema == nil {
			continue
		}
		checkSchemaProperties(t, docPath, fenceLine, document, value, propertySchema, pathPrefix, allowlist)
	}
}

func checkSchemaProperties(t *testing.T, docPath string, fenceLine, document int, node *yaml.Node, schema map[string]any, propertyPath string, allowlist *docsDriftAllowlist) {
	node = unwrapYAMLNode(node)
	if node == nil || schema == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		properties := schemaMap(schema["properties"])
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			childPath := propertyPath + "." + key.Value
			childSchema := schemaMap(properties[key.Value])
			known := childSchema != nil
			if !known {
				additional, hasAdditional := schema["additionalProperties"]
				switch {
				case hasAdditional && additional == false:
					if allowlist.record(docPath, childPath) {
						t.Logf("%s:%d: document %d: known schema drift %s", docPath, fenceLine+key.Line, document, childPath)
					} else {
						t.Errorf("%s:%d: document %d: unknown property %s", docPath, fenceLine+key.Line, document, childPath)
					}
				case hasAdditional:
					childSchema = schemaMap(additional)
				}
			}
			if childSchema != nil {
				checkSchemaProperties(t, docPath, fenceLine, document, value, childSchema, childPath, allowlist)
			}
		}
	case yaml.SequenceNode:
		itemSchema := schemaMap(schema["items"])
		for _, item := range node.Content {
			checkSchemaProperties(t, docPath, fenceLine, document, item, itemSchema, propertyPath, allowlist)
		}
	}
}

func containsConfigurationRoot(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		for _, key := range []string{"services:", "opencenter:", "schema_version:", "secrets:"} {
			if strings.HasPrefix(trimmed, key) {
				return true
			}
		}
	}
	return false
}

func schemaMap(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	return mapped
}

func unwrapYAMLNode(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		node = node.Content[0]
	}
	return node
}

func hasTopLevelKey(node *yaml.Node, wanted string) bool {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == wanted {
			return true
		}
	}
	return false
}
