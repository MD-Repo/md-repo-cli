package types

import (
	"fmt"

	"github.com/cockroachdb/errors"
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
	var mdRepoErr *MDRepoServiceError
	return errors.As(err, &mdRepoErr)
}

type TokenNotProvidedError struct {
}

func NewTokenNotProvidedError() error {
	return &TokenNotProvidedError{}
}

// Error returns error message
func (err *TokenNotProvidedError) Error() string {
	return "token not provided"
}

// Is tests type of error
func (err *TokenNotProvidedError) Is(other error) bool {
	_, ok := other.(*TokenNotProvidedError)
	return ok
}

// ToString stringifies the object
func (err *TokenNotProvidedError) ToString() string {
	return fmt.Sprintf("TokenNotProvidedError: %s", err.Error())
}

// IsTokenNotProvidedError evaluates if the given error is TokenNotProvidedError
func IsTokenNotProvidedError(err error) bool {
	var tokenNotProvidedErr *TokenNotProvidedError
	return errors.As(err, &tokenNotProvidedErr)
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
	var invalidTicketErr *InvalidTicketError
	return errors.As(err, &invalidTicketErr)
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
	return fmt.Sprintf("the number of simulations indicated (%d) does not match the number of simulations found (%d)", err.Expected, len(err.ValidSimulationPaths))
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
	var simulationNoNotMatchingErr *SimulationNoNotMatchingError
	return errors.As(err, &simulationNoNotMatchingErr)
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
	var notDirErr *NotDirError
	return errors.As(err, &notDirErr)
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
	var notFileErr *NotFileError
	return errors.As(err, &notFileErr)
}

type InvalidOrcIDError struct {
	RequestedOrcID string
	FoundOrcID     string
}

func NewInvalidOrcIDError(requestedOrcID string, foundOrcID string) error {
	return &InvalidOrcIDError{
		RequestedOrcID: requestedOrcID,
		FoundOrcID:     foundOrcID,
	}
}

// Error returns error message
func (err *InvalidOrcIDError) Error() string {
	return fmt.Sprintf("invalid ORC-ID: %q (expected %q)", err.FoundOrcID, err.RequestedOrcID)
}

// Is tests type of error
func (err *InvalidOrcIDError) Is(other error) bool {
	_, ok := other.(*InvalidOrcIDError)
	return ok
}

// ToString stringifies the object
func (err *InvalidOrcIDError) ToString() string {
	return fmt.Sprintf("InvalidOrcIDError: %q", err.Error())
}

// IsInvalidOrcIDError evaluates if the given error is InvalidOrcIDError
func IsInvalidOrcIDError(err error) bool {
	var invalidOrcIDErr *InvalidOrcIDError
	return errors.As(err, &invalidOrcIDErr)
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
	var invalidSubmitMetadataErr *InvalidSubmitMetadataError
	return errors.As(err, &invalidSubmitMetadataErr)
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
	var webDAVErr *WebDAVError
	return errors.As(err, &webDAVErr)
}
