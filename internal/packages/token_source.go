package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	tokenSourceSeparatorConstant               = ":"
	environmentTokenSourceTypeValueConstant    = "env"
	fileTokenSourceTypeValueConstant           = "file"
	tokenSourceMissingErrorMessageConstant     = "token source must be provided"
	environmentNameMissingErrorMessageConstant = "environment variable name must be provided"
	filePathMissingErrorMessageConstant        = "token file path must be provided"
	environmentLookupNilErrorMessageConstant   = "environment lookup function not configured"
	fileReaderNilErrorMessageConstant          = "file reader function not configured"
	environmentTokenMissingTemplateConstant    = "environment variable %s is not set"
	fileReadErrorTemplateConstant              = "unable to read token file %s: %w"
	fileTokenEmptyErrorTemplateConstant        = "token file %s is empty"
	unsupportedTokenSourceTemplateConstant     = "unsupported token source type %q"
)

// TokenSourceType enumerates the supported token retrieval mechanisms.
type TokenSourceType string

// Token source type enumerations.
const (
	TokenSourceTypeEnvironment TokenSourceType = TokenSourceType(environmentTokenSourceTypeValueConstant)
	TokenSourceTypeFile        TokenSourceType = TokenSourceType(fileTokenSourceTypeValueConstant)
)

// TokenSourceConfiguration specifies how to locate a credentials token.
type TokenSourceConfiguration struct {
	Type      TokenSourceType
	Reference string
}

// TokenResolver retrieves authentication tokens from configured sources.
type TokenResolver interface {
	ResolveToken(resolutionContext context.Context, source TokenSourceConfiguration) (string, error)
}

// EnvironmentLookup obtains an environment variable value.
type EnvironmentLookup func(key string) (string, bool)

// FileReader reads the contents of a file path.
type FileReader func(path string) ([]byte, error)

// NewTokenResolver creates a token resolver with optional dependency overrides.
func NewTokenResolver(environmentLookup EnvironmentLookup, fileReader FileReader) TokenResolver {
	resolvedEnvironmentLookup := environmentLookup
	if resolvedEnvironmentLookup == nil {
		resolvedEnvironmentLookup = os.LookupEnv
	}

	resolvedFileReader := fileReader
	if resolvedFileReader == nil {
		resolvedFileReader = os.ReadFile
	}

	return &tokenResolver{
		environmentLookup: resolvedEnvironmentLookup,
		fileReader:        resolvedFileReader,
	}
}

// ParseTokenSource interprets textual token source declarations.
func ParseTokenSource(sourceValue string) (TokenSourceConfiguration, error) {
	trimmedValue := strings.TrimSpace(sourceValue)
	if len(trimmedValue) == 0 {
		return TokenSourceConfiguration{}, errors.New(tokenSourceMissingErrorMessageConstant)
	}

	components := strings.SplitN(trimmedValue, tokenSourceSeparatorConstant, 2)
	if len(components) == 1 {
		return TokenSourceConfiguration{
			Type:      TokenSourceTypeEnvironment,
			Reference: trimmedValue,
		}, nil
	}

	sourceType := strings.ToLower(strings.TrimSpace(components[0]))
	reference := strings.TrimSpace(components[1])

	switch sourceType {
	case environmentTokenSourceTypeValueConstant:
		if len(reference) == 0 {
			return TokenSourceConfiguration{}, errors.New(environmentNameMissingErrorMessageConstant)
		}
		return TokenSourceConfiguration{Type: TokenSourceTypeEnvironment, Reference: reference}, nil
	case fileTokenSourceTypeValueConstant:
		if len(reference) == 0 {
			return TokenSourceConfiguration{}, errors.New(filePathMissingErrorMessageConstant)
		}
		return TokenSourceConfiguration{Type: TokenSourceTypeFile, Reference: reference}, nil
	default:
		return TokenSourceConfiguration{}, fmt.Errorf(unsupportedTokenSourceTemplateConstant, sourceType)
	}
}

type tokenResolver struct {
	environmentLookup EnvironmentLookup
	fileReader        FileReader
}

func (resolver *tokenResolver) ResolveToken(resolutionContext context.Context, source TokenSourceConfiguration) (string, error) {
	_ = resolutionContext
	switch source.Type {
	case TokenSourceTypeEnvironment:
		if resolver.environmentLookup == nil {
			return "", errors.New(environmentLookupNilErrorMessageConstant)
		}
		value, found := resolver.environmentLookup(source.Reference)
		if !found {
			return "", fmt.Errorf(environmentTokenMissingTemplateConstant, source.Reference)
		}
		trimmedValue := strings.TrimSpace(value)
		if len(trimmedValue) == 0 {
			return "", fmt.Errorf(environmentTokenMissingTemplateConstant, source.Reference)
		}
		return trimmedValue, nil
	case TokenSourceTypeFile:
		if resolver.fileReader == nil {
			return "", errors.New(fileReaderNilErrorMessageConstant)
		}
		contents, readError := resolver.fileReader(source.Reference)
		if readError != nil {
			return "", fmt.Errorf(fileReadErrorTemplateConstant, source.Reference, readError)
		}
		trimmedValue := strings.TrimSpace(string(contents))
		if len(trimmedValue) == 0 {
			return "", fmt.Errorf(fileTokenEmptyErrorTemplateConstant, source.Reference)
		}
		return trimmedValue, nil
	default:
		return "", fmt.Errorf(unsupportedTokenSourceTemplateConstant, source.Type)
	}
}
