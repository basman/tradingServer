package server

import "fmt"

type userError struct {
	Message string
}

func newUserError(msg string, args ...interface{}) userError {
	return userError{ Message: fmt.Sprintf(msg, args...)}
}
