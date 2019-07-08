package storage

import (

	// This line is must for working MySQL database
	_ "github.com/go-sql-driver/mysql"

	"github.com/jmoiron/sqlx"
	"github.com/maddevsio/mad-internship-bot/config"
)

// MySQL provides api for work with mysql database
type MySQL struct {
	conn *sqlx.DB
}

// NewMySQL creates a new instance of database API
func NewMySQL(c *config.BotConfig) (*MySQL, error) {
	conn, err := sqlx.Open("mysql", c.DatabaseURL)
	if err != nil {
		conn, err = sqlx.Connect("mysql", "telegram:telegram@tcp(localhost:3306)/telegram?parseTime=true")
		if err != nil {
			return nil, err
		}
	}
	m := &MySQL{conn}
	return m, nil
}
