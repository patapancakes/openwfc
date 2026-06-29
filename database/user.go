package database

import (
	"errors"
	"math/rand"
	"time"
)

const (
	InsertUser              = `INSERT INTO users (user_id, gsbrcd, password, ng_device_id, email, unique_nick) VALUES (?, ?, ?, ?, ?, ?) RETURNING profile_id`
	InsertUserWithProfileID = `INSERT INTO users (profile_id, user_id, gsbrcd, password, ng_device_id, email, unique_nick) VALUES (?, ?, ?, ?, ?, ?, ?)`
	UpdateUserTable         = `UPDATE users SET firstname = CASE WHEN ? THEN ? ELSE firstname END, lastname = CASE WHEN ? THEN ? ELSE lastname END, open_host = CASE WHEN ? THEN ? ELSE open_host END WHERE profile_id = ?`
	UpdateUserProfileID     = `UPDATE users SET profile_id = ? WHERE user_id = ? AND gsbrcd = ?`
	UpdateUserNGDeviceID    = `UPDATE users SET ng_device_id = ? WHERE profile_id = ?`
	GetUser                 = `SELECT user_id, gsbrcd, email, unique_nick, firstname, lastname, open_host, last_ip_address, last_ingamesn FROM users WHERE profile_id = ?`
	ClearProfileQuery       = `DELETE FROM users WHERE profile_id = ? RETURNING user_id, gsbrcd, email, unique_nick, firstname, lastname, open_host, last_ip_address, last_ingamesn`
	DoesUserExist           = `SELECT EXISTS(SELECT 1 FROM users WHERE user_id = ? AND gsbrcd = ?)`
	IsProfileIDInUse        = `SELECT EXISTS(SELECT 1 FROM users WHERE profile_id = ?)`
	DeleteUserSession       = `DELETE FROM sessions WHERE profile_id = ?`
	GetUserProfileID        = `SELECT profile_id, ng_device_id, email, unique_nick, firstname, lastname, open_host, last_ip_address, allow_default_keys FROM users WHERE user_id = ? AND gsbrcd = ?`
	UpdateUserLastIPAddress = `UPDATE users SET last_ip_address = ?, last_ingamesn = ? WHERE profile_id = ?`
	UpdateUserBan           = `UPDATE users SET has_ban = true, ban_issued = ?, ban_expires = ?, ban_reason = ?, ban_reason_hidden = ?, ban_moderator = ?, ban_tos = ? WHERE profile_id = ?`
	SearchUserBan           = `SELECT has_ban, ban_tos, ng_device_id FROM users WHERE has_ban = true AND (profile_id = ? OR ng_device_id = ? OR last_ip_address = ?) AND (ban_expires IS NULL OR ban_expires > ?) AND (ban_expires IS NULL OR ban_expires > ?) ORDER BY ban_tos DESC LIMIT 1`
	SearchUserBanInfo       = `SELECT has_ban, ban_tos, ban_issued, ban_expires, ban_reason, ng_device_id, profile_id, gsbrcd, last_ingamesn FROM users WHERE has_ban = true AND (profile_id = ? OR ng_device_id = ? OR last_ip_address = ?) ORDER BY ban_expires DESC LIMIT 1`
	DisableUserBan          = `UPDATE users SET has_ban = false WHERE profile_id = ?`
)

type User struct {
	ProfileId          uint32
	UserId             uint64
	GsbrCode           string
	NgDeviceId         uint32
	Email              string
	UniqueNick         string
	FirstName          string
	LastName           string
	Restricted         bool
	RestrictedDeviceId uint32
	BanReason          string
	OpenHost           bool
	LastInGameSn       string
	LastIPAddress      string
	Created            bool
}

var (
	ErrProfileIDInUse         = errors.New("profile ID is already in use")
	ErrReservedProfileIDRange = errors.New("profile ID is in reserved range")
)

func (c *Connection) CreateUser(user *User) error {
	if user.ProfileId == 0 {
		return c.pool.QueryRowContext(c.ctx, InsertUser, user.UserId, user.GsbrCode, "", user.NgDeviceId, user.Email, user.UniqueNick).Scan(&user.ProfileId)
	}

	if user.ProfileId >= 1000000000 {
		return ErrReservedProfileIDRange
	}

	var exists bool
	err := c.pool.QueryRowContext(c.ctx, IsProfileIDInUse, user.ProfileId).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return ErrProfileIDInUse
	}

	_, err = c.pool.ExecContext(c.ctx, InsertUserWithProfileID, user.ProfileId, user.UserId, user.GsbrCode, "", user.NgDeviceId, user.Email, user.UniqueNick)
	return err
}

func (c *Connection) UpdateProfileID(user *User, newProfileId uint32) error {
	if newProfileId >= 1000000000 {
		return ErrReservedProfileIDRange
	}

	var exists bool
	err := c.pool.QueryRowContext(c.ctx, IsProfileIDInUse, newProfileId).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return ErrProfileIDInUse
	}

	_, err = c.pool.ExecContext(c.ctx, UpdateUserProfileID, newProfileId, user.UserId, user.GsbrCode)
	if err == nil {
		user.ProfileId = newProfileId
	}

	return err
}

func GetUniqueUserID() uint64 {
	// Not guaranteed unique but doesn't matter in practice if multiple people have the same user ID.
	return uint64(rand.Int63n(0x80000000000))
}

func (c *Connection) UpdateProfile(user *User, data map[string]string) {
	firstName, firstNameExists := data["firstname"]
	lastName, lastNameExists := data["lastname"]
	openHost, openHostExists := data["wl:oh"]
	openHostBool := openHostExists && openHost != "0"

	_, err := c.pool.ExecContext(c.ctx, UpdateUserTable, firstNameExists, firstName, lastNameExists, lastName, openHostBool, openHost, user.ProfileId)
	if err != nil {
		panic(err)
	}

	if firstNameExists {
		user.FirstName = firstName
	}

	if lastNameExists {
		user.LastName = lastName
	}

	if openHostExists {
		user.OpenHost = openHostBool
	}
}

func (c *Connection) GetProfile(profileId uint32) (User, bool) {
	user := User{}
	row := c.pool.QueryRowContext(c.ctx, GetUser, profileId)
	err := row.Scan(&user.UserId, &user.GsbrCode, &user.Email, &user.UniqueNick, &user.FirstName, &user.LastName, &user.OpenHost, &user.LastIPAddress, &user.LastInGameSn)
	if err != nil {
		return User{}, false
	}

	user.ProfileId = profileId
	return user, true
}

func (c *Connection) ClearProfile(profileId uint32) (User, bool) {
	user := User{}
	row := c.pool.QueryRowContext(c.ctx, ClearProfileQuery, profileId)
	err := row.Scan(&user.UserId, &user.GsbrCode, &user.Email, &user.UniqueNick, &user.FirstName, &user.LastName, &user.OpenHost, &user.LastIPAddress, &user.LastInGameSn)

	if err != nil {
		return User{}, false
	}

	user.ProfileId = profileId
	return user, true
}

func (c *Connection) BanUser(profileId uint32, tos bool, length time.Duration, reason string, reasonHidden string, moderator string) bool {
	_, err := c.pool.ExecContext(c.ctx, UpdateUserBan, time.Now().UTC(), time.Now().UTC().Add(length), reason, reasonHidden, moderator, tos, profileId)
	return err == nil
}

func (c *Connection) UnbanUser(profileId uint32) bool {
	_, err := c.pool.ExecContext(c.ctx, DisableUserBan, profileId)
	return err == nil
}

func (c *Connection) SearchUserBan(profileId uint32, ngDeviceId uint32, ipAddress string, lastIpAddress string) (
	tos bool, issued time.Time, expires time.Time, reason string, bannedProfileId uint32, gsbrCode string, inGameName string, err error) {
	row := c.pool.QueryRowContext(c.ctx, SearchUserBanInfo, ngDeviceId, profileId, ipAddress, lastIpAddress)
	var hasBan bool
	var bannedNgDeviceId []uint32
	err = row.Scan(&hasBan, &tos, &issued, &expires, &reason, &bannedNgDeviceId, &bannedProfileId, &gsbrCode, &inGameName)
	if err == nil && !hasBan {
		err = errors.New("no ban found")
	}
	if len(gsbrCode) > 4 {
		gsbrCode = gsbrCode[:4]
	}
	return tos, issued, expires, reason, bannedProfileId, gsbrCode, inGameName, err
}
