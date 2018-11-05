package template

import (
	"fmt"
	"reflect"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var defaults = map[string]string{
	"FOO": "first",
	"BAR": "",
}

func defaultMapping(name string) (string, bool) {
	val, ok := defaults[name]
	return val, ok
}

func TestEscaped(t *testing.T) {
	result, err := Substitute("$${foo}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("${foo}", result))
}

func TestSubstituteNoMatch(t *testing.T) {
	result, err := Substitute("foo", defaultMapping)
	assert.NilError(t, err)
	assert.Equal(t, "foo", result)
}

func TestInvalid(t *testing.T) {
	invalidTemplates := []string{
		"${",
		"$}",
		"${}",
		"${ }",
		"${ foo}",
		"${foo }",
		"${foo!}",
	}

	for _, template := range invalidTemplates {
		_, err := Substitute(template, defaultMapping)
		assert.ErrorContains(t, err, "Invalid template")
	}
}

func TestNoValueNoDefault(t *testing.T) {
	for _, template := range []string{"This ${missing} var", "This ${BAR} var"} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal("This  var", result))
	}
}

func TestValueNoDefault(t *testing.T) {
	for _, template := range []string{"This $FOO var", "This ${FOO} var"} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal("This first var", result))
	}
}

func TestNoValueWithDefault(t *testing.T) {
	for _, template := range []string{"ok ${missing:-def}", "ok ${missing-def}"} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal("ok def", result))
	}
}

func TestEmptyValueWithSoftDefault(t *testing.T) {
	result, err := Substitute("ok ${BAR:-def}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok def", result))
}

func TestValueWithSoftDefault(t *testing.T) {
	result, err := Substitute("ok ${FOO:-def}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok first", result))
}

func TestEmptyValueWithHardDefault(t *testing.T) {
	result, err := Substitute("ok ${BAR-def}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))
}

func TestNonAlphanumericDefault(t *testing.T) {
	result, err := Substitute("ok ${BAR:-/non:-alphanumeric}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok /non:-alphanumeric", result))
}

func TestMandatoryVariableErrors(t *testing.T) {
	testCases := []struct {
		template      string
		expectedError string
	}{
		{
			template:      "not ok ${UNSET_VAR:?Mandatory Variable Unset}",
			expectedError: "required variable UNSET_VAR is missing a value: Mandatory Variable Unset",
		},
		{
			template:      "not ok ${BAR:?Mandatory Variable Empty}",
			expectedError: "required variable BAR is missing a value: Mandatory Variable Empty",
		},
		{
			template:      "not ok ${UNSET_VAR:?}",
			expectedError: "required variable UNSET_VAR is missing a value",
		},
		{
			template:      "not ok ${UNSET_VAR?Mandatory Variable Unset}",
			expectedError: "required variable UNSET_VAR is missing a value: Mandatory Variable Unset",
		},
		{
			template:      "not ok ${UNSET_VAR?}",
			expectedError: "required variable UNSET_VAR is missing a value",
		},
	}

	for _, tc := range testCases {
		_, err := Substitute(tc.template, defaultMapping)
		assert.ErrorContains(t, err, tc.expectedError)
		assert.ErrorType(t, err, reflect.TypeOf(&InvalidTemplateError{}))
	}
}

func TestDefaultsForMandatoryVariables(t *testing.T) {
	testCases := []struct {
		template string
		expected string
	}{
		{
			template: "ok ${FOO:?err}",
			expected: "ok first",
		},
		{
			template: "ok ${FOO?err}",
			expected: "ok first",
		},
		{
			template: "ok ${BAR?err}",
			expected: "ok ",
		},
	}

	for _, tc := range testCases {
		result, err := Substitute(tc.template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(tc.expected, result))
	}
}

func TestSubstituteWithCustomFunc(t *testing.T) {
	errIsMissing := func(substitution string, mapping Mapping) (string, bool, error) {
		value, found := mapping(substitution)
		if !found {
			return "", true, &InvalidTemplateError{
				Template: fmt.Sprintf("required variable %s is missing a value", substitution),
			}
		}
		return value, true, nil
	}

	result, err := SubstituteWith("ok ${FOO}", defaultMapping, defaultPattern, errIsMissing)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok first", result))

	result, err = SubstituteWith("ok ${BAR}", defaultMapping, defaultPattern, errIsMissing)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))

	_, err = SubstituteWith("ok ${NOTHERE}", defaultMapping, defaultPattern, errIsMissing)
	assert.Check(t, is.ErrorContains(err, "required variable"))
}

func TestExtractVariables(t *testing.T) {
	testCases := []struct {
		name     string
		dict     map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "empty",
			dict:     map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name: "no-variables",
			dict: map[string]interface{}{
				"foo": "bar",
			},
			expected: map[string]string{},
		},
		{
			name: "variable-without-curly-braces",
			dict: map[string]interface{}{
				"foo": "$bar",
			},
			expected: map[string]string{
				"bar": "",
			},
		},
		{
			name: "variable",
			dict: map[string]interface{}{
				"foo": "${bar}",
			},
			expected: map[string]string{
				"bar": "",
			},
		},
		{
			name: "required-variable",
			dict: map[string]interface{}{
				"foo": "${bar?:foo}",
			},
			expected: map[string]string{
				"bar": "",
			},
		},
		{
			name: "required-variable2",
			dict: map[string]interface{}{
				"foo": "${bar?foo}",
			},
			expected: map[string]string{
				"bar": "",
			},
		},
		{
			name: "default-variable",
			dict: map[string]interface{}{
				"foo": "${bar:-foo}",
			},
			expected: map[string]string{
				"bar": "foo",
			},
		},
		{
			name: "default-variable2",
			dict: map[string]interface{}{
				"foo": "${bar-foo}",
			},
			expected: map[string]string{
				"bar": "foo",
			},
		},
		{
			name: "multiple-values",
			dict: map[string]interface{}{
				"foo": "${bar:-foo}",
				"bar": map[string]interface{}{
					"foo": "${fruit:-banana}",
					"bar": "vegetable",
				},
				"baz": []interface{}{
					"foo",
					"$docker:${project:-cli}",
					"$toto",
				},
			},
			expected: map[string]string{
				"bar":     "foo",
				"fruit":   "banana",
				"toto":    "",
				"docker":  "",
				"project": "cli",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := ExtractVariables(tc.dict, defaultPattern)
			assert.Check(t, is.DeepEqual(actual, tc.expected))
		})
	}
}
