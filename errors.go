package client

import (
	"fmt"
	"strings"
)

// RemoteError represents a generic error returned by remote Switcher API calls.
// Concrete remote error types embed RemoteError to allow type assertions by callers.
type RemoteError struct {
	message string
}

func (e *RemoteError) Error() string {
	return e.message
}

// RemoteAuthError indicates an authentication/authorization failure with the remote API.
// It embeds RemoteError.
type RemoteAuthError struct {
	RemoteError
}

// RemoteCriteriaError indicates a criteria validation or execution error returned by the remote API.
// It embeds RemoteError.
type RemoteCriteriaError struct {
	RemoteError
}

// RemoteSnapshotError indicates snapshot-related errors coming from the remote API.
// It embeds RemoteError.
type RemoteSnapshotError struct {
	RemoteError
}

// RemoteSwitcherError indicates a switcher configuration error returned by the remote API.
// It embeds RemoteError.
type RemoteSwitcherError struct {
	RemoteError
}

// LocalCriteriaError represents an error raised when local snapshot evaluation fails due to
// invalid criteria or inputs. It implements the error interface.
type LocalCriteriaError struct {
	message string
}

// LocalSwitcherError indicates a missing switcher in local snapshot validation.
type LocalSwitcherError struct {
	message string
}

func (e *LocalCriteriaError) Error() string {
	return e.message
}

func (e *LocalSwitcherError) Error() string {
	return e.message
}

func newRemoteAuthError(format string, args ...any) error {
	return &RemoteAuthError{RemoteError: RemoteError{message: fmt.Sprintf(format, args...)}}
}

func newRemoteCriteriaError(format string, args ...any) error {
	return &RemoteCriteriaError{RemoteError: RemoteError{message: fmt.Sprintf(format, args...)}}
}

func newRemoteSnapshotError(format string, args ...any) error {
	return &RemoteSnapshotError{RemoteError: RemoteError{message: fmt.Sprintf(format, args...)}}
}

func newRemoteError(format string, args ...any) error {
	return &RemoteError{message: fmt.Sprintf(format, args...)}
}

func newRemoteSwitcherError(notFound []string) error {
	if len(notFound) == 0 {
		return nil
	}

	return &RemoteSwitcherError{RemoteError: RemoteError{message: fmt.Sprintf("%s not found", strings.Join(notFound, ", "))}}
}

func newLocalCriteriaError(format string, args ...any) error {
	return &LocalCriteriaError{message: fmt.Sprintf(format, args...)}
}

func newLocalSwitcherError(notFound []string) error {
	if len(notFound) == 0 {
		return nil
	}

	return &LocalSwitcherError{message: fmt.Sprintf("%s not found", strings.Join(notFound, ", "))}
}
