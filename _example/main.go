package main

import (
	"time"

	"gopkg.in/src-d/go-mysql-server.v0"
	"gopkg.in/src-d/go-mysql-server.v0/mem"
	"gopkg.in/src-d/go-mysql-server.v0/server"
	"gopkg.in/src-d/go-mysql-server.v0/sql"
	"gopkg.in/src-d/go-vitess.v0/mysql"
)

// Example of how to implement a MySQL server based on a Engine:
//
// ```
// > mysql --host=127.0.0.1 --port=5123 -u user -ppass db -e "SELECT * FROM mytable"
// +----------+-------------------+-------------------------------+---------------------+
// | name     | email             | phone_numbers                 | created_at          |
// +----------+-------------------+-------------------------------+---------------------+
// | John Doe | john@doe.com      | ["555-555-555"]               | 2018-04-18 09:41:13 |
// | John Doe | johnalt@doe.com   | []                            | 2018-04-18 09:41:13 |
// | Jane Doe | jane@doe.com      | []                            | 2018-04-18 09:41:13 |
// | Evil Bob | evilbob@gmail.com | ["555-666-555","666-666-666"] | 2018-04-18 09:41:13 |
// +----------+-------------------+-------------------------------+---------------------+
// ```
func main() {
	driver := sqle.NewDefault()
	driver.AddDatabase(createTestDatabase())

	auth := mysql.NewAuthServerStatic()
	auth.Entries["user"] = []*mysql.AuthServerStaticEntry{{
		Password: "pass",
	}}

	config := server.Config{
		Protocol: "tcp",
		Address:  "localhost:3306",
		Auth:     auth,
	}

	s, err := server.NewDefaultServer(config, driver)
	if err != nil {
		panic(err)
	}

	s.Start()
}

func createTestDatabase() *mem.Database {
	const (
		dbName    = "test"
		tableName = "mytable"
	)

	db := mem.NewDatabase(dbName)
	table := mem.NewTable(tableName, sql.Schema{
		{Name: "name", Type: sql.Text, Nullable: false, Source: tableName},
		{Name: "email", Type: sql.Text, Nullable: false, Source: tableName},
		{Name: "phone_numbers", Type: sql.JSON, Nullable: false, Source: tableName},
		{Name: "created_at", Type: sql.Timestamp, Nullable: false, Source: tableName},
	})

	db.AddTable(tableName, table)
	ctx := sql.NewEmptyContext()
	table.Insert(ctx, sql.NewRow("John Doe", "john@doe.com", []string{"555-555-555"}, time.Now()))
	table.Insert(ctx, sql.NewRow("John Doe", "johnalt@doe.com", []string{}, time.Now()))
	table.Insert(ctx, sql.NewRow("Jane Doe", "jane@doe.com", []string{}, time.Now()))
	table.Insert(ctx, sql.NewRow("Evil Bob", "evilbob@gmail.com", []string{"555-666-555", "666-666-666"}, time.Now()))
	return db
}
