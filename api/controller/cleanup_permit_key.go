package controller

import (
	"io"
	"os"
	"strings"
)

// An explicitly configured key must fail closed; never silently substitute the
// administrator token when the dedicated key is missing or invalid.
func systemRegistryPermitKey() []byte {
	path := os.Getenv("CLEANUP_REGISTRY_PERMIT_KEY_FILE")
	if path == "" {
		return []byte(os.Getenv("TOKEN"))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(raw) > 8192 {
		return nil
	}
	value := strings.TrimSpace(string(raw))
	if len(value) < 32 || strings.ContainsAny(value, " \t\r\n\x00") {
		return nil
	}
	return []byte(value)
}
