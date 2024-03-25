package command

import (
	"fmt"
)

type EnumValue[T any] struct {
	ptr     *T
	options map[string]T
	current string
}

func NewEnumValue[T any](ptr *T, def string, values map[string]T) *EnumValue[T] {
	*ptr = values[def]
	return &EnumValue[T]{
		ptr:     ptr,
		current: def,
		options: values,
	}
}

func (e *EnumValue[T]) String() string {
	return e.current
}

func (e *EnumValue[T]) Set(v string) error {
	value, exists := e.options[v]
	if !exists {
		return fmt.Errorf("option %q is invalid", v)
	}

	*e.ptr = value
	e.current = v
	return nil
}

func (e *EnumValue[T]) Type() string {
	return "string"
}
