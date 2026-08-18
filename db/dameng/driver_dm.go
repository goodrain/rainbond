//go:build dm

package dameng

// The dm module is prepared from the official Dameng driver bundle only in
// private DM image builds. It registers both database/sql's dm driver and the
// GORM v1 dm dialect through package initialization.
import _ "dm"

// DriverBuilt reports whether this binary includes the official Dameng driver
// and the matching GORM v1 dialect.
func DriverBuilt() bool {
	return true
}
