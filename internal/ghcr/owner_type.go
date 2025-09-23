package ghcr

import (
	"fmt"
	"strings"
)

const (
	ownerTypeUserConstant              OwnerType = "user"
	ownerTypeOrganizationConstant      OwnerType = "org"
	usersPathSegmentConstant                     = "users"
	organizationsPathSegmentConstant             = "orgs"
	ownerTypeEmptyErrorMessageConstant           = "owner type must be provided"
	ownerTypeInvalidTemplateConstant             = "owner type %q is not supported"
)

// OwnerType enumerates supported GHCR owner scopes.
type OwnerType string

// UserOwnerType identifies GitHub user-owned container packages.
const UserOwnerType OwnerType = ownerTypeUserConstant

// OrganizationOwnerType identifies organization-owned container packages.
const OrganizationOwnerType OwnerType = ownerTypeOrganizationConstant

// ParseOwnerType normalizes textual owner type values.
func ParseOwnerType(ownerTypeValue string) (OwnerType, error) {
	trimmedValue := strings.TrimSpace(ownerTypeValue)
	if len(trimmedValue) == 0 {
		return "", fmt.Errorf(ownerTypeEmptyErrorMessageConstant)
	}

	lowerCasedValue := strings.ToLower(trimmedValue)
	switch OwnerType(lowerCasedValue) {
	case UserOwnerType:
		return UserOwnerType, nil
	case OrganizationOwnerType:
		return OrganizationOwnerType, nil
	default:
		return "", fmt.Errorf(ownerTypeInvalidTemplateConstant, ownerTypeValue)
	}
}

// PathSegment resolves the REST API segment for the owner type.
func (ownerType OwnerType) PathSegment() string {
	switch ownerType {
	case OrganizationOwnerType:
		return organizationsPathSegmentConstant
	default:
		return usersPathSegmentConstant
	}
}
