module github.com/atcdot/gorm-bulk-upsert

go 1.23

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/go-sql-driver/mysql v1.8.1
	github.com/jinzhu/gorm v1.9.16
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
)

replace github.com/mattn/go-sqlite3 => github.com/mattn/go-sqlite3 v1.14.17
