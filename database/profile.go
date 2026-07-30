package database

import (
	"database/sql"
	"fmt"
	"owfc/common"
	"strings"
)

const (
	InsertProfile    = `INSERT INTO profiles (user_id, gsbrcd) VALUES (?, ?) RETURNING id`
	GetProfile       = `SELECT user_id, gsbrcd, firstname, lastname FROM profiles WHERE id = ?`
	GetUserProfileID = `SELECT id FROM profiles WHERE user_id = ? AND gsbrcd = ?`
	UpdateProfile    = `UPDATE profiles SET firstname = CASE WHEN ? THEN ? ELSE firstname END, lastname = CASE WHEN ? THEN ? ELSE lastname END WHERE id = ?`
)

type Profile struct {
	ID        uint32
	UserID    uint64
	GsbrCode  string // should be removed and dependent fields added to db
	FirstName string
	LastName  string
}

func (p Profile) Email() string {
	return p.UniqueNick() + "@nds"
}

func (p Profile) UniqueNick() string {
	return common.Base32Encode(p.UserID) + p.GsbrCode
}

func (c *Connection) CreateProfile(userId uint64, gsbrcd string) (uint32, error) {
	profile := Profile{
		UserID:   userId,
		GsbrCode: gsbrcd,
	}

	var id uint32
	err := c.pool.QueryRowContext(c.ctx, InsertProfile, profile.UserID, profile.GsbrCode).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (c *Connection) GetProfile(profileId uint32) (Profile, error) {
	var profile Profile
	var firstname sql.NullString
	err := c.pool.QueryRowContext(c.ctx, GetProfile, profileId).Scan(&profile.UserID, &profile.GsbrCode, &firstname, &profile.LastName)
	if err != nil {
		return Profile{}, err
	}

	profile.ID = profileId
	profile.FirstName = firstname.String
	return profile, nil
}

func (c *Connection) GetProfileID(userId uint64, gsbrcd string) (uint32, error) {
	var id uint32
	err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (c *Connection) UpdateProfile(profile Profile) error {
	_, err := c.pool.ExecContext(c.ctx, UpdateProfile, profile.FirstName != "", profile.FirstName, profile.LastName != "", profile.LastName, profile.ID)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) SearchProfile(filter map[string]string) ([]uint32, error) {
	q := "SELECT id FROM profiles WHERE"
	var args []any

	var sub strings.Builder
	for field, value := range filter {
		if sub.Len() != 0 {
			sub.WriteString(" AND")
		}

		fmt.Fprintf(&sub, " %s = ?", field)
		args = append(args, value)
	}

	rows, err := c.pool.QueryContext(c.ctx, q+sub.String(), args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	var ids []uint32
	for rows.Next() {
		var id uint32
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}
