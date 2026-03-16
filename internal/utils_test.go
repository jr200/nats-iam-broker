package internal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIgnoreError(t *testing.T) {
	assert.Equal(t, 42, IgnoreError(42, errors.New("ignored")))
	assert.Equal(t, "hello", IgnoreError("hello", nil))
	assert.Equal(t, 0, IgnoreError(0, errors.New("err")))
}

func TestCollapseError(t *testing.T) {
	t.Run("with error returns error message", func(t *testing.T) {
		result := CollapseError("value", errors.New("something failed"))
		assert.Equal(t, "something failed", result)
	})

	t.Run("with string value", func(t *testing.T) {
		result := CollapseError("hello", nil)
		assert.Equal(t, "hello", result)
	})

	t.Run("with Stringer value", func(t *testing.T) {
		result := CollapseError(myStringer{}, nil)
		assert.Equal(t, "stringer_value", result)
	})

	t.Run("with int value", func(t *testing.T) {
		result := CollapseError(42, nil)
		assert.Equal(t, "42", result)
	})

	t.Run("with nil value and nil error", func(t *testing.T) {
		result := CollapseError(nil, nil)
		assert.Equal(t, "<nil>", result)
	})
}

type myStringer struct{}

func (m myStringer) String() string {
	return "stringer_value"
}

func TestParseDelimiters(t *testing.T) {
	t.Run("valid delimiters", func(t *testing.T) {
		left, right := ParseDelimiters("<<,>>")
		assert.Equal(t, "<<", left)
		assert.Equal(t, ">>", right)
	})

	t.Run("standard go delimiters", func(t *testing.T) {
		left, right := ParseDelimiters("{{,}}")
		assert.Equal(t, "{{", left)
		assert.Equal(t, "}}", right)
	})

	// Note: ParseDelimiters calls os.Exit(1) on invalid input, which can't be
	// easily tested without refactoring. The valid cases above confirm the
	// happy path works correctly.
	_ = "suppressing unused import"
}
