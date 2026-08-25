package secrets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRewriteSOPSAgeValuesUpdatesRulesIndependently(t *testing.T) {
	input := []byte("# keep this comment\ncreation_rules:\n  - path_regex: one\\.yaml$\n    encrypted_regex: ^data$\n    age: age-one\n  - path_regex: two\\.yaml$\n    age: age-two\nother: keep\n")
	var calls [][]string
	output, err := rewriteSOPSAgeValues(input, func(existing []string) ([]string, error) {
		calls = append(calls, append([]string(nil), existing...))
		return append(existing, "age-new"), nil
	})
	require.NoError(t, err)
	require.Len(t, calls, 2)
	assertSOPSAgeValues(t, output, []string{"age-one,age-new", "age-two,age-new"})
	require.Contains(t, string(output), "other: keep")
}

func TestRewriteSOPSAgeValuesDeduplicatesAndProtectsEmptyRules(t *testing.T) {
	input := []byte("creation_rules:\n  - path_regex: one\\.yaml$\n    age: age-a,age-a,age-b\n  - path_regex: two\\.yaml$\n    age: age-c\n")
	output, err := rewriteSOPSAgeValues(input, func(existing []string) ([]string, error) {
		result := make([]string, 0, len(existing))
		for _, recipient := range existing {
			result = appendUniqueRecipient(result, recipient)
		}
		return result, nil
	})
	require.NoError(t, err)
	assertSOPSAgeValues(t, output, []string{"age-a,age-b", "age-c"})

	_, err = rewriteSOPSAgeValues(input, func(existing []string) ([]string, error) {
		if strings.Contains(existing[0], "age-a") {
			return nil, nil
		}
		return existing, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "recipient set is empty")
}

func assertSOPSAgeValues(t *testing.T, data []byte, want []string) {
	t.Helper()
	var document yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &document))
	var got []string
	for _, rule := range findSOPSCreationRules(document.Content[0]) {
		for i := 0; i+1 < len(rule.Content); i += 2 {
			if rule.Content[i].Value == "age" {
				got = append(got, rule.Content[i+1].Value)
			}
		}
	}
	require.Equal(t, want, got)
}
