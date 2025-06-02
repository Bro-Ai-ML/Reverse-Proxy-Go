package errors

import "fmt"

type Error struct {
	Code    string
	Message string
	Op      string
	Err     error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("[%s] %s", e.Op, e.Message)
	}
	return fmt.Sprintf("[%s] %s: %v", e.Op, e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func New(op, message string) *Error {
	return &Error{Op: op, Message: message}
}

func Wrap(err error, op, message string) *Error {
	return &Error{
		Op:      op,
		Message: message,
		Err:     err,
	}
}
