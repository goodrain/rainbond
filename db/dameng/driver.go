package dameng

import "errors"

// ErrDriverNotBuilt indicates that DB_TYPE=dm was requested from an image
// that was not built with the proprietary Dameng driver bundle.
var ErrDriverNotBuilt = errors.New("Dameng database support is not included in this image; rebuild with ENABLE_DM=true")
