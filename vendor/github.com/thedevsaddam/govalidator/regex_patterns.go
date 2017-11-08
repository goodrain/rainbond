package validator

import (
	"regexp"
)

const (
	// Alpha represents regular expression for alpha chartacters
	Alpha string = "^[a-zA-Z]+$"
	// AlphaDash represents regular expression for alpha chartacters with underscore and ash
	AlphaDash string = "^[a-zA-Z0-9_-]+$"
	// AlphaNumeric represents regular expression for alpha numeric chartacters
	AlphaNumeric string = "^[a-zA-Z0-9]+$"
	// CreditCard represents regular expression for credit cards like (Visa, MasterCard, American Express, Diners Club, Discover, and JCB cards). Ref: https://stackoverflow.com/questions/9315647/regex-credit-card-number-tests
	CreditCard string = "^(?:4[0-9]{12}(?:[0-9]{3})?|[25][1-7][0-9]{14}|6(?:011|5[0-9][0-9])[0-9]{12}|3[47][0-9]{13}|3(?:0[0-5]|[68][0-9])[0-9]{11}|(?:2131|1800|35\\d{3})\\d{11})$"
	// Coordinate represents latitude and longitude regular expression
	Coordinate string = "^[-+]?([1-8]?\\d(\\.\\d+)?|90(\\.0+)?),\\s*[-+]?(180(\\.0+)?|((1[0-7]\\d)|([1-9]?\\d))(\\.\\d+)?)$" // Ref: https://stackoverflow.com/questions/3518504/regular-expression-for-matching-latitude-longitude-coordinates
	// CSSColor represents css valid color code with hex, rgb, rgba, hsl, hsla etc. Ref: http://www.regexpal.com/97509
	CSSColor string = "^(#([\\da-f]{3}){1,2}|(rgb|hsl)a\\((\\d{1,3}%?,\\s?){3}(1|0?\\.\\d+)\\)|(rgb|hsl)\\(\\d{1,3}%?(,\\s?\\d{1,3}%?){2}\\))$"
	// Date represents regular expression for valid date like: yyyy-mm-dd
	Date string = "^(((19|20)([2468][048]|[13579][26]|0[48])|2000)[/-]02[/-]29|((19|20)[0-9]{2}[/-](0[4678]|1[02])[/-](0[1-9]|[12][0-9]|30)|(19|20)[0-9]{2}[/-](0[1359]|11)[/-](0[1-9]|[12][0-9]|3[01])|(19|20)[0-9]{2}[/-]02[/-](0[1-9]|1[0-9]|2[0-8])))$"
	// DateDDMMYY represents regular expression for valid date of format dd/mm/yyyy , dd-mm-yyyy etc.Ref: http://regexr.com/346hf
	DateDDMMYY string = "^(0?[1-9]|[12][0-9]|3[01])[\\/\\-](0?[1-9]|1[012])[\\/\\-]\\d{4}$"
	// Email represents regular expression for email
	Email string = "^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$"
	// Float represents regular expression for finding fload number
	Float string = "^[+-]?([0-9]*[.])?[0-9]+$"
	// IP represents regular expression for ip address
	IP string = "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
	// IPV4 represents regular expression for ip address version 4
	IPV4 string = "^([0-9]{1,3}\\.){3}[0-9]{1,3}(\\/([0-9]|[1-2][0-9]|3[0-2]))?$"
	// IPV6 represents regular expression for ip address version 6
	IPV6 string = `^s*((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]d|1dd|[1-9]?d)(.(25[0-5]|2[0-4]d|1dd|[1-9]?d)){3}))|:)))(%.+)?s*(\/([0-9]|[1-9][0-9]|1[0-1][0-9]|12[0-8]))?$`
	// Latitude represents latitude regular expression
	Latitude string = "^(\\+|-)?(?:90(?:(?:\\.0{1,6})?)|(?:[0-9]|[1-8][0-9])(?:(?:\\.[0-9]{1,6})?))$"
	// Longitude represents longitude regular expression
	Longitude string = "^(\\+|-)?(?:180(?:(?:\\.0{1,6})?)|(?:[0-9]|[1-9][0-9]|1[0-7][0-9])(?:(?:\\.[0-9]{1,6})?))$"
	// Numeric represents regular expression for numeric
	Numeric string = "^[0-9]+$"
	// URL represents regular expression for url
	URL string = "^(?:http(s)?:\\/\\/)?[\\w.-]+(?:\\.[\\w\\.-]+)+[\\w\\-\\._~:/?#[\\]@!\\$&'\\(\\)\\*\\+,;=.]+$" // Ref: https://stackoverflow.com/questions/136505/searching-for-uuids-in-text-with-regex
	// UUID represents regular expression for UUID
	UUID string = "^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89aAbB][a-f0-9]{3}-[a-f0-9]{12}$"
	// UUID3 represents regular expression for UUID version 3
	UUID3 string = "^[0-9a-f]{8}-[0-9a-f]{4}-3[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$"
	// UUID4 represents regular expression for UUID version 4
	UUID4 string = "^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"
	// UUID5 represents regular expression for UUID version 5
	UUID5 string = "^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"
)

var (
	regexAlpha        = regexp.MustCompile(Alpha)
	regexAlphaDash    = regexp.MustCompile(AlphaDash)
	regexAlphaNumeric = regexp.MustCompile(AlphaNumeric)
	regexCreditCard   = regexp.MustCompile(CreditCard)
	regexCoordinate   = regexp.MustCompile(Coordinate)
	regexCSSColor     = regexp.MustCompile(CSSColor)
	regexDate         = regexp.MustCompile(Date)
	regexDateDDMMYY   = regexp.MustCompile(DateDDMMYY)
	regexEmail        = regexp.MustCompile(Email)
	regexFloat        = regexp.MustCompile(Float)
	regexNumeric      = regexp.MustCompile(Numeric)
	regexLatitude     = regexp.MustCompile(Latitude)
	regexLongitude    = regexp.MustCompile(Longitude)
	regexIP           = regexp.MustCompile(IP)
	regexIPV4         = regexp.MustCompile(IPV4)
	regexIPV6         = regexp.MustCompile(IPV6)
	regexURL          = regexp.MustCompile(URL)
	regexUUID         = regexp.MustCompile(UUID)
	regexUUID3        = regexp.MustCompile(UUID3)
	regexUUID4        = regexp.MustCompile(UUID4)
	regexUUID5        = regexp.MustCompile(UUID5)
)
