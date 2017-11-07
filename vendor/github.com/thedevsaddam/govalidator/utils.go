package govalidator

import (
	"fmt"
	"reflect"
	"strings"
)

// containsRequiredField check rules contain any required field
func isContainRequiredField(rules []string) bool {
	for _, rule := range rules {
		if rule == "required" {
			return true
		}
	}
	return false
}

// isRuleExist check if the provided rule name is exist or not
func isRuleExist(rule string) bool {
	if strings.Contains(rule, ":") {
		rule = strings.Split(rule, ":")[0]
	}
	if _, ok := rulesFuncMap[rule]; ok {
		return true
	}
	return false
}

// toString force data to be string
func toString(v interface{}) string {
	str, ok := v.(string)
	if !ok {
		str = fmt.Sprintf("%#v", v)
	}
	return str
}

// isEmpty check a type is Zero
func isEmpty(x interface{}) bool {
	rt := reflect.TypeOf(x)
	if rt == nil {
		return true
	}
	rv := reflect.ValueOf(x)
	switch rv.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return rv.Len() == 0
	}
	return reflect.DeepEqual(x, reflect.Zero(rt).Interface())
}
