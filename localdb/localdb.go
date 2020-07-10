package localdb

import (
	"os"
	"time"

	//"time"
	sqle "github.com/src-d/go-mysql-server"
	"github.com/src-d/go-mysql-server/auth"
	"github.com/src-d/go-mysql-server/memory"
	"github.com/src-d/go-mysql-server/server"
	"github.com/src-d/go-mysql-server/sql"
)

// DBUser is a struct to allow for sending many users into the DB launcher.

// StartDB creates and initializes the database.
func StartDB(user, pass string) {

	driver := sqle.NewDefault()
	driver.AddDatabase(createTestDatabase())

	config := server.Config{
		Protocol: "tcp",
		Address:  "localhost:3306",
		Auth:     auth.NewNativeSingle(user, pass, auth.AllPermissions),
	}

	s, err := server.NewDefaultServer(config, driver)
	if err != nil {
		panic(err)
	}

	s.Start()

	//FIXME Export data on Exit.
}

func createTestDatabase() *memory.Database {
	const (
		dbName    = "test"
		tableName = "mytable"
	)

	db := memory.NewDatabase(dbName)
	table := memory.NewTable(tableName, sql.Schema{
		{Name: "name", Type: sql.Text, Nullable: false, Source: tableName},
		{Name: "email", Type: sql.Text, Nullable: false, Source: tableName},
		{Name: "phone_numbers", Type: sql.JSON, Nullable: false, Source: tableName},
		{Name: "created_at", Type: sql.Timestamp, Nullable: false, Source: tableName},
	})

	db.AddTable(tableName, table)
	ctx := sql.NewEmptyContext()

	rows := []sql.Row{
		sql.NewRow("John Doe", "john@doe.com", []string{"555-555-555"}, time.Now()),
		sql.NewRow("John Doe", "johnalt@doe.com", []string{}, time.Now()),
		sql.NewRow("Jane Doe", "jane@doe.com", []string{}, time.Now()),
		sql.NewRow("Evil Bob", "evilbob@gmail.com", []string{"555-666-555", "666-666-666"}, time.Now()),
	}

	for _, row := range rows {
		table.Insert(ctx, row)
	}

	return db
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err

}
