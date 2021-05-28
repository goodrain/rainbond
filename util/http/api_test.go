package http

import (
	"errors"
	"testing"

	"github.com/coreos/pkg/multierror"
	"github.com/stretchr/testify/assert"
)

func TestConvertMultiError(t *testing.T) {
	errs := []error{
		errors.New("apple"),
		errors.New("banana"),
	}
	err := multierror.Error(errs)
	result := convertMultiError(err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Data)
	arr := result.Data.([]string)
	assert.ElementsMatch(t, []string{"apple", "banana"}, arr)
}
