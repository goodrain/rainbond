package validator

import (
	"fmt"
	"net/url"
	"strings"
)

var customRules = make(map[string]func(string, interface{}, string) error, 0)

// AddCustomRule help to add custom rules for validator
// First argument it takes the rule name and second arg a func
// Second arg must have this signature below
// fn func(fieldName string, fieldValue string, rule string) error
func AddCustomRule(name string, fn func(field string, value interface{}, rule string) error) {
	if isRuleExist(name) {
		panic(fmt.Errorf("validator: %s is already defined in rules", name))
	}
	customRules[name] = fn
}

// validateCustomRules validate custom rules
func validateCustomRules(field string, value interface{}, rule string, errsBag url.Values) {
	for k, v := range customRules {
		if k == rule || strings.HasPrefix(rule, k+":") {
			err := v(field, value, rule)
			if err != nil {
				errsBag.Add(field, err.Error())
			}
			break
		}
	}
}
