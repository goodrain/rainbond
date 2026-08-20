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

// SchemaName returns the explicit schema selected by a Rainbond Dameng DSN.
// Rainbond uses separate schemas for Region and Console, so falling back to a
// database user's default schema can silently verify or migrate the wrong one.
func SchemaName(connectionInfo string) (string, error) {
	normalized, err := NormalizeDSN(connectionInfo)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", errInvalidDSN
	}
	schema := parsed.Query().Get("schema")
	if !validSchema(schema) {
		return "", errInvalidDSN
	}
	return strings.ToUpper(schema), nil
}

func normalizeNativeDSN(connectionInfo string) (string, error) {
	parsed, err := url.Parse(connectionInfo)
	if err != nil || !strings.EqualFold(parsed.Scheme, "dm") || !validHostPort(parsed.Host) {
		return "", errInvalidDSN
	}

	schema := strings.TrimPrefix(parsed.Path, "/")
	if schema == "" {
		return connectionInfo, nil
	}
	if !validSchema(schema) {
		return "", errInvalidDSN
	}

	query := parsed.Query()
	if query.Get("schema") == "" {
		query.Set("schema", strings.ToUpper(schema))
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
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
	// The official DM Go driver only reads schema from a URL query parameter;
	// its URL path is ignored. Normalize legacy database names to a schema query.
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
		RawQuery: url.Values{
			"schema": []string{schema},
		}.Encode(),
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
