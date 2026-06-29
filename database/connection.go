package database

import (
	"context"
	"database/sql"
	"fmt"
	"wwfc/common"
)

type Connection struct {
	pool *sql.DB
	ctx  context.Context
}

func Start(config common.Config) Connection {
	conn := Connection{
		ctx: context.Background(),
	}

	var err error
	conn.pool, err = sql.Open("mysql", fmt.Sprintf("%s:%s@%s/%s?parseTime=true", config.Username, config.Password, config.DatabaseAddress, config.DatabaseName))
	if err != nil {
		panic(err)
	}

	return conn
}

func (c *Connection) Close() {
	if c != nil && c.pool != nil {
		c.pool.Close()
	}
}
