package govalidator

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

var rulesFuncMap = make(map[string]func(string, string, string, interface{}) error, 0)

// AddCustomRule help to add custom rules for validator
// First argument it takes the rule name and second arg a func
// Second arg must have this signature below
// fn func(name string, fn func(field string, rule string, message string, value interface{}) error
// see example in readme: https://github.com/thedevsaddam/govalidator#add-custom-rules
func AddCustomRule(name string, fn func(field string, rule string, message string, value interface{}) error) {
	if isRuleExist(name) {
		panic(fmt.Errorf("validator: %s is already defined in rules", name))
	}
	rulesFuncMap[name] = fn
}

// validateCustomRules validate custom rules
func validateCustomRules(field string, rule string, message string, value interface{}, errsBag url.Values) {
	for k, v := range rulesFuncMap {
		if k == rule || strings.HasPrefix(rule, k+":") {
			err := v(field, rule, message, value)
			if err != nil {
				errsBag.Add(field, err.Error())
			}
			break
		}
	}
}

func init() {

	// Required check the Required fields
	AddCustomRule("required", func(field, rule, message string, value interface{}) error {
		err := fmt.Errorf("The %s field is required", field)
		if message != "" {
			err = errors.New(message)
		}
		if value == nil {
			return err
		}
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			if rv.Len() == 0 {
				return err
			}
		case reflect.Int:
			if value.(int) == 0 {
				return err
			}
		case reflect.Int8:
			if value.(int8) == 0 {
				return err
			}
		case reflect.Int16:
			if value.(int16) == 0 {
				return err
			}
		case reflect.Int32:
			if value.(int32) == 0 {
				return err
			}
		case reflect.Int64:
			if value.(int64) == 0 {
				return err
			}
		case reflect.Float32:
			if value.(float32) == 0 {
				return err
			}
		case reflect.Float64:
			if value.(float64) == 0 {
				return err
			}
		case reflect.Uint:
			if value.(uint) == 0 {
				return err
			}
		case reflect.Uint8:
			if value.(uint8) == 0 {
				return err
			}
		case reflect.Uint16:
			if value.(uint16) == 0 {
				return err
			}
		case reflect.Uint32:
			if value.(uint32) == 0 {
				return err
			}
		case reflect.Uint64:
			if value.(uint64) == 0 {
				return err
			}
		case reflect.Uintptr:
			if value.(uintptr) == 0 {
				return err
			}
		default:
			panic("validtor: invalid type for required")

		}
		return nil
	})

	// Regex check the custom Regex rules
	// Regex:^[a-zA-Z]+$ means this field can only contain alphabet (a-z and A-Z)
	AddCustomRule("regex", func(field, rule, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field format is invalid", field)
		if message != "" {
			err = errors.New(message)
		}
		rxStr := strings.TrimPrefix(rule, "regex:")
		if !isMatchedRegex(rxStr, str) {
			return err
		}
		return nil
	})

	// Alpha check if provided field contains valid letters
	AddCustomRule("alpha", func(field string, vlaue string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s may only contain letters", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isAlpha(str) {
			return err
		}
		return nil
	})

	// AlphaDash check if provided field contains valid letters, numbers, underscore and dash
	AddCustomRule("alpha_dash", func(field string, vlaue string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s may only contain letters, numbers, and dashes", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isAlphaDash(str) {
			return err
		}
		return nil
	})

	// AlphaNumeric check if provided field contains valid letters and numbers
	AddCustomRule("alpha_num", func(field string, vlaue string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s may only contain letters and numbers", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isAlphaNumeric(str) {
			return err
		}
		return nil
	})

	// Boolean check if provided field contains Boolean
	// in this case: "0", "1", 0, 1, "true", "false", true, false etc
	AddCustomRule("bool", func(field string, vlaue string, message string, value interface{}) error {
		err := fmt.Errorf("The %s may only contain boolean value, string or int 0, 1", field)
		if message != "" {
			err = errors.New(message)
		}
		switch value.(type) {
		case bool:
			//if value is boolean then pass
		case string:
			if !isBoolean(value.(string)) {
				return err
			}
		case int:
			v := value.(int)
			if v != 0 && v != 1 {
				return err
			}
		case int8:
			v := value.(int8)
			if v != 0 && v != 1 {
				return err
			}
		case int16:
			v := value.(int16)
			if v != 0 && v != 1 {
				return err
			}
		case int32:
			v := value.(int32)
			if v != 0 && v != 1 {
				return err
			}
		case int64:
			v := value.(int64)
			if v != 0 && v != 1 {
				return err
			}
		case uint:
			v := value.(uint)
			if v != 0 && v != 1 {
				return err
			}
		case uint8:
			v := value.(uint8)
			if v != 0 && v != 1 {
				return err
			}
		case uint16:
			v := value.(uint16)
			if v != 0 && v != 1 {
				return err
			}
		case uint32:
			v := value.(uint32)
			if v != 0 && v != 1 {
				return err
			}
		case uint64:
			v := value.(uint64)
			if v != 0 && v != 1 {
				return err
			}
		case uintptr:
			v := value.(uintptr)
			if v != 0 && v != 1 {
				return err
			}
		}
		return nil
	})

	// Between check the fields character length range
	// if the field is array, map, slice then the valdiation rule will be the length of the data
	// if the value is int or float then the valdiation rule will be the value comparison
	AddCustomRule("between", func(field string, rule string, message string, value interface{}) error {
		rng := strings.Split(strings.TrimPrefix(rule, "between:"), ",")
		if len(rng) != 2 {
			panic(errInvalidArgument)
		}
		minFloat, err := strconv.ParseFloat(rng[0], 64)
		if err != nil {
			panic(errStringToInt)
		}
		maxFloat, err := strconv.ParseFloat(rng[1], 64)
		if err != nil {
			panic(errStringToInt)
		}
		min := int(minFloat)

		max := int(maxFloat)

		err = fmt.Errorf("The %s field must be between %d and %d", field, min, max)
		if message != "" {
			err = errors.New(message)
		}
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
			inLen := rv.Len()
			if !(inLen >= min && inLen <= max) {
				return err
			}
		case reflect.Int:
			in := value.(int)
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Int8:
			in := int(value.(int8))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Int16:
			in := int(value.(int16))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Int32:
			in := int(value.(int32))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Int64:
			in := int(value.(int64))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Uint:
			in := int(value.(uint))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Uint8:
			in := int(value.(uint8))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Uint16:
			in := int(value.(uint16))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Uint32:
			in := int(value.(uint32))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Uint64:
			in := int(value.(uint64))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Uintptr:
			in := int(value.(uintptr))
			if !(in >= min && in <= max) {
				return err
			}
		case reflect.Float32:
			in := float64(value.(float32))
			if !(in >= minFloat && in <= maxFloat) {
				return fmt.Errorf("The %s field must be between %f and %f", field, minFloat, maxFloat)
			}
		case reflect.Float64:
			in := value.(float64)
			if !(in >= minFloat && in <= maxFloat) {
				return fmt.Errorf("The %s field must be between %f and %f", field, minFloat, maxFloat)
			}

		}

		return nil
	})

	// CreditCard check if provided field contains valid credit card number
	// Accepted cards are Visa, MasterCard, American Express, Diners Club, Discover and JCB card
	AddCustomRule("credit_card", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a valid credit card number", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isCreditCard(str) {
			return err
		}
		return nil
	})

	// Coordinate check if provided field contains valid Coordinate
	AddCustomRule("coordinate", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a valid coordinate", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isCoordinate(str) {
			return err
		}
		return nil
	})

	// ValidateCSSColor check if provided field contains a valid CSS color code
	AddCustomRule("css_color", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a valid CSS color code", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isCSSColor(str) {
			return err
		}
		return nil
	})

	// Digits check the exact matching length of digit (0,9)
	// Digits:5 means the field must have 5 digit of length.
	// e.g: 12345 or 98997 etc
	AddCustomRule("digits", func(field string, rule string, message string, value interface{}) error {
		l, err := strconv.Atoi(strings.TrimPrefix(rule, "digits:"))
		if err != nil {
			panic(errStringToInt)
		}
		err = fmt.Errorf("The %s field must be %d digits", field, l)
		if l == 1 {
			err = fmt.Errorf("The %s field must be 1 digit", field)
		}
		if message != "" {
			err = errors.New(message)
		}
		str := toString(value)
		if len(str) != l || !isNumeric(str) {
			return err
		}

		return nil
	})

	// DigitsBetween check if the field contains only digit and length between provided range
	// e.g: digits_between:4,5 means the field can have value like: 8887 or 12345 etc
	AddCustomRule("digits_between", func(field string, rule string, message string, value interface{}) error {
		rng := strings.Split(strings.TrimPrefix(rule, "digits_between:"), ",")
		if len(rng) != 2 {
			panic(errInvalidArgument)
		}
		min, err := strconv.Atoi(rng[0])
		if err != nil {
			panic(errStringToInt)
		}
		max, err := strconv.Atoi(rng[1])
		if err != nil {
			panic(errStringToInt)
		}
		err = fmt.Errorf("The %s field must be digits between %d and %d", field, min, max)
		if message != "" {
			err = errors.New(message)
		}
		str := toString(value)
		if !isNumeric(str) || !(len(str) >= min && len(str) <= max) {
			return err
		}

		return nil
	})

	// Date check the provided field is valid Date
	AddCustomRule("date", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		if rule == "date:dd-mm-yyyy" {
			if !isDateDDMMYY(str) {
				if message != "" {
					return errors.New(message)
				}
				return fmt.Errorf("The %s field must be a valid date format. e.g: dd-mm-yyyy, dd/mm/yyyy etc", field)
			}
		}
		if !isDate(str) {
			if message != "" {
				return errors.New(message)
			}
			return fmt.Errorf("The %s field must be a valid date format. e.g: yyyy-mm-dd, yyyy/mm/dd etc", field)
		}
		return nil
	})

	// Email check the provided field is valid Email
	AddCustomRule("email", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a valid email address", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isEmail(str) {
			return err
		}
		return nil
	})

	// validFloat check the provided field is valid float number
	AddCustomRule("float", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a float number", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isFloat(str) {
			return err
		}
		return nil
	})

	// IP check if provided field is valid IP address
	AddCustomRule("ip", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a valid IP address", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isIP(str) {
			return err
		}
		return nil
	})

	// IP check if provided field is valid IP v4 address
	AddCustomRule("ip_v4", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a valid IPv4 address", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isIPV4(str) {
			return err
		}
		return nil
	})

	// IP check if provided field is valid IP v6 address
	AddCustomRule("ip_v6", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be a valid IPv6 address", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isIPV6(str) {
			return err
		}
		return nil
	})

	// ValidateJSON check if provided field contains valid json string
	AddCustomRule("json", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must contain valid JSON string", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isJSON(str) {
			return err
		}
		return nil
	})

	/// Latitude check if provided field contains valid Latitude
	AddCustomRule("lat", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must contain valid latitude", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isLatitude(str) {
			return err
		}
		return nil
	})

	// Longitude check if provided field contains valid Longitude
	AddCustomRule("lon", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must contain valid longitude", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isLongitude(str) {
			return err
		}
		return nil
	})

	// Length check the field's character Length
	AddCustomRule("len", func(field string, rule string, message string, value interface{}) error {
		l, err := strconv.Atoi(strings.TrimPrefix(rule, "len:"))
		if err != nil {
			panic(errStringToInt)
		}
		err = fmt.Errorf("The %s field must be length of %d", field, l)
		if message != "" {
			err = errors.New(message)
		}
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
			vLen := rv.Len()
			if vLen != l {
				return err
			}
		default:
			str := toString(value) //force the value to be string
			if len(str) != l {
				return err
			}
		}

		return nil
	})

	// Min check the field's minimum character length for string, value for int, float and size for array, map, slice
	AddCustomRule("min", func(field string, rule string, message string, value interface{}) error {
		mustLen := strings.TrimPrefix(rule, "min:")
		lenInt, err := strconv.Atoi(mustLen)
		if err != nil {
			panic(errStringToInt)
		}
		lenFloat, err := strconv.ParseFloat(mustLen, 64)
		if err != nil {
			panic(errStringToFloat)
		}
		errMsg := fmt.Errorf("The %s field value can not be less than %d", field, lenInt)
		if message != "" {
			errMsg = errors.New(message)
		}
		errMsgFloat := fmt.Errorf("The %s field value can not be less than %f", field, lenFloat)
		if message != "" {
			errMsgFloat = errors.New(message)
		}
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.String:
			inLen := rv.Len()
			if inLen < lenInt {
				if message != "" {
					return errors.New(message)
				}
				return fmt.Errorf("The %s field must be minimum %d char", field, lenInt)
			}
		case reflect.Array, reflect.Map, reflect.Slice:
			inLen := rv.Len()
			if inLen < lenInt {
				if message != "" {
					return errors.New(message)
				}
				return fmt.Errorf("The %s field must be minimum %d in size", field, lenInt)
			}
		case reflect.Int:
			in := value.(int)
			if in < lenInt {
				return errMsg
			}
		case reflect.Int8:
			in := int(value.(int8))
			if in < lenInt {
				return errMsg
			}
		case reflect.Int16:
			in := int(value.(int16))
			if in < lenInt {
				return errMsg
			}
		case reflect.Int32:
			in := int(value.(int32))
			if in < lenInt {
				return errMsg
			}
		case reflect.Int64:
			in := int(value.(int64))
			if in < lenInt {
				return errMsg
			}
		case reflect.Uint:
			in := int(value.(uint))
			if in < lenInt {
				return errMsg
			}
		case reflect.Uint8:
			in := int(value.(uint8))
			if in < lenInt {
				return errMsg
			}
		case reflect.Uint16:
			in := int(value.(uint16))
			if in < lenInt {
				return errMsg
			}
		case reflect.Uint32:
			in := int(value.(uint32))
			if in < lenInt {
				return errMsg
			}
		case reflect.Uint64:
			in := int(value.(uint64))
			if in < lenInt {
				return errMsg
			}
		case reflect.Uintptr:
			in := int(value.(uintptr))
			if in < lenInt {
				return errMsg
			}
		case reflect.Float32:
			in := value.(float32)
			if in < float32(lenFloat) {
				return errMsgFloat
			}
		case reflect.Float64:
			in := value.(float64)
			if in < lenFloat {
				return errMsgFloat
			}

		}

		return nil
	})

	// Max check the field's maximum character length for string, value for int, float and size for array, map, slice
	AddCustomRule("max", func(field string, rule string, message string, value interface{}) error {
		mustLen := strings.TrimPrefix(rule, "max:")
		lenInt, err := strconv.Atoi(mustLen)
		if err != nil {
			panic(errStringToInt)
		}
		lenFloat, err := strconv.ParseFloat(mustLen, 64)
		if err != nil {
			panic(errStringToFloat)
		}
		errMsg := fmt.Errorf("The %s field value can not be greater than %d", field, lenInt)
		if message != "" {
			errMsg = errors.New(message)
		}
		errMsgFloat := fmt.Errorf("The %s field value can not be greater than %f", field, lenFloat)
		if message != "" {
			errMsgFloat = errors.New(message)
		}
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.String:
			inLen := rv.Len()
			if inLen > lenInt {
				if message != "" {
					return errors.New(message)
				}
				return fmt.Errorf("The %s field must be maximum %d char", field, lenInt)
			}
		case reflect.Array, reflect.Map, reflect.Slice:
			inLen := rv.Len()
			if inLen > lenInt {
				if message != "" {
					return errors.New(message)
				}
				return fmt.Errorf("The %s field must be minimum %d in size", field, lenInt)
			}
		case reflect.Int:
			in := value.(int)
			if in > lenInt {
				return errMsg
			}
		case reflect.Int8:
			in := int(value.(int8))
			if in > lenInt {
				return errMsg
			}
		case reflect.Int16:
			in := int(value.(int16))
			if in > lenInt {
				return errMsg
			}
		case reflect.Int32:
			in := int(value.(int32))
			if in > lenInt {
				return errMsg
			}
		case reflect.Int64:
			in := int(value.(int64))
			if in > lenInt {
				return errMsg
			}
		case reflect.Uint:
			in := int(value.(uint))
			if in > lenInt {
				return errMsg
			}
		case reflect.Uint8:
			in := int(value.(uint8))
			if in > lenInt {
				return errMsg
			}
		case reflect.Uint16:
			in := int(value.(uint16))
			if in > lenInt {
				return errMsg
			}
		case reflect.Uint32:
			in := int(value.(uint32))
			if in > lenInt {
				return errMsg
			}
		case reflect.Uint64:
			in := int(value.(uint64))
			if in > lenInt {
				return errMsg
			}
		case reflect.Uintptr:
			in := int(value.(uintptr))
			if in > lenInt {
				return errMsg
			}
		case reflect.Float32:
			in := value.(float32)
			if in > float32(lenFloat) {
				return errMsgFloat
			}
		case reflect.Float64:
			in := value.(float64)
			if in > lenFloat {
				return errMsgFloat
			}

		}

		return nil
	})

	// Numeric check if the value of the field is Numeric
	AddCustomRule("numeric", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must be numeric", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isNumeric(str) {
			return err
		}
		return nil
	})

	// NumericBetween check if the value field numeric value range
	// e.g: numeric_between:18, 65 means number value must be in between a numeric value 18 & 65
	AddCustomRule("numeric_between", func(field string, rule string, message string, value interface{}) error {
		rng := strings.Split(strings.TrimPrefix(rule, "numeric_between:"), ",")
		if len(rng) != 2 {
			panic(errInvalidArgument)
		}
		// check for integer value
		_min, err := strconv.ParseFloat(rng[0], 64)
		if err != nil {
			panic(errStringToInt)
		}
		min := int(_min)
		_max, err := strconv.ParseFloat(rng[1], 64)
		if err != nil {
			panic(errStringToInt)
		}
		max := int(_max)
		errMsg := fmt.Errorf("The %s field must be numeric value between %d and %d", field, min, max)
		if message != "" {
			errMsg = errors.New(message)
		}

		val := toString(value)

		if !strings.Contains(rng[0], ".") || !strings.Contains(rng[1], ".") {
			digit, errs := strconv.Atoi(val)
			if errs != nil {
				return errMsg
			}
			if !(digit >= min && digit <= max) {
				return errMsg
			}
		}
		// check for float value
		minFloat, err := strconv.ParseFloat(rng[0], 64)
		if err != nil {
			panic(errStringToFloat)
		}
		maxFloat, err := strconv.ParseFloat(rng[1], 64)
		if err != nil {
			panic(errStringToFloat)
		}
		errMsg = fmt.Errorf("The %s field must be numeric value between %f and %f", field, minFloat, maxFloat)
		if message != "" {
			errMsg = errors.New(message)
		}

		digit, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return errMsg
		}
		if !(digit >= minFloat && digit <= maxFloat) {
			return errMsg
		}
		return nil
	})

	// ValidateURL check if provided field is valid URL
	AddCustomRule("url", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field format is invalid", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isURL(str) {
			return err
		}
		return nil
	})

	// UUID check if provided field contains valid UUID
	AddCustomRule("uuid", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must contain valid UUID", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isUUID(str) {
			return err
		}
		return nil
	})

	// UUID3 check if provided field contains valid UUID of version 3
	AddCustomRule("uuid_v3", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must contain valid UUID V3", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isUUID3(str) {
			return err
		}
		return nil
	})

	// UUID4 check if provided field contains valid UUID of version 4
	AddCustomRule("uuid_v4", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must contain valid UUID V4", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isUUID4(str) {
			return err
		}
		return nil
	})

	// UUID5 check if provided field contains valid UUID of version 5
	AddCustomRule("uuid_v5", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		err := fmt.Errorf("The %s field must contain valid UUID V5", field)
		if message != "" {
			err = errors.New(message)
		}
		if !isUUID5(str) {
			return err
		}
		return nil
	})

}
