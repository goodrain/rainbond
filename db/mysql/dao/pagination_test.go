package dao

import "testing"

// capability_id: rainbond.database.dameng-query-capabilities
func TestPaginationSQLKeepsMySQLAndUsesDamengOffsetSyntax(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		want    string
	}{
		{name: "mysql", dialect: "mysql", want: " LIMIT ?,?"},
		{name: "dameng", dialect: "dm", want: " LIMIT ? OFFSET ?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paginationSQL(tt.dialect); got != tt.want {
				t.Fatalf("paginationSQL(%q) = %q, want %q", tt.dialect, got, tt.want)
			}
		})
	}
}
