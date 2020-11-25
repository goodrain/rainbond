package validator

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

// callMethodByName call a method by its name
func callMethodByName(myClass interface{}, funcName string, params ...interface{}) (out []reflect.Value, err error) {
	myClassValue := reflect.ValueOf(myClass)
	m := myClassValue.MethodByName(funcName)
	if !m.IsValid() {
		return make([]reflect.Value, 0), fmt.Errorf("Method not found \"%s\"", funcName)
	}
	in := make([]reflect.Value, len(params))
	for i, param := range params {
		in[i] = reflect.ValueOf(param)
	}
	out = m.Call(in)
	return
}

// isRuleExist check if the provided rule name is exist or not
func isRuleExist(rule string) bool {
	if strings.Contains(rule, ":") {
		rule = strings.Split(rule, ":")[0]
	}
	if _, ok := rmMap[rule]; ok {
		return true
	} else if _, ok := customRules[rule]; ok {
		return true
	} else {
		return false
	}
}

// dField describe a single field for both nested and non-nested struct field
type dField struct {
	field         string
	originalValue interface{}
	value         string
	rules         []string
}

// deepFields traverse through the nested/embedded struct if exist
// return a slice of dField
func deepFields(iface interface{}, tagIdentifier, tagSeparator string, UniqueKey bool) []dField {
	fields := make([]dField, 0)
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	if ift.Kind() == reflect.Ptr {
		ifv = ifv.Elem()
		ift = ift.Elem()
	}

	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		rfv := ift.Field(i)

		switch v.Kind() {
		case reflect.Struct:
			fields = append(fields, deepFields(v.Interface(), tagIdentifier, tagSeparator, UniqueKey)...)
		default:
			tags := strings.Split(rfv.Tag.Get(tagIdentifier), tagSeparator)
			fieldName := tags[0]
			if fieldName == "" {
				continue
			}
			value := fmt.Sprintf("%v", v)
			rules := tags[1:]
			if UniqueKey {
				fields = append(fields, dField{field: ift.Name() + "." + fieldName, originalValue: v.Interface(), value: value, rules: rules})
			} else {
				fields = append(fields, dField{field: fieldName, originalValue: v.Interface(), value: value, rules: rules})
			}
		}
	}
	return fields
}

// keepRequiredFields remove the rules which does not contain requried field if SetDefaultRequired is false
func keepRequiredFields(dfs []dField) []dField {
	fields := make([]dField, 0)
	for _, f := range dfs {
		if !isZeroOfUnderlyingType(f.originalValue) || isContainRequiredField(f.rules) { // TODO: need to update the logic
			fields = append(fields, f)
		}
	}
	return fields
}

// isZeroOfUnderlyingType detect if the provided type is in its zero value state
func isZeroOfUnderlyingType(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}
