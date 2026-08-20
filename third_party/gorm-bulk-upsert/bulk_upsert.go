// Modified from github.com/goodrain/gorm-bulk-upsert.
//
// The original MySQL bulk-upsert implementation is retained below. Dameng
// support adds a dialect-specific row-by-row path because Dameng does not
// implement MySQL's ON DUPLICATE KEY UPDATE syntax.
package gormbulkups

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

// BulkUpsert writes a batch of structs.  MySQL retains the upstream bulk
// INSERT ... ON DUPLICATE KEY UPDATE statement. Dameng uses its existing
// single-row create/update semantics because that MySQL-only statement is not
// valid Dameng SQL.
func BulkUpsert(db *gorm.DB, objects []interface{}, chunkSize int, excludeColumns ...string) error {
	if usesDamengRowByRowUpsert(db.Dialect().GetName()) {
		return damengUpsertObjectSet(db, objects, chunkSize, excludeColumns...)
	}

	for _, objSet := range splitObjects(objects, chunkSize) {
		if err := upsertObjSet(db, objSet, excludeColumns...); err != nil {
			return err
		}
	}
	return nil
}

func usesDamengRowByRowUpsert(dialect string) bool {
	return dialect == "dm"
}

func damengUpsertObjectSet(db *gorm.DB, objects []interface{}, chunkSize int, excludeColumns ...string) error {
	for _, objSet := range splitObjects(objects, chunkSize) {
		tx, ownsTransaction, err := damengUpsertTransaction(db)
		if err != nil {
			return err
		}

		for _, obj := range objSet {
			if _, err := extractMapValue(obj, excludeColumns); err != nil {
				if ownsTransaction {
					tx.Rollback()
				}
				return err
			}
			if err := damengUpsertObject(tx, obj); err != nil {
				if ownsTransaction {
					tx.Rollback()
				}
				return err
			}
		}

		if ownsTransaction {
			if err := tx.Commit().Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// damengUpsertTransaction returns the caller transaction unchanged when one
// was supplied. A DAO may be part of a larger service transaction, and GORM
// v1 cannot begin a transaction from an existing *sql.Tx.
func damengUpsertTransaction(db *gorm.DB) (*gorm.DB, bool, error) {
	if _, ok := db.CommonDB().(*sql.Tx); ok {
		return db, false, nil
	}

	tx := db.Begin()
	if err := tx.Error; err != nil {
		return nil, false, err
	}
	return tx, true, nil
}

func damengUpsertObject(db *gorm.DB, object interface{}) error {
	scope := db.NewScope(object)
	primaryField := scope.PrimaryField()
	if primaryField == nil {
		return errors.New("Dameng upsert requires a primary key")
	}
	if scope.PrimaryKeyZero() {
		return db.Create(object).Error
	}

	primaryKey := primaryField.DBName
	primaryValue := scope.PrimaryKeyValue()
	existing := reflect.New(reflect.TypeOf(object)).Interface()
	query := db.Where(fmt.Sprintf("%s = ?", primaryKey), primaryValue).First(existing)
	if query.RecordNotFound() {
		return db.Create(object).Error
	}
	if query.Error != nil {
		return query.Error
	}
	return db.Model(object).Where(fmt.Sprintf("%s = ?", primaryKey), primaryValue).Update(object).Error
}

func upsertObjSet(db *gorm.DB, objects []interface{}, excludeColumns ...string) error {
	if len(objects) == 0 {
		return nil
	}

	firstAttrs, err := extractMapValue(objects[0], excludeColumns)
	if err != nil {
		return err
	}

	attrSize := len(firstAttrs)
	mainScope := db.NewScope(objects[0])
	placeholders := make([]string, 0, attrSize)

	dbColumns := make([]string, 0, attrSize)
	for _, key := range sortedKeys(firstAttrs) {
		dbColumns = append(dbColumns, gorm.ToColumnName(key))
	}

	duplicates := make([]string, 0)
	for _, field := range mainScope.Fields() {
		_, hasForeignKey := field.TagSettingsGet("FOREIGNKEY")
		_, isUnique := field.TagSettingsGet("UNIQUE")
		_, hasUniqueIndex := field.TagSettingsGet("UNIQUE_INDEX")
		if containString(excludeColumns, field.Struct.Name) ||
			field.StructField.Relationship != nil ||
			hasForeignKey ||
			field.IsIgnored ||
			field.IsPrimaryKey ||
			isUnique ||
			hasUniqueIndex {
			continue
		}

		duplicates = append(duplicates, fmt.Sprintf("`%s`=VALUES(`%s`)", field.DBName, field.DBName))
	}

	for _, obj := range objects {
		objAttrs, err := extractMapValue(obj, excludeColumns)
		if err != nil {
			return err
		}
		if len(objAttrs) != attrSize {
			return errors.New("attribute sizes are inconsistent")
		}

		scope := db.NewScope(obj)
		variables := make([]string, 0, attrSize)
		for _, key := range sortedKeys(objAttrs) {
			scope.AddToVars(objAttrs[key])
			variables = append(variables, "?")
		}
		placeholders = append(placeholders, "("+strings.Join(variables, ", ")+")")
		mainScope.SQLVars = append(mainScope.SQLVars, scope.SQLVars...)
	}

	sql := "INSERT INTO %s (`%s`) VALUES %s"
	args := []interface{}{
		mainScope.QuotedTableName(),
		strings.Join(dbColumns, "`, `"),
		strings.Join(placeholders, ", "),
	}
	if len(duplicates) > 0 {
		sql += " ON DUPLICATE KEY UPDATE %s"
		args = append(args, strings.Join(duplicates, ", "))
	}
	mainScope.Raw(fmt.Sprintf(sql, args...))

	return db.Exec(mainScope.SQL, mainScope.SQLVars...).Error
}

func extractMapValue(value interface{}, excludeColumns []string) (map[string]interface{}, error) {
	if reflect.ValueOf(value).Kind() != reflect.Struct {
		return nil, errors.New("value must be kind of Struct")
	}

	attrs := map[string]interface{}{}
	for _, field := range (&gorm.Scope{Value: value}).Fields() {
		_, hasForeignKey := field.TagSettingsGet("FOREIGNKEY")
		if containString(excludeColumns, field.Struct.Name) ||
			field.StructField.Relationship != nil ||
			hasForeignKey ||
			field.IsIgnored {
			continue
		}
		if field.Struct.Name == "CreatedAt" || field.Struct.Name == "UpdatedAt" {
			attrs[field.DBName] = time.Now()
		} else if field.StructField.HasDefaultValue && field.IsBlank {
			if value, ok := field.TagSettingsGet("DEFAULT"); ok {
				attrs[field.DBName] = value
			} else {
				attrs[field.DBName] = field.Field.Interface()
			}
		} else {
			attrs[field.DBName] = field.Field.Interface()
		}
	}
	return attrs, nil
}
