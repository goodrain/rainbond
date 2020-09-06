package validator

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type fieldValidator struct {
	field   string
	value   string
	rule    string
	message string
	errsBag url.Values
}

// rmMap contains all the pre defined rules and their associate methods
var rmMap = map[string]string{
	"required":        "Required",
	"regex":           "Regex",
	"alpha":           "Alpha",
	"alpha_dash":      "AlphaDash",
	"alpha_num":       "AlphaNumeric",
	"bool":            "Boolean",
	"between":         "Between",
	"credit_card":     "CreditCard",
	"coordinate":      "Coordinate",
	"css_color":       "ValidateCSSColor",
	"digits":          "Digits",
	"digits_between":  "DigitsBetween",
	"date":            "Date",
	"email":           "Email",
	"float":           "ValidateFloat",
	"in":              "In",
	"ip":              "IP",
	"ip_v4":           "IPv4",
	"ip_v6":           "IPv6",
	"not_in":          "NotIn",
	"json":            "ValidateJSON",
	"len":             "Length",
	"lat":             "Latitude",
	"lon":             "Longitude",
	"min":             "Min",
	"max":             "Max",
	"numeric":         "Numeric",
	"numeric_between": "NumericBetween",
	"url":             "ValidateURL",
	"uuid":            "UUID",
	"uuid_v3":         "UUID3",
	"uuid_v4":         "UUID4",
	"uuid_v5":         "UUID5",
}

//run perform all the available field validation against a field and rule
func (v *fieldValidator) run() {
	v.field = strings.TrimSpace(v.field) //remove space form field
	v.rule = strings.TrimSpace(v.rule)   //remove space form rule

	rule := v.rule
	if strings.Contains(rule, ":") {
		rule = strings.Split(v.rule, ":")[0]
	}

	if r, ok := rmMap[rule]; ok {
		callMethodByName(v, r)
	}
}

// Required check the Required fields
func (v *fieldValidator) Required() {
	if v.value != "" {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field is required", v.field))
}

// Regex check the custom Regex rules
// Regex:^[a-zA-Z]+$ means this field can only contain alphabet (a-z and A-Z)
func (v *fieldValidator) Regex() {
	rxStr := strings.TrimPrefix(v.rule, "regex:")
	if IsMatchedRegex(rxStr, v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field format is invalid", v.field))
}

// Alpha check if provided field contains valid letters
func (v *fieldValidator) Alpha() {
	if IsAlpha(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s may only contain letters", v.field))
}

// AlphaDash check if provided field contains valid letters, numbers, underscore and dash
func (v *fieldValidator) AlphaDash() {
	if IsAlphaDash(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s may only contain letters, numbers, and dashes", v.field))
}

// AlphaNumeric check if provided field contains valid letters and numbers
func (v *fieldValidator) AlphaNumeric() {
	if IsAlphaNumeric(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s may only contain letters and numbers", v.field))
}

// Boolean check if provided field contains Boolean
// in this case: "0", "1", 0, 1, "true", "false", true, false etc
func (v *fieldValidator) Boolean() {
	if IsBoolean(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s may only contain 0, 1, true, false", v.field))
}

// CreditCard check if provided field contains valid credit card number
// Accepted cards are Visa, MasterCard, American Express, Diners Club, Discover and JCB card
func (v *fieldValidator) CreditCard() {
	if IsCreditCard(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid credit card number", v.field))
}

// Coordinate check if provided field contains valid Coordinate
func (v *fieldValidator) Coordinate() {
	if IsCoordinate(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid coordinate", v.field))
}

// ValidateCSSColor check if provided field contains a valid CSS color code
func (v *fieldValidator) ValidateCSSColor() {
	if IsCSSColor(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid CSS color code", v.field))
}

// ValidateJSON check if provided field contains valid json string
func (v *fieldValidator) ValidateJSON() {
	if IsJSON(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain valid JSON string", v.field))
}

// Length check the field's character Length
func (v *fieldValidator) Length() {
	l, err := strconv.Atoi(strings.TrimPrefix(v.rule, "len:"))
	if err != nil {
		panic(errStringToInt)
	}
	if len(v.value) == l {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be %d characters", v.field, l))
}

// Min check the field's minimum character length
func (v *fieldValidator) Min() {
	l, err := strconv.Atoi(strings.TrimPrefix(v.rule, "min:"))
	if err != nil {
		panic(errStringToInt)
	}
	if l <= len(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be minimum %d characters", v.field, l))
}

// Max check the field's maximum character length
func (v *fieldValidator) Max() {
	l, err := strconv.Atoi(strings.TrimPrefix(v.rule, "max:"))
	if err != nil {
		panic(errStringToInt)
	}
	if l >= len(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be maximum %d characters", v.field, l))
}

// Between check the fields character length range
func (v *fieldValidator) Between() {
	rng := strings.Split(strings.TrimPrefix(v.rule, "between:"), ",")
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
	if len(v.value) >= min && len(v.value) <= max {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be between %d and %d", v.field, min, max))
}

// Numeric check if the value of the field is Numeric
func (v *fieldValidator) Numeric() {
	if IsNumeric(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be numeric", v.field))
}

// NumericBetween check if the value field numeric value range
// e.g: numeric_between:18, 65 means number value must be in between a numeric value 18 & 65
func (v *fieldValidator) NumericBetween() {
	rng := strings.Split(strings.TrimPrefix(v.rule, "numeric_between:"), ",")
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
	digit, err := strconv.Atoi(v.value)
	if err != nil {
		if v.message != "" {
			v.errsBag.Add(v.field, v.message)
			return
		}
		v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be numeric value between %d and %d", v.field, min, max))
		return
	}
	if digit >= min && digit <= max {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be numeric value between %d and %d", v.field, min, max))
}

// Digits check the exact matching length of digit (0,9)
// Digits:5 means the field must have 5 digit of length.
// e.g: 12345 or 98997 etc
func (v *fieldValidator) Digits() {
	l, err := strconv.Atoi(strings.TrimPrefix(v.rule, "digits:"))
	if err != nil {
		panic(errStringToInt)
	}
	if len(v.value) == l && IsNumeric(v.value) {
		return
	}
	if !IsNumeric(v.value) || len(v.value) != l {
		if v.message != "" {
			v.errsBag.Add(v.field, v.message)
			return
		}
		v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be %d digits", v.field, l))
	}
}

// DigitsBetween check if the field contains only digit and length between provided range
// e.g: digits_between:4,5 means the field can have value like: 8887 or 12345 etc
func (v *fieldValidator) DigitsBetween() {
	rng := strings.Split(strings.TrimPrefix(v.rule, "digits_between:"), ",")
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
	if (len(v.value) >= min && len(v.value) <= max) && IsNumeric(v.value) {
		return
	}
	if !IsNumeric(v.value) || !(len(v.value) >= min && len(v.value) <= max) {
		if v.message != "" {
			v.errsBag.Add(v.field, v.message)
			return
		}
		v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be digits between %d and %d", v.field, min, max))
	}
}

// Email check the provided field is valid Email
func (v *fieldValidator) Email() {
	if IsEmail(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid email address", v.field))
}

// Date check the provided field is valid Date
func (v *fieldValidator) Date() {
	if v.rule == "date:dd-mm-yyyy" {
		if IsDateDDMMYY(v.value) {
			return
		}
		if v.message != "" {
			v.errsBag.Add(v.field, v.message)
			return
		}
		v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid date format. e.g: dd-mm-yyyy, dd/mm/yyyy etc", v.field))
		return
	}
	if IsDate(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid date format. e.g: yyyy-mm-dd, yyyy/mm/dd etc.", v.field))
}

// validFloat check the provided field is valid float number
func (v *fieldValidator) ValidateFloat() {
	if IsFloat(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a float number", v.field))
}

// Latitude check if provided field contains valid Latitude
func (v *fieldValidator) Latitude() {
	if IsLatitude(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain valid latitude", v.field))
}

// Longitude check if provided field contains valid Longitude
func (v *fieldValidator) Longitude() {
	if IsLongitude(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain valid longitude", v.field))
}

// In check if provided field's value exist In the provided rules
func (v *fieldValidator) In() {
	params := strings.TrimPrefix(v.rule, "in:")
	haystack := strings.Split(params, ",")
	if IsIn(haystack, v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain one of these values %s", v.field, params))
}

// NotIn check if provided field's value exist in the provided rules
func (v *fieldValidator) NotIn() {
	params := strings.TrimPrefix(v.rule, "not_in:")
	haystack := strings.Split(params, ",")
	if !IsIn(haystack, v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain values except %s", v.field, params))
}

// IP check if provided field is valid IP address
func (v *fieldValidator) IP() {
	if IsIP(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid IP address", v.field))
}

// IPv4 check if provided field is valid IP address of version 4
func (v *fieldValidator) IPv4() {
	if IsIPV4(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid IPV4 address", v.field))
}

// IPv6 check if provided field is valid IP address of version 6
func (v *fieldValidator) IPv6() {
	if IsIPV6(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must be a valid IPV6 address", v.field))
}

// ValidateURL check if provided field is valid URL
func (v *fieldValidator) ValidateURL() {
	if IsURL(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field format is invalid", v.field))
}

// UUID check if provided field contains valid UUID
func (v *fieldValidator) UUID() {
	if IsUUID(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain valid UUID", v.field))
}

// UUID3 check if provided field contains valid UUID of version 3
func (v *fieldValidator) UUID3() {
	if IsUUID3(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain valid UUID V3", v.field))
}

// UUID4 check if provided field contains valid UUID of version 4
func (v *fieldValidator) UUID4() {
	if IsUUID4(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain valid UUID V4", v.field))
}

// UUID5 check if provided field contains valid UUID of version 5
func (v *fieldValidator) UUID5() {
	if IsUUID5(v.value) {
		return
	}
	if v.message != "" {
		v.errsBag.Add(v.field, v.message)
		return
	}
	v.errsBag.Add(v.field, fmt.Sprintf("The %s field must contain valid UUID V5", v.field))
}
