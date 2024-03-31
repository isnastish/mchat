package session

// Regardless if client's remote address, they should be verified by its name.
// And inserted/deleted from a db accordingly
// TODO: Try out gorm package so we don't construct all the queries manually. (for learning purposes) github.com/go-gorm/gorm

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type SqlDB struct {
	driver         string
	dataSrouceName string // TODO: Figure out how to handle it more securily
	tableName      string
	tableExists    bool
	*sql.DB
}

func newDb(driver, dataSourceName string) (*SqlDB, error) {
	db, err := sql.Open(driver, dataSourceName)
	if err != nil {
		return nil, err
	}
	// defer db.Close()

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	tableName := "sessionParticipants"

	// NOTE: Only for development purposes
	_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", tableName))
	if err != nil {
		return nil, err
	}

	// All the metrics regarding send/received messages should be dumped at the end to a db.
	// We shouldn't update our db every time we recv or send a messages, it will be inefficient.
	query := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s ("+
			"id smallint NOT NULL AUTO_INCREMENT,"+
			"name varchar(64) NOT NULL,"+
			"ip_address varchar(64) NOT NULL,"+
			"status varchar(32) NOT NULL,"+
			"connectionTime DATETIME,"+
			"sentMessages int,"+
			"recvMessages int,"+
			"PRIMARY KEY (id, name)"+
			") AUTO_INCREMENT = 1;",
		tableName,
	)

	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	res := &SqlDB{
		driver:         driver,
		dataSrouceName: dataSourceName,
		tableName:      tableName,
		DB:             db,
	}

	return res, nil
}

func (db *SqlDB) hasClient(name string) bool {
	query := fmt.Sprintf(
		"SELECT "+
			"name"+
			" FROM %s "+
			"WHERE ("+
			"name = \"%s\""+
			");",
		db.tableName,
		name,
	)

	rows, err := db.DB.Query(query)
	if err != nil {
		log.Error().Msgf("query [%s] failed with an error: %s", query, err.Error())
		return false
	}
	defer rows.Close()

	exists := rows.Next()
	if err = rows.Err(); err != nil {
		log.Error().Msgf("failed to prepare the next row: %s", err.Error())
		return false
	}

	return exists
}

func (db *SqlDB) insertClient(name string, ipAddress string, status string, connectionTime string) bool {
	query := fmt.Sprintf(
		"INSERT INTO %s ("+
			"name,"+
			"ip_address,"+
			"status,"+
			"connectionTime"+
			") "+
			"VALUES ("+
			"\"%s\","+
			"\"%s\","+
			"\"%s\","+
			"\"%s\""+
			");",
		db.tableName,
		name,
		ipAddress,
		status,
		connectionTime,
	)

	res, err := db.Exec(query)
	if err != nil {
		log.Error().Msgf("query [%s] failed with an error: %s", query, err.Error())
		return false
	}

	id, err := res.LastInsertId()
	_ = id

	return true
}

type Participants [][3]string

func (db *SqlDB) getAllParticipants() (Participants, error) {
	// keep a query in a separate variable in case it grows
	query := fmt.Sprintf("SELECT (name, ip_address, status) FROM %s", db.tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query for a participants list: %s", err.Error())
	}
	defer rows.Close()

	res := Participants{}
	for rows.Next() {
		col := [3]string{}
		err = rows.Scan(col)
		if err != nil {
			return nil, fmt.Errorf("failed to scan the rows: %s", err.Error())
		}
		res = append(res, col)
	}

	return res, nil
}
