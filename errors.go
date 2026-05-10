package client

import "fmt"

type RemoteError struct {
	message string
}

func (e *RemoteError) Error() string {
	return e.message
}

type RemoteAuthError struct {
	RemoteError
}

type RemoteCriteriaError struct {
	RemoteError
}

func newRemoteAuthError(format string, args ...any) error {
	return &RemoteAuthError{RemoteError: RemoteError{message: fmt.Sprintf(format, args...)}}
}

func newRemoteCriteriaError(format string, args ...any) error {
	return &RemoteCriteriaError{RemoteError: RemoteError{message: fmt.Sprintf(format, args...)}}
}
