package validator

import "errors"

var (
	errStringToInt                 = errors.New("validator: unable to parse string to integer")
	errValidateArgsMismatch        = errors.New("validator: provide at least *http.Request and rules for Validate method")
	errValidateMapJSONArgsMismatch = errors.New("validator: provide at least *http.Request and rules for ValidateMapJSON method")
	errValidateJSONArgsMismatch    = errors.New("validator: provide at least *http.Request and data structure for ValidateJSON method")
	errInvalidArgument             = errors.New("validator: invalid number of argument")
	errRequirePtr                  = errors.New("validator: provide pointer to the data structure")
)
