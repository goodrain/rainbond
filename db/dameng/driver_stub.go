//go:build !dm

package dameng

// DriverBuilt reports whether this binary includes the official Dameng driver
// and the matching GORM v1 dialect.
func DriverBuilt() bool {
	return false
}
