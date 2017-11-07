package govalidator

import "errors"

var (
	errStringToInt                 = errors.New("govalidator: unable to parse string to integer")
	errStringToFloat               = errors.New("govalidator: unable to parse string to float")
	errValidateArgsMismatch        = errors.New("govalidator: provide at least *http.Request and rules for Validate method")
	errValidateMapJSONArgsMismatch = errors.New("govalidator: provide at least *http.Request and rules for ValidateMapJSON method")
	errValidateJSONArgsMismatch    = errors.New("govalidator: provide at least *http.Request and data structure for ValidateJSON method")
	errInvalidArgument             = errors.New("govalidator: invalid number of argument")
	errRequirePtr                  = errors.New("govalidator: provide pointer to the data structure")
)
