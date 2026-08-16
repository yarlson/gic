package commit

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

//go:embed commit_prompt.md
var commitPromptTemplateText string

var commitPromptTemplate = template.Must(template.New("commit_prompt").Parse(commitPromptTemplateText))

// CleanStatus strips ANSI codes and trailing whitespace from each line.
func CleanStatus(s string) string {
	var cleanedLines []string

	for _, line := range strings.Split(s, "\n") {
		cleaned := ansiRegex.ReplaceAllString(line, "")
		cleaned = strings.TrimRight(cleaned, " \t\r")

		if strings.TrimSpace(cleaned) == "" {
			continue
		}

		cleanedLines = append(cleanedLines, cleaned)
	}

	return strings.Join(cleanedLines, "\n")
}

// BuildPrompt combines the shared commit-writing guidance with a provider-
// specific inspection overlay and optional user hint.
func BuildPrompt(overlay, userInput string) (string, error) {
	data := struct {
		Overlay      string
		UserInput    string
		HasUserInput bool
	}{
		Overlay:      normalizePromptBlock(overlay),
		UserInput:    normalizePromptBlock(userInput),
		HasUserInput: strings.TrimSpace(userInput) != "",
	}

	var b strings.Builder

	if err := commitPromptTemplate.Execute(&b, data); err != nil {
		return "", fmt.Errorf("render commit prompt: %w", err)
	}

	return b.String(), nil
}

func normalizePromptBlock(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	if strings.HasSuffix(value, "\n") {
		return value
	}

	return value + "\n"
}
