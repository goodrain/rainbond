package portable

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var databaseInspectionPattern = regexp.MustCompile(`(?m)(\.Dialect\(\)\.GetName\(\)|DBType\s*(?:==|!=))`)

// capability_id: region.database.backend-boundary
func TestBusinessPackagesDoNotInspectDatabaseDialect(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, relativeRoot := range []string{"api", "builder", "db/mysql/dao", "worker"} {
		root := filepath.Join(repoRoot, relativeRoot)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if databaseInspectionPattern.Match(source) {
				relativePath, _ := filepath.Rel(repoRoot, path)
				t.Errorf("%s inspects the concrete database outside the backend boundary", relativePath)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
