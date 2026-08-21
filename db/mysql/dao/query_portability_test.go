package dao

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
)

var vendorSpecificRuntimeSQL = regexp.MustCompile(
	`(?i)(GROUP_CONCAT|FIND_IN_SET|ON\s+DUPLICATE\s+KEY|INSERT\s+IGNORE|REPLACE\s+INTO|` +
		`AS\s+(SIGNED|UNSIGNED)|COLLATE\s+UTF8|LIMIT\s+[^,;\n]+,\s*[^;\n]+|` + "`[^`]+`" + `)`,
)

func TestDAORuntimeSQLDoesNotUseVendorSpecificSyntax(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if relativePath == ".git" || relativePath == "vendor" || relativePath == "third_party" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
			relativePath == filepath.Join("db", "mysql", "mysql.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relativePath, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			upper := strings.ToUpper(value)
			isSQL := (strings.Contains(upper, "SELECT") && strings.Contains(upper, "FROM")) ||
				(strings.Contains(upper, "UPDATE") && strings.Contains(upper, "SET")) ||
				(strings.Contains(upper, "DELETE") && strings.Contains(upper, "FROM")) ||
				(strings.Contains(upper, "INSERT") && strings.Contains(upper, "INTO"))
			if isSQL && vendorSpecificRuntimeSQL.MatchString(value) {
				position := fileSet.Position(literal.Pos())
				t.Errorf("%s:%d contains vendor-specific runtime SQL", relativePath, position.Line)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// capability_id: region.database.portable-pagination
func TestGetEventsByTenantIDsUsesORMPagination(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatal(err)
	}

	query := regexp.QuoteMeta("FROM tenant_services_event AS a LEFT JOIN tenant_service_version AS b ON a.target_id = b.service_id AND a.event_id = b.event_id WHERE (a.target = ?) AND (a.tenant_id IN (?,?)) ORDER BY a.ID DESC LIMIT 2 OFFSET 1")
	mock.ExpectQuery(query).
		WithArgs("service", "tenant-1", "tenant-2").
		WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(1))

	dao := &EventDaoImpl{DB: db}
	if _, err := dao.GetEventsByTenantIDs([]string{"tenant-1", "tenant-2"}, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetPagedTenantServiceUsesORMPagination(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT tenant_id FROM `tenant_services` WHERE (service_id in (?)) GROUP BY tenant_id")).
		WithArgs("service-1").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id"}).AddRow("tenant-1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT tenant_id, SUM(container_cpu * replicas) AS use_cpu, SUM(container_memory * replicas) AS use_memory FROM `tenant_services` WHERE (service_id in (?)) GROUP BY tenant_id ORDER BY use_memory DESC LIMIT 1 OFFSET 0")).
		WithArgs("service-1").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "use_cpu", "use_memory"}).AddRow("tenant-1", 100, 256))
	mock.ExpectQuery("SELECT tenant_id, SUM").
		WithArgs("tenant-1").
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "cap_cpu", "cap_memory"}).AddRow("tenant-1", 100, 256))
	mock.ExpectQuery("SELECT uuid,name,eid").
		WithArgs("tenant-1").
		WillReturnRows(sqlmock.NewRows([]string{"uuid", "name", "eid"}).AddRow("tenant-1", "Tenant", "enterprise"))

	dao := &TenantServicesDaoImpl{DB: db}
	result, count, err := dao.GetPagedTenantService(0, 1, []string{"service-1"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(result) != 1 {
		t.Fatalf("expected one tenant result, got count=%d len=%d", count, len(result))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
