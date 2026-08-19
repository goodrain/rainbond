package dao

// paginationSQL returns a bound pagination clause for the active database
// dialect. MySQL interprets LIMIT arguments as offset,count; Dameng uses the
// standard LIMIT count OFFSET offset order.
func paginationSQL(dialect string) string {
	if dialect == "dm" {
		return " LIMIT ? OFFSET ?"
	}
	return " LIMIT ?,?"
}
