package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var vendorSpecificModelType = regexp.MustCompile(`(?i)type:(longtext|mediumtext|tinytext|clob|long\s+varchar)\b`)

// capability_id: region.database.portable-model-types
func TestModelTagsDoNotUseVendorSpecificTextTypes(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(".", entry.Name())
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			field, ok := node.(*ast.Field)
			if !ok || field.Tag == nil {
				return true
			}

			tag := strings.Trim(field.Tag.Value, "`")
			if vendorSpecificModelType.MatchString(tag) {
				position := fileSet.Position(field.Pos())
				t.Errorf("%s:%d uses a vendor-specific text type: %s", path, position.Line, tag)
			}
			return true
		})
	}
}
