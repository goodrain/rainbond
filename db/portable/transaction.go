package portable

import (
	"errors"

	"github.com/jinzhu/gorm"
)

// WithinTransaction executes callback in a database transaction. SQLite keeps
// the legacy direct-execution behavior because this call path can otherwise
// contend with operations that use the shared database handle.
func WithinTransaction(db *gorm.DB, callback func(*gorm.DB) error) error {
	if db == nil {
		return errors.New("database is required")
	}
	if callback == nil {
		return errors.New("transaction callback is required")
	}
	if db.Dialect().GetName() == "sqlite3" {
		return callback(db)
	}
	return db.Transaction(callback)
}
