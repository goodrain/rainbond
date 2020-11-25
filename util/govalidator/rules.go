package validator

import (
	"encoding/json"
	"regexp"
)

// IsAlpha check the input is letters (a-z,A-Z) or not
func IsAlpha(str string) bool {
	return regexAlpha.MatchString(str)
}

// IsAlphaDash check the input is letters, number with dash and underscore
func IsAlphaDash(str string) bool {
	return regexAlphaDash.MatchString(str)
}

// IsAlphaNumeric check the input is alpha numeric or not
func IsAlphaNumeric(str string) bool {
	return regexAlphaNumeric.MatchString(str)
}

// IsBoolean check the input contains boolean type values
// in this case: "0", "1", "true", "false", "True", "False"
func IsBoolean(str string) bool {
	bools := []string{"0", "1", "true", "false", "True", "False"}
	for _, b := range bools {
		if b == str {
			return true
		}
	}
	return false
}

//IsCreditCard check the provided card number is a valid
//  Visa, MasterCard, American Express, Diners Club, Discover or JCB card
func IsCreditCard(card string) bool {
	return regexCreditCard.MatchString(card)
}

// IsCoordinate is a valid Coordinate or not
func IsCoordinate(str string) bool {
	return regexCoordinate.MatchString(str)
}

// IsCSSColor is a valid CSS color value (hex, rgb, rgba, hsl, hsla) etc like #909, #00aaff, rgb(255,122,122)
func IsCSSColor(str string) bool {
	return regexCSSColor.MatchString(str)
}

// IsDate check the date string is valid or not
func IsDate(date string) bool {
	return regexDate.MatchString(date)
}

// IsDateDDMMYY check the date string is valid or not
func IsDateDDMMYY(date string) bool {
	return regexDateDDMMYY.MatchString(date)
}

// IsEmail check a email is valid or not
func IsEmail(email string) bool {
	return regexEmail.MatchString(email)
}

// IsFloat check the input string is a float or not
func IsFloat(str string) bool {
	return regexFloat.MatchString(str)
}

// IsIn check if the niddle exist in the haystack
func IsIn(haystack []string, niddle string) bool {
	for _, h := range haystack {
		if h == niddle {
			return true
		}
	}
	return false
}

// IsJSON check wheather the input string is a valid json or not
func IsJSON(str string) bool {
	var data interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		return false
	}
	return true
}

// IsNumeric check the provided input string is numeric or not
func IsNumeric(str string) bool {
	return regexNumeric.MatchString(str)
}

// IsLatitude check the provided input string is a valid latitude or not
func IsLatitude(str string) bool {
	return regexLatitude.MatchString(str)
}

// IsLongitude check the provided input string is a valid longitude or not
func IsLongitude(str string) bool {
	return regexLongitude.MatchString(str)
}

// IsIP check the provided input string is a valid IP address or not
func IsIP(str string) bool {
	return regexIP.MatchString(str)
}

// IsIPV4 check the provided input string is a valid IP address version 4 or not
// Ref: https://en.wikipedia.org/wiki/IPv4
func IsIPV4(str string) bool {
	return regexIPV4.MatchString(str)
}

// IsIPV6 check the provided input string is a valid IP address version 6 or not
// Ref: https://en.wikipedia.org/wiki/IPv6
func IsIPV6(str string) bool {
	return regexIPV6.MatchString(str)
}

// IsMatchedRegex match the regular expression string provided in first argument
// with second argument which is also a string
func IsMatchedRegex(rxStr, str string) bool {
	rx := regexp.MustCompile(rxStr)
	return rx.MatchString(str)
}

// IsURL check a URL is valid or not
func IsURL(url string) bool {
	return regexURL.MatchString(url)
}

// IsUUID check the provided string is valid UUID or not
func IsUUID(str string) bool {
	return regexUUID.MatchString(str)
}

// IsUUID3 check the provided string is valid UUID version 3 or not
func IsUUID3(str string) bool {
	return regexUUID3.MatchString(str)
}

// IsUUID4 check the provided string is valid UUID version 4 or not
func IsUUID4(str string) bool {
	return regexUUID4.MatchString(str)
}

// IsUUID5 check the provided string is valid UUID version 5 or not
func IsUUID5(str string) bool {
	return regexUUID5.MatchString(str)
}
