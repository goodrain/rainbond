package validator

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
)

// ValidateMapJSON validate request data from JSON body to Go map[string]interface{}
// interface{} can not be arrray, map, slice or struct
// it can be string, bool, number
// e.g: data := map[string]interface{}{"name": "John Doe", "age": 30, "single": false}
func (v *Validator) ValidateMapJSON() url.Values {
	if len(v.Opts.Rules) == 0 || v.Opts.Request == nil {
		panic(errValidateMapJSONArgsMismatch)
	}
	if reflect.TypeOf(v.Opts.Data).Kind() != reflect.Ptr {
		panic(errRequirePtr)
	}
	// TODO: check if the provided data is a map
	errsBag := url.Values{}

	defer v.Opts.Request.Body.Close()
	err := json.NewDecoder(v.Opts.Request.Body).Decode(v.Opts.Data)
	if err != nil {
		errsBag.Add("_error", err.Error())
		return errsBag
	}

	//keep the rules if the fields have value or contains required rule
	v.keepRequiredFieldForMap()

	for field, rules := range v.Opts.Rules {
		reqVal := strings.TrimSpace(v.parseAndGetMapVal(field))
		for _, rule := range rules {
			if !isRuleExist(rule) {
				panic(fmt.Errorf("validator: %s is not a valid rule", rule))
			}
			v := fieldValidator{
				errsBag: errsBag,
				field:   field,
				value:   reqVal,
				rule:    rule,
				message: v.getCustomMessage(field, rule),
			}
			v.run()
			// validate if custom rules exist
			validateCustomRules(field, reqVal, rule, errsBag)
		}
	}

	return errsBag
}

// parseAndGetVal parse the incoming request object and return the value associated with the key
func (v *Validator) parseAndGetMapVal(key string) string {
	data := *v.Opts.Data.(*map[string]interface{})
	if val, ok := data[key]; ok {
		return fmt.Sprintf("%v", val)
	}
	return ""
}

// keepRequiredFieldForMap remove non required rules field from rules if requiredDefault field is false
func (v *Validator) keepRequiredFieldForMap() {
	if !v.Opts.RequiredDefault {
		for k, r := range v.Opts.Rules {
			val := v.parseAndGetMapVal(k)
			if isZeroOfUnderlyingType(val) && !isContainRequiredField(r) {
				delete(v.Opts.Rules, k)
			}
		}
	}
}
