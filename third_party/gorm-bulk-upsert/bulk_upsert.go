// Modified from github.com/goodrain/gorm-bulk-upsert.
// This package is the optimized MySQL implementation used by db/portable.
package gormbulkups

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

// BulkUpsert writes a batch of structs with MySQL's
// INSERT ... ON DUPLICATE KEY UPDATE statement.
func BulkUpsert(db *gorm.DB, objects []interface{}, chunkSize int, excludeColumns ...string) error {
	for _, objSet := range splitObjects(objects, chunkSize) {
		if err := upsertObjSet(db, objSet, excludeColumns...); err != nil {
			return err
		}
	}
	return nil
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
