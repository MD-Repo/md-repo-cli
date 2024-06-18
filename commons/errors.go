package commons

import (
	"errors"
	"fmt"

	"golang.org/x/xerrors"
)

var (
	InvalidTicketError    error = xerrors.Errorf("invalid ticket string")
	InvalidTokenError     error = xerrors.Errorf("invalid token")
	TokenNotProvidedError error = xerrors.Errorf("token not provided")
	TicketNotReadyError   error = xerrors.Errorf("ticket not ready")
	InvalidOrcIDError     error = xerrors.Errorf("invalid ORC-ID")
	//SimulationNoNotMatchingError error = xerrors.Errorf("simulation number not match")
	InvalidSubmitMetadataError error = xerrors.Errorf("invalid submit metadata")
)

type SimulationNoNotMatchingError struct {
	ValidSimulationPaths   []string
	InvalidSimulationPaths []string
	Expected               int
}

// NewSimulationNoNotMatchingError creates a simulation no not matching error
func NewSimulationNoNotMatchingError(valid []string, invalid []string, expected int) error {
	return &SimulationNoNotMatchingError{
		ValidSimulationPaths:   valid,
		InvalidSimulationPaths: invalid,
		Expected:               expected,
	}
}

// Error returns error message
func (err *SimulationNoNotMatchingError) Error() string {
	return fmt.Sprintf("the number of simulations typed (%d) does not match the number of simulations found (%d)", err.Expected, len(err.ValidSimulationPaths))
}

// Is tests type of error
func (err *SimulationNoNotMatchingError) Is(other error) bool {
	_, ok := other.(*SimulationNoNotMatchingError)
	return ok
}

// ToString stringifies the object
func (err *SimulationNoNotMatchingError) ToString() string {
	return "<SimulationNoNotMatchingError>"
}

// IsSimulationNoNotMatchingError evaluates if the given error is SimulationNoNotMatchingError
func IsSimulationNoNotMatchingError(err error) bool {
	return errors.Is(err, &SimulationNoNotMatchingError{})
}
