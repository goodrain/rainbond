package govalidator

import (
	"encoding/json"
	"regexp"
)

// isAlpha check the input is letters (a-z,A-Z) or not
func isAlpha(str string) bool {
	return regexAlpha.MatchString(str)
}

// isAlphaDash check the input is letters, number with dash and underscore
func isAlphaDash(str string) bool {
	return regexAlphaDash.MatchString(str)
}

// isAlphaNumeric check the input is alpha numeric or not
func isAlphaNumeric(str string) bool {
	return regexAlphaNumeric.MatchString(str)
}

// isBoolean check the input contains boolean type values
// in this case: "0", "1", "true", "false", "True", "False"
func isBoolean(str string) bool {
	bools := []string{"0", "1", "true", "false", "True", "False"}
	for _, b := range bools {
		if b == str {
			return true
		}
	}
	return false
}

//isCreditCard check the provided card number is a valid
//  Visa, MasterCard, American Express, Diners Club, Discover or JCB card
func isCreditCard(card string) bool {
	return regexCreditCard.MatchString(card)
}

// isCoordinate is a valid Coordinate or not
func isCoordinate(str string) bool {
	return regexCoordinate.MatchString(str)
}

// isCSSColor is a valid CSS color value (hex, rgb, rgba, hsl, hsla) etc like #909, #00aaff, rgb(255,122,122)
func isCSSColor(str string) bool {
	return regexCSSColor.MatchString(str)
}

// isDate check the date string is valid or not
func isDate(date string) bool {
	return regexDate.MatchString(date)
}

// isDateDDMMYY check the date string is valid or not
func isDateDDMMYY(date string) bool {
	return regexDateDDMMYY.MatchString(date)
}

// isEmail check a email is valid or not
func isEmail(email string) bool {
	return regexEmail.MatchString(email)
}

// isFloat check the input string is a float or not
func isFloat(str string) bool {
	return regexFloat.MatchString(str)
}

// isIn check if the niddle exist in the haystack
func isIn(haystack []string, niddle string) bool {
	for _, h := range haystack {
		if h == niddle {
			return true
		}
	}
	return false
}

// isJSON check wheather the input string is a valid json or not
func isJSON(str string) bool {
	var data interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		return false
	}
	return true
}

// isNumeric check the provided input string is numeric or not
func isNumeric(str string) bool {
	return regexNumeric.MatchString(str)
}

// isLatitude check the provided input string is a valid latitude or not
func isLatitude(str string) bool {
	return regexLatitude.MatchString(str)
}

// isLongitude check the provided input string is a valid longitude or not
func isLongitude(str string) bool {
	return regexLongitude.MatchString(str)
}

// isIP check the provided input string is a valid IP address or not
func isIP(str string) bool {
	return regexIP.MatchString(str)
}

// isIPV4 check the provided input string is a valid IP address version 4 or not
// Ref: https://en.wikipedia.org/wiki/IPv4
func isIPV4(str string) bool {
	return regexIPV4.MatchString(str)
}

// isIPV6 check the provided input string is a valid IP address version 6 or not
// Ref: https://en.wikipedia.org/wiki/IPv6
func isIPV6(str string) bool {
	return regexIPV6.MatchString(str)
}

// isMatchedRegex match the regular expression string provided in first argument
// with second argument which is also a string
func isMatchedRegex(rxStr, str string) bool {
	rx := regexp.MustCompile(rxStr)
	return rx.MatchString(str)
}

// isURL check a URL is valid or not
func isURL(url string) bool {
	return regexURL.MatchString(url)
}

// isUUID check the provided string is valid UUID or not
func isUUID(str string) bool {
	return regexUUID.MatchString(str)
}

// isUUID3 check the provided string is valid UUID version 3 or not
func isUUID3(str string) bool {
	return regexUUID3.MatchString(str)
}

// isUUID4 check the provided string is valid UUID version 4 or not
func isUUID4(str string) bool {
	return regexUUID4.MatchString(str)
}

// isUUID5 check the provided string is valid UUID version 5 or not
func isUUID5(str string) bool {
	return regexUUID5.MatchString(str)
}
