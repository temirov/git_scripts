package workflow

import (
	"errors"
	"fmt"

	repoerrors "github.com/temirov/gix/internal/repos/errors"
)

func logRepositoryOperationError(environment *Environment, err error) bool {
	if environment == nil {
		return true
	}

	var operationError repoerrors.OperationError
	if !errors.As(err, &operationError) {
		return false
	}

	if environment.Errors != nil {
		message := operationError.Message()
		if len(message) == 0 {
			message = operationError.Error()
		}
		fmt.Fprint(environment.Errors, message)
	}

	return true
}
