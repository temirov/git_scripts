package flags

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/pflag"
)

const (
	toggleTrueCanonicalValue               = "true"
	toggleFalseCanonicalValue              = "false"
	toggleYesLiteral                       = "yes"
	toggleNoLiteral                        = "no"
	toggleOnLiteral                        = "on"
	toggleOffLiteral                       = "off"
	toggleOneLiteral                       = "1"
	toggleZeroLiteral                      = "0"
	toggleTLiteral                         = "t"
	toggleFLiteral                         = "f"
	toggleYLiteral                         = "y"
	toggleNLiteral                         = "n"
	toggleParseErrorTemplate               = "invalid toggle value %q"
	toggleArgumentTruePlaceholderConstant  = "<YES|no>"
	toggleArgumentFalsePlaceholderConstant = "<yes|NO>"
)

var (
	trueLiteralSet = map[string]struct{}{
		toggleTrueCanonicalValue: {},
		toggleYesLiteral:         {},
		toggleOnLiteral:          {},
		toggleOneLiteral:         {},
		toggleTLiteral:           {},
		toggleYLiteral:           {},
	}
	falseLiteralSet = map[string]struct{}{
		toggleFalseCanonicalValue: {},
		toggleNoLiteral:           {},
		toggleOffLiteral:          {},
		toggleZeroLiteral:         {},
		toggleFLiteral:            {},
		toggleNLiteral:            {},
	}

	toggleFlagRegistryMutex sync.RWMutex
	toggleFlagNames         = map[string]struct{}{}
	toggleFlagShorthands    = map[string]struct{}{}
)

// AddToggleFlag registers a boolean toggle flag that accepts yes/no style values.
func AddToggleFlag(flagSet *pflag.FlagSet, target *bool, name string, shorthand string, defaultValue bool, usage string) {
	if flagSet == nil {
		return
	}
	if len(name) == 0 {
		return
	}

	toggleValue := newToggleFlagValue(defaultValue, target)
	if len(shorthand) > 0 {
		flagSet.VarP(toggleValue, name, shorthand, usage)
	} else {
		flagSet.Var(toggleValue, name, usage)
	}

	flag := flagSet.Lookup(name)
	if flag == nil {
		return
	}
	flag.NoOptDefVal = toggleTrueCanonicalValue
	flag.Usage = formatToggleUsage(usage, defaultValue)

	registerToggleFlag(name, shorthand)
}

func formatToggleUsage(description string, defaultValue bool) string {
	placeholder := toggleArgumentFalsePlaceholderConstant
	if defaultValue {
		placeholder = toggleArgumentTruePlaceholderConstant
	}
	trimmed := strings.TrimSpace(description)
	if len(trimmed) == 0 {
		return fmt.Sprintf("`%s`", placeholder)
	}
	return fmt.Sprintf("`%s` %s", placeholder, trimmed)
}

// NormalizeToggleArguments rewrites toggle flag arguments so "--flag value" becomes "--flag=value" before parsing.
func NormalizeToggleArguments(arguments []string) []string {
	if len(arguments) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(arguments))
	index := 0
	for index < len(arguments) {
		current := arguments[index]
		if current == "--" {
			normalized = append(normalized, arguments[index:]...)
			break
		}

		if normalizedArgument, consumed := normalizeToggleLong(current, arguments, index); consumed > 0 {
			normalized = append(normalized, normalizedArgument)
			index += consumed
			continue
		}

		if normalizedArgument, consumed := normalizeToggleShort(current, arguments, index); consumed > 0 {
			normalized = append(normalized, normalizedArgument)
			index += consumed
			continue
		}

		normalized = append(normalized, current)
		index++
	}

	return normalized
}

type toggleFlagValue struct {
	currentValue bool
	target       *bool
}

func newToggleFlagValue(defaultValue bool, target *bool) *toggleFlagValue {
	if target != nil {
		*target = defaultValue
	}
	return &toggleFlagValue{currentValue: defaultValue, target: target}
}

func (value *toggleFlagValue) Set(rawValue string) error {
	parsedValue, parseError := parseToggleValue(rawValue)
	if parseError != nil {
		return parseError
	}

	value.currentValue = parsedValue
	if value.target != nil {
		*value.target = parsedValue
	}

	return nil
}

func (value *toggleFlagValue) String() string {
	if value == nil {
		return toggleFalseCanonicalValue
	}
	if value.currentValue {
		return toggleTrueCanonicalValue
	}
	return toggleFalseCanonicalValue
}

func (value *toggleFlagValue) Type() string {
	return "bool"
}

func parseToggleValue(rawValue string) (bool, error) {
	trimmedValue := strings.TrimSpace(rawValue)
	if len(trimmedValue) == 0 {
		trimmedValue = toggleTrueCanonicalValue
	}

	normalizedValue := strings.ToLower(trimmedValue)
	if _, isTrue := trueLiteralSet[normalizedValue]; isTrue {
		return true, nil
	}
	if _, isFalse := falseLiteralSet[normalizedValue]; isFalse {
		return false, nil
	}

	return false, fmt.Errorf(toggleParseErrorTemplate, rawValue)
}

func registerToggleFlag(name string, shorthand string) {
	toggleFlagRegistryMutex.Lock()
	defer toggleFlagRegistryMutex.Unlock()
	toggleFlagNames[name] = struct{}{}
	if len(shorthand) > 0 {
		toggleFlagShorthands[shorthand] = struct{}{}
	}
}

func normalizeToggleLong(current string, arguments []string, index int) (string, int) {
	if !strings.HasPrefix(current, "--") {
		return "", 0
	}
	trimmed := strings.TrimPrefix(current, "--")
	if len(trimmed) == 0 {
		return "", 0
	}
	splitIndex := strings.Index(trimmed, "=")
	name := trimmed
	if splitIndex >= 0 {
		name = trimmed[:splitIndex]
	}
	if len(name) == 0 {
		return "", 0
	}
	if !isToggleName(name) {
		return "", 0
	}
	if splitIndex >= 0 {
		return current, 1
	}
	if index+1 >= len(arguments) {
		return current, 1
	}
	nextValue := arguments[index+1]
	if startsWithDash(nextValue) {
		return current, 1
	}
	return current + "=" + nextValue, 2
}

func normalizeToggleShort(current string, arguments []string, index int) (string, int) {
	if !strings.HasPrefix(current, "-") || strings.HasPrefix(current, "--") {
		return "", 0
	}
	trimmed := strings.TrimPrefix(current, "-")
	if len(trimmed) == 0 {
		return "", 0
	}
	splitIndex := strings.Index(trimmed, "=")
	shorthand := trimmed
	if splitIndex >= 0 {
		shorthand = trimmed[:splitIndex]
	}
	if len(shorthand) != 1 {
		return "", 0
	}
	if !isToggleShorthand(shorthand) {
		return "", 0
	}
	if splitIndex >= 0 {
		return current, 1
	}
	if index+1 >= len(arguments) {
		return current, 1
	}
	nextValue := arguments[index+1]
	if startsWithDash(nextValue) {
		return current, 1
	}
	return current + "=" + nextValue, 2
}

func isToggleName(name string) bool {
	toggleFlagRegistryMutex.RLock()
	defer toggleFlagRegistryMutex.RUnlock()
	_, exists := toggleFlagNames[name]
	return exists
}

func isToggleShorthand(shorthand string) bool {
	toggleFlagRegistryMutex.RLock()
	defer toggleFlagRegistryMutex.RUnlock()
	_, exists := toggleFlagShorthands[shorthand]
	return exists
}

func startsWithDash(value string) bool {
	return strings.HasPrefix(value, "-")
}
