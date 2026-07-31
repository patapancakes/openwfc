package database

import (
	"database/sql"
	"errors"
)

const (
	insertUser         = `INSERT INTO users (id, unitcd, macadr, passwd, csnum) VALUES (?, ?, ?, ?, ?)`
	getUser            = `SELECT unitcd, macadr, passwd, csnum, banned FROM users WHERE id = ?`
	updateUserName     = `UPDATE users SET name = ? WHERE id = ?`
	isUserIDInUse      = `SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`
	isMACInUse         = `SELECT EXISTS(SELECT 1 FROM users WHERE macadr = ?)`
	isSerialNumerInUse = `SELECT EXISTS(SELECT 1 FROM users WHERE csnum = ?)`
)

type User struct {
	ID           uint64
	Name         string
	UnitCode     int
	MacAddress   string
	Password     int    // ds only
	SerialNumber string // wii only
	Banned       bool
}

func (u User) IsWii() bool {
	return u.UnitCode == 1
}

var (
	ErrUserIDInUse       = errors.New("user ID is already in use")
	ErrMACInUse          = errors.New("mac address is already in use")
	ErrSerialNumberInUse = errors.New("serial number is already in use")
)

func (c *Connection) CreateUser(user User) error {
	var exists bool

	err := c.pool.QueryRowContext(c.ctx, isUserIDInUse, user.ID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrUserIDInUse
	}

	err = c.pool.QueryRowContext(c.ctx, isMACInUse, user.MacAddress).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrMACInUse
	}

	if user.IsWii() {
		err = c.pool.QueryRowContext(c.ctx, isSerialNumerInUse, user.SerialNumber).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			return ErrSerialNumberInUse
		}
	}

	var password *int
	var serial *string
	if !user.IsWii() {
		password = &user.Password
	} else {
		serial = &user.SerialNumber
	}

	_, err = c.pool.ExecContext(c.ctx, insertUser, user.ID, user.UnitCode, user.MacAddress, password, serial)
	return err
}

func (c *Connection) GetUser(userId uint64) (User, bool) {
	var user User
	var password sql.NullInt16
	var serial sql.NullString
	err := c.pool.QueryRowContext(c.ctx, getUser, userId).Scan(&user.UnitCode, &user.MacAddress, &password, &serial, &user.Banned)
	if err != nil {
		return User{}, false
	}

	user.ID = userId
	user.Password = int(password.Int16)
	user.SerialNumber = serial.String

	return user, true
}

func (c *Connection) UpdateUserName(userId uint64, name string) error {
	_, err := c.pool.ExecContext(c.ctx, updateUserName, name, userId)
	if err != nil {
		return err
	}

	return nil
}
