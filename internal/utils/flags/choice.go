package flags

import (
	"fmt"
	"strings"
)

const (
	choicePlaceholderPrefix  = "<"
	choicePlaceholderSuffix  = ">"
	choiceSeparatorLiteral   = "|"
	choiceUsageEmptyTemplate = "`%s`"
	choiceUsageFullTemplate  = "`%s` %s"
)

// FormatChoiceUsage builds a usage string where the default option is capitalized inside a placeholder.
func FormatChoiceUsage(defaultChoice string, choices []string, description string) string {
	placeholder := buildChoicePlaceholder(defaultChoice, choices)
	if len(strings.TrimSpace(description)) == 0 {
		return fmt.Sprintf(choiceUsageEmptyTemplate, placeholder)
	}
	return fmt.Sprintf(choiceUsageFullTemplate, placeholder, description)
}

func buildChoicePlaceholder(defaultChoice string, choices []string) string {
	highlightedChoices := highlightDefaultChoice(defaultChoice, choices)
	return choicePlaceholderPrefix + strings.Join(highlightedChoices, choiceSeparatorLiteral) + choicePlaceholderSuffix
}

func highlightDefaultChoice(defaultChoice string, choices []string) []string {
	normalizedDefault := strings.ToLower(strings.TrimSpace(defaultChoice))
	highlighted := make([]string, 0, len(choices))
	seen := make(map[string]struct{}, len(choices))

	for _, choice := range choices {
		trimmedChoice := strings.TrimSpace(choice)
		if len(trimmedChoice) == 0 {
			continue
		}

		normalizedChoice := strings.ToLower(trimmedChoice)
		if _, exists := seen[normalizedChoice]; exists {
			continue
		}

		displayValue := trimmedChoice
		if normalizedChoice == normalizedDefault && len(normalizedChoice) > 0 {
			displayValue = strings.ToUpper(trimmedChoice)
		}

		highlighted = append(highlighted, displayValue)
		seen[normalizedChoice] = struct{}{}
	}

	return highlighted
}
