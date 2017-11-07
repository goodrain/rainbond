package govalidator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

const (
	tagIdentifier = "json" //tagName idetify the struct tag for govalidator
	tagSeparator  = "|"    //tagSeparator use to separate tags in struct
)

type (
	// MapData represents basic data structure for govalidator Rules and Messages
	MapData map[string][]string

	// Options describes configuration option for validator
	Options struct {
		Data            interface{} // Data represents structure for JSON body
		Request         *http.Request
		RequiredDefault bool    // RequiredDefault represents if all the fields are by default required or not
		Rules           MapData // Rules represents rules for form-data/x-url-encoded/query params data
		Messages        MapData // Messages represents custom/localize message for rules
		TagIdentifier   string  //TagIdentifier represents struct tag identifier, e.g: json or validate etc
	}

	// Validator represents a validator with options
	Validator struct {
		Opts Options // Opts contains all the options for validator
	}
)

// New return a new validator object using provided options
func New(opts Options) *Validator {
	return &Validator{Opts: opts}
}

// getMessage return if a custom message exist against the field name and rule
// if not available it return an empty string
func (v *Validator) getCustomMessage(field, rule string) string {
	if msgList, ok := v.Opts.Messages[field]; ok {
		for _, m := range msgList {
			//if rules has params, remove params. e.g: between:3,5 would be between
			if strings.Contains(rule, ":") {
				rule = strings.Split(rule, ":")[0]
			}
			if strings.HasPrefix(m, rule+":") {
				return strings.TrimPrefix(m, rule+":")
			}
		}
	}
	return ""
}

// SetDefaultRequired change the required behavior of fields
// Default value if false
// If SetDefaultRequired set to true then it will mark all the field in the rules list as required
func (v *Validator) SetDefaultRequired(required bool) {
	v.Opts.RequiredDefault = required
}

// SetTagIdentifier change the default tag identifier (json) to your custom tag.
func (v *Validator) SetTagIdentifier(identifier string) {
	v.Opts.TagIdentifier = identifier
}

// Validate validate request data like form-data, x-www-form-urlencoded and query params
// see example in README.md file
// ref: https://github.com/thedevsaddam/govalidator#example
func (v *Validator) Validate() url.Values {
	// if request object and rules not passed rise a panic
	if len(v.Opts.Rules) == 0 || v.Opts.Request == nil {
		panic(errValidateArgsMismatch)
	}
	errsBag := url.Values{}

	// clean rules
	v.keepRequiredField()

	for field, rules := range v.Opts.Rules {
		reqVal := strings.TrimSpace(v.parseAndGetVal(field))
		for _, rule := range rules {
			if !isRuleExist(rule) {
				panic(fmt.Errorf("govalidator: %s is not a valid rule", rule))
			}
			msg := v.getCustomMessage(field, rule)
			// validate if custom rules exist
			validateCustomRules(field, rule, msg, reqVal, errsBag)
		}
	}

	return errsBag
}

// parseAndGetVal parse the incoming request object and return the value associated with the key
func (v *Validator) parseAndGetVal(key string) string {
	// v.Opts.Request.ParseMultipartForm(1024)
	// r.ParseForm()
	return v.Opts.Request.Form.Get(key)
}

// keepRequiredField remove non required rules field from rules if requiredDefault field is false
// and if the input data is empty for this field
func (v *Validator) keepRequiredField() {
	v.Opts.Request.ParseMultipartForm(1024)
	//r.ParseForm()
	inputs := v.Opts.Request.Form
	if !v.Opts.RequiredDefault {
		for k, r := range v.Opts.Rules {
			if _, ok := inputs[k]; !ok {
				if !isContainRequiredField(r) {
					delete(v.Opts.Rules, k)
				}
			}
		}
	}
}

// ValidateJSON validate request data from JSON body to Go struct
// see example in README.md file
func (v *Validator) ValidateJSON() url.Values {
	if len(v.Opts.Rules) == 0 || v.Opts.Request == nil {
		panic(errValidateArgsMismatch)
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
	r := roller{}
	r.setTagIdentifier(tagIdentifier)
	if v.Opts.TagIdentifier != "" {
		r.setTagIdentifier(v.Opts.TagIdentifier)
	}
	r.setTagSeparator(tagSeparator)
	r.start(v.Opts.Data)

	//clean if the key is not exist or value is empty or zero value
	v.keepJSONRequiredField(r.getFlatMap())

	for field, rules := range v.Opts.Rules {
		value, _ := r.getFlatVal(field)
		for _, rule := range rules {
			if !isRuleExist(rule) {
				panic(fmt.Errorf("validator: %s is not a valid rule", rule))
			}
			msg := v.getCustomMessage(field, rule)
			validateCustomRules(field, rule, msg, value, errsBag)
		}
	}

	return errsBag
}

// keepJSONRequiredField remove non required rules field from rules if requiredDefault field is false
// and if the input data is empty for this field
func (v *Validator) keepJSONRequiredField(inputs map[string]interface{}) {
	if !v.Opts.RequiredDefault {
		for k, r := range v.Opts.Rules {
			if val, _ := inputs[k]; isEmpty(val) { // TODO: need to write test to check it working fine
				if !isContainRequiredField(r) {
					delete(v.Opts.Rules, k)
				}
			}
		}
	}
}
