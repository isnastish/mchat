package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	lgr "github.com/isnastish/chat/pkg/logger"
)

type MysqlBackend struct {
	tableName string // we can have multiple tables, thus this field should be removed
	*sql.DB
}

type MysqlSettings struct {
	Driver         string
	DataSrouceName string
}

var log = lgr.NewLogger("debug")

func NewMysqlBackend(settings *MysqlSettings) (*MysqlBackend, error) {
	db, err := sql.Open(settings.Driver, settings.DataSrouceName)
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

	res := &MysqlBackend{
		tableName: tableName,
		DB:        db,
	}

	return res, nil
}

func (mb *MysqlBackend) HasClient(name string) bool {
	query := fmt.Sprintf(
		"SELECT "+
			"name"+
			" FROM %s "+
			"WHERE ("+
			"name = \"%s\""+
			");",
		mb.tableName,
		name,
	)

	rows, err := mb.DB.Query(query)
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

func (mb *MysqlBackend) RegisterClient(name string, ipAddress string, status string, joinedTime time.Time) error {
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
		mb.tableName,
		name,
		ipAddress,
		status,
		joinedTime.Format(time.DateTime),
	)

	if _, err := mb.Exec(query); err != nil {
		return err
	}
	return nil
}

func (mb *MysqlBackend) GetClients() (map[string]*map[string]string, error) {
	// keep a query in a separate variable in case it grows
	query := fmt.Sprintf("SELECT * FROM %s", mb.tableName)
	rows, err := mb.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query for a participants list: %s", err.Error())
	}
	defer rows.Close()

	res := make(map[string]*map[string]string)
	for rows.Next() {
		col := [3]string{}
		err = rows.Scan(col)
		if err != nil {
			return nil, fmt.Errorf("failed to scan the rows: %s", err.Error())
		}
		// res = append(res, col)
	}
	return res, nil
}

func (mb *MysqlBackend) AddMessage(clientName string, sentTime time.Time, body [1024]byte) /* return error as well */ {
	query := fmt.Sprintf("SELECT * FROM %s WHERE (name = '%s')", mb.tableName, clientName)
	rows, err := mb.Query(query)

	if err != nil {
		return
	}
	defer rows.Close()

}
