package validator

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
)

// ValidateStructJSON validate request data from JSON body to Go struct
// see example in README.md file
func (v *Validator) ValidateStructJSON() url.Values {
	if v.Opts.Request == nil {
		panic(errValidateJSONArgsMismatch)
	}
	if reflect.TypeOf(v.Opts.Data).Kind() != reflect.Ptr {
		panic(errRequirePtr)
	}
	errsBag := url.Values{}

	defer v.Opts.Request.Body.Close()
	err := json.NewDecoder(v.Opts.Request.Body).Decode(v.Opts.Data)
	if err != nil {
		errsBag.Add("_error", err.Error())
		return errsBag
	}
	df := deepFields(v.Opts.Data, tagIdentifier, tagSeparator, v.Opts.UniqueKey)

	if !v.Opts.RequiredDefault {
		// clean rules
		df = keepRequiredFields(df) // TODO: need to rethink about it
	}

	for _, d := range df {
		for _, rule := range d.rules {
			if !isRuleExist(rule) {
				panic(fmt.Errorf("validator: %s is not a valid rule", rule))
			}
			fv := fieldValidator{
				errsBag: errsBag,
				field:   d.field,
				value:   d.value,
				rule:    rule,
				message: v.getCustomMessage(d.field, rule),
			}
			fv.run()
			// validate if custom rules exist
			validateCustomRules(d.field, d.originalValue, rule, errsBag)
		}
	}

	return errsBag
}

// SetUniqueKey represents struct field name with prefix of struct name
// helps to stop collission between same field name in embeded struct
func (v *Validator) SetUniqueKey(unique bool) {
	v.Opts.RequiredDefault = unique
}
