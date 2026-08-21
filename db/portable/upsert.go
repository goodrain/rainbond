package portable

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	"github.com/jinzhu/gorm"
)

// BulkUpsert writes rows using conflict columns declared by the DAO. MySQL
// keeps its existing bulk implementation; other databases use portable
// update-then-insert semantics on the caller's DB or transaction handle.
func BulkUpsert(db *gorm.DB, rows []interface{}, batchSize int, conflictColumns ...string) error {
	if db == nil {
		return errors.New("database is required")
	}
	if batchSize <= 0 {
		return errors.New("batch size must be greater than zero")
	}
	if len(conflictColumns) == 0 {
		return errors.New("at least one conflict column is required")
	}
	if len(rows) == 0 {
		return nil
	}

	if db.Dialect().GetName() == "mysql" {
		return gormbulkups.BulkUpsert(db, rows, batchSize)
	}

	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		for _, row := range rows[start:end] {
			if err := upsertRow(db, row, conflictColumns); err != nil {
				return err
			}
		}
	}
	return nil
}

func upsertRow(db *gorm.DB, row interface{}, conflictColumns []string) error {
	value, err := addressableValue(row)
	if err != nil {
		return err
	}

	conditions, updates, err := splitUpsertAttributes(value, conflictColumns)
	if err != nil {
		return err
	}

	existing := reflect.New(reflect.TypeOf(value).Elem()).Interface()
	query := db.Where(conditions).First(existing)
	if query.Error == nil {
		return db.Model(existing).Updates(updates).Error
	}
	if !query.RecordNotFound() {
		return query.Error
	}

	if createErr := db.Create(value).Error; createErr != nil {
		// Another writer may have inserted the same logical row between the
		// lookup and insert. Retry the lookup and update without creating a nested
		// transaction; the caller still owns commit and rollback.
		retryValue := reflect.New(reflect.TypeOf(value).Elem()).Interface()
		retryQuery := db.Where(conditions).First(retryValue)
		if retryQuery.Error == nil {
			return db.Model(retryValue).Updates(updates).Error
		}
		return createErr
	}
	return nil
}

func addressableValue(row interface{}) (interface{}, error) {
	if row == nil {
		return nil, errors.New("upsert row is nil")
	}

	value := reflect.ValueOf(row)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() || value.Elem().Kind() != reflect.Struct {
			return nil, fmt.Errorf("upsert row must point to a struct, got %T", row)
		}
		return row, nil
	}
	if value.Kind() != reflect.Struct {
		return nil, fmt.Errorf("upsert row must be a struct or struct pointer, got %T", row)
	}

	pointer := reflect.New(value.Type())
	pointer.Elem().Set(value)
	return pointer.Interface(), nil
}

func splitUpsertAttributes(row interface{}, conflictColumns []string) (map[string]interface{}, map[string]interface{}, error) {
	conflictSet := make(map[string]struct{}, len(conflictColumns))
	for _, column := range conflictColumns {
		column = strings.TrimSpace(column)
		if column == "" {
			return nil, nil, errors.New("conflict column must not be empty")
		}
		conflictSet[strings.ToLower(column)] = struct{}{}
	}

	conditions := make(map[string]interface{}, len(conflictColumns))
	updates := make(map[string]interface{})
	for _, field := range (&gorm.Scope{Value: row}).Fields() {
		_, hasForeignKey := field.TagSettingsGet("FOREIGNKEY")
		if field.StructField.Relationship != nil || hasForeignKey || field.IsIgnored {
			continue
		}

		column := strings.ToLower(field.DBName)
		fieldValue := field.Field.Interface()
		if field.Struct.Name == "CreatedAt" {
			continue
		}
		if field.Struct.Name == "UpdatedAt" {
			fieldValue = time.Now()
		}
		if _, ok := conflictSet[column]; ok {
			conditions[field.DBName] = fieldValue
			continue
		}
		if !field.IsPrimaryKey {
			updates[field.DBName] = fieldValue
		}
	}

	if len(conditions) != len(conflictSet) {
		missing := make([]string, 0, len(conflictSet)-len(conditions))
		for column := range conflictSet {
			found := false
			for condition := range conditions {
				if strings.EqualFold(condition, column) {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, column)
			}
		}
		return nil, nil, fmt.Errorf("conflict columns not found in upsert row: %s", strings.Join(missing, ", "))
	}
	return conditions, updates, nil
}
