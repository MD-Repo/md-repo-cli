package commons

import (
	"errors"
	"fmt"

	"golang.org/x/xerrors"
)

var (
	TokenNotProvidedError      error = xerrors.Errorf("token not provided")
	InvalidOrcIDError          error = xerrors.Errorf("invalid ORC-ID")
	InvalidSubmitMetadataError error = xerrors.Errorf("invalid submit metadata")
)

type MDRepoServiceError struct {
	Message string
}

func NewMDRepoServiceError(message string) error {
	return &MDRepoServiceError{
		Message: message,
	}
}

// Error returns error message
func (err *MDRepoServiceError) Error() string {
	return err.Message
}

// Is tests type of error
func (err *MDRepoServiceError) Is(other error) bool {
	_, ok := other.(*MDRepoServiceError)
	return ok
}

// ToString stringifies the object
func (err *MDRepoServiceError) ToString() string {
	return fmt.Sprintf("MDRepoServiceError: %s", err.Message)
}

// IsMDRepoServiceError evaluates if the given error is MDRepoServiceError
func IsMDRepoServiceError(err error) bool {
	return errors.Is(err, &MDRepoServiceError{})
}

type InvalidTicketError struct {
	Ticket string
}

func NewInvalidTicketError(ticket string) error {
	return &InvalidTicketError{
		Ticket: ticket,
	}
}

// Error returns error message
func (err *InvalidTicketError) Error() string {
	return fmt.Sprintf("ticket '%s' is invalid", err.Ticket)
}

// Is tests type of error
func (err *InvalidTicketError) Is(other error) bool {
	_, ok := other.(*InvalidTicketError)
	return ok
}

// ToString stringifies the object
func (err *InvalidTicketError) ToString() string {
	return fmt.Sprintf("InvalidTicketError: %s", err.Ticket)
}

// IsInvalidTicketError evaluates if the given error is InvalidTicketError
func IsInvalidTicketError(err error) bool {
	return errors.Is(err, &InvalidTicketError{})
}

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

type NotDirError struct {
	Path string
}

func NewNotDirError(dest string) error {
	return &NotDirError{
		Path: dest,
	}
}

// Error returns error message
func (err *NotDirError) Error() string {
	return fmt.Sprintf("path %q is not a directory", err.Path)
}

// Is tests type of error
func (err *NotDirError) Is(other error) bool {
	_, ok := other.(*NotDirError)
	return ok
}

// ToString stringifies the object
func (err *NotDirError) ToString() string {
	return fmt.Sprintf("NotDirError: %q", err.Path)
}

// IsNotDirError evaluates if the given error is NotDirError
func IsNotDirError(err error) bool {
	return errors.Is(err, &NotDirError{})
}

type NotFileError struct {
	Path string
}

func NewNotFileError(dest string) error {
	return &NotFileError{
		Path: dest,
	}
}

// Error returns error message
func (err *NotFileError) Error() string {
	return fmt.Sprintf("path %q is not a file", err.Path)
}

// Is tests type of error
func (err *NotFileError) Is(other error) bool {
	_, ok := other.(*NotFileError)
	return ok
}

// ToString stringifies the object
func (err *NotFileError) ToString() string {
	return fmt.Sprintf("NotFileError: %q", err.Path)
}

// IsNotFileError evaluates if the given error is NotFileError
func IsNotFileError(err error) bool {
	return errors.Is(err, &NotFileError{})
}

type DialHTTPError struct {
	URL string
}

func NewDialHTTPError(url string) error {
	return &DialHTTPError{
		URL: url,
	}
}

// Error returns error message
func (err *DialHTTPError) Error() string {
	return fmt.Sprintf("failed to dial to %q", err.URL)
}

// Is tests type of error
func (err *DialHTTPError) Is(other error) bool {
	_, ok := other.(*DialHTTPError)
	return ok
}

// ToString stringifies the object
func (err *DialHTTPError) ToString() string {
	return fmt.Sprintf("DialHTTPError: %q", err.URL)
}

// IsDialHTTPError evaluates if the given error is DialHTTPError
func IsDialHTTPError(err error) bool {
	return errors.Is(err, &DialHTTPError{})
}
