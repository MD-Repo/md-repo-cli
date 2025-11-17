package commons

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

var (
	TokenNotProvidedError error = errors.Errorf("token not provided")
	InvalidOrcIDError     error = errors.Errorf("invalid ORC-ID")
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
	ValidSimulationPaths         []string
	InvalidSimulationPaths       []string
	InvalidSimulationPathsErrors []error
	Expected                     int
}

// NewSimulationNoNotMatchingError creates a simulation no not matching error
func NewSimulationNoNotMatchingError(valid []string, invalid []string, invalidErrors []error, expected int) error {
	return &SimulationNoNotMatchingError{
		ValidSimulationPaths:         valid,
		InvalidSimulationPaths:       invalid,
		InvalidSimulationPathsErrors: invalidErrors,
		Expected:                     expected,
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

type InvalidSubmitMetadataError struct {
	Errors []error
}

func NewInvalidSubmitMetadataError() error {
	return &InvalidSubmitMetadataError{
		Errors: []error{},
	}
}

func (err *InvalidSubmitMetadataError) Add(message error) {
	if err.Errors == nil {
		err.Errors = []error{}
	}
	err.Errors = append(err.Errors, message)
}

func (err *InvalidSubmitMetadataError) ErrorLen() int {
	return len(err.Errors)
}

// Error returns error message
func (err *InvalidSubmitMetadataError) Error() string {
	message := ""
	for idx, e := range err.Errors {
		message += fmt.Sprintf("%d. %s\n", idx+1, e.Error())
	}

	return fmt.Sprintf("invalid submit metadata\n%s", message)
}

// Is tests type of error
func (err *InvalidSubmitMetadataError) Is(other error) bool {
	_, ok := other.(*InvalidSubmitMetadataError)
	return ok
}

// ToString stringifies the object
func (err *InvalidSubmitMetadataError) ToString() string {
	return fmt.Sprintf("InvalidSubmitMetadataError: \n%q", err.Error())
}

// IsInvalidSubmitMetadataError evaluates if the given error is InvalidSubmitMetadataError
func IsInvalidSubmitMetadataError(err error) bool {
	return errors.Is(err, &InvalidSubmitMetadataError{})
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

type WebDAVError struct {
	URL       string
	ErrorCode int
}

func NewWebDAVError(url string, errorCode int) error {
	return &WebDAVError{
		URL:       url,
		ErrorCode: errorCode,
	}
}

// Error returns error message
func (err *WebDAVError) Error() string {
	return fmt.Sprintf("failed to access %q, received %d error", err.URL, err.ErrorCode)
}

// Is tests type of error
func (err *WebDAVError) Is(other error) bool {
	_, ok := other.(*WebDAVError)
	return ok
}

// ToString stringifies the object
func (err *WebDAVError) ToString() string {
	return fmt.Sprintf("WebDAVError: %q (error %d)", err.URL, err.ErrorCode)
}

// IsWebDAVError evaluates if the given error is WebDAVError
func IsWebDAVError(err error) bool {
	return errors.Is(err, &WebDAVError{})
}
