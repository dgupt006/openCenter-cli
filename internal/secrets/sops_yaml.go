package secrets

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// rewriteSOPSAgeValues changes only existing creation-rule age scalar values.
// The YAML node tree preserves all unrelated fields, comments, and scalar styles.
func rewriteSOPSAgeValues(data []byte, update func([]string) ([]string, error)) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return data, nil
	}

	rules := findSOPSCreationRules(document.Content[0])
	var ageNodes []*yaml.Node
	var existing []string
	for _, rule := range rules {
		if rule.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(rule.Content); i += 2 {
			if rule.Content[i].Value != "age" {
				continue
			}
			value := rule.Content[i+1]
			if value.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("creation rule age value is not a scalar")
			}
			ageNodes = append(ageNodes, value)
			existing = append(existing, splitSOPSRecipients(value.Value)...)
			break
		}
	}
	if len(ageNodes) == 0 {
		return data, nil
	}

	recipients, err := update(existing)
	if err != nil {
		return nil, err
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("computed SOPS age recipient set is empty")
	}
	value := strings.Join(recipients, ",")
	for _, ageNode := range ageNodes {
		ageNode.Value = value
	}

	return marshalSOPSDocument(&document)
}

// marshalSOPSDocument serializes the node tree at two-space indentation.
// yaml.Marshal defaults to four, which would reindent every sequence in the
// user's committed .sops.yaml on each rotation or revocation and produce pure
// diff noise in their GitOps repository.
func marshalSOPSDocument(document *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func findSOPSCreationRules(root *yaml.Node) []*yaml.Node {
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "creation_rules" && root.Content[i+1].Kind == yaml.SequenceNode {
			return root.Content[i+1].Content
		}
	}
	return nil
}

func splitSOPSRecipients(value string) []string {
	parts := strings.Split(value, ",")
	recipients := make([]string, 0, len(parts))
	for _, part := range parts {
		if recipient := strings.TrimSpace(part); recipient != "" {
			recipients = append(recipients, recipient)
		}
	}
	return recipients
}
