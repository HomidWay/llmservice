package helpers

import (
	"fmt"
	"reflect"
)

type InvalidOptionError struct {
	ExpectedType string
	ActualType   string
}

func (e InvalidOptionError) Error() string {
	return fmt.Sprintf("invalid option: expected %s, got %s",
		e.ExpectedType, e.ActualType)
}

func NewInvalidOptionError(expected, actual interface{}) error {
	return &InvalidOptionError{
		ExpectedType: reflect.TypeOf(expected).String(),
		ActualType:   reflect.TypeOf(actual).String(),
	}
}
