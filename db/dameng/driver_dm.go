//go:build dm

package dameng

// The official driver and its GORM v1 dialect are separate local modules. The
// dialect imports the driver and registers both during package initialization.
import _ "github.com/goodrain/dameng-gorm-dialect"

// DriverBuilt reports whether this binary includes the official Dameng driver
// and the matching GORM v1 dialect.
func DriverBuilt() bool {
	return true
}
