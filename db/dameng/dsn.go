package dameng

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var errInvalidDSN = errors.New("invalid Dameng DSN")

// NormalizeDSN accepts a native Dameng URL or converts the legacy MySQL DSN
// format supplied by the current operator into a native Dameng URL.
func NormalizeDSN(connectionInfo string) (string, error) {
	if strings.HasPrefix(strings.ToLower(connectionInfo), "dm://") {
		return normalizeNativeDSN(connectionInfo)
	}
	return normalizeLegacyDSN(connectionInfo)
}

func normalizeNativeDSN(connectionInfo string) (string, error) {
	parsed, err := url.Parse(connectionInfo)
	if err != nil || !strings.EqualFold(parsed.Scheme, "dm") || !validHostPort(parsed.Host) {
		return "", errInvalidDSN
	}
	return connectionInfo, nil
}

func normalizeLegacyDSN(connectionInfo string) (string, error) {
	const tcpPrefix = "@tcp("

	tcpIndex := strings.LastIndex(connectionInfo, tcpPrefix)
	if tcpIndex <= 0 {
		return "", errInvalidDSN
	}

	credentials := connectionInfo[:tcpIndex]
	endpointAndSchema := connectionInfo[tcpIndex+len(tcpPrefix):]
	endpointEnd := strings.Index(endpointAndSchema, ")/")
	if endpointEnd <= 0 {
		return "", errInvalidDSN
	}

	hostPort := endpointAndSchema[:endpointEnd]
	schema := endpointAndSchema[endpointEnd+2:]
	if !validHostPort(hostPort) || !validSchema(schema) {
		return "", errInvalidDSN
	}
	// The official DM Go driver quotes the schema when opening a connection.
	// Normalize legacy MySQL-style database names to DM's conventional uppercase
	// identifiers so a typical /region DSN selects REGION rather than "region".
	schema = strings.ToUpper(schema)

	credentialSeparator := strings.IndexByte(credentials, ':')
	if credentialSeparator <= 0 {
		return "", errInvalidDSN
	}
	username := credentials[:credentialSeparator]
	password := credentials[credentialSeparator+1:]
	if username == "" {
		return "", errInvalidDSN
	}

	return (&url.URL{
		Scheme: "dm",
		User:   url.UserPassword(username, password),
		Host:   hostPort,
		Path:   "/" + schema,
	}).String(), nil
}

func validHostPort(hostPort string) bool {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil || host == "" {
		return false
	}

	portNumber, err := strconv.Atoi(port)
	return err == nil && portNumber > 0 && portNumber <= 65535
}

func validSchema(schema string) bool {
	return schema != "" && schema == strings.TrimSpace(schema) && !strings.ContainsAny(schema, "/?#")
}
