package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

var (
	ErrDeviceIDMismatch   = errors.New("NG device ID mismatch")
	ErrProhibitedDeviceID = errors.New("used prohibited NG device ID in request")
	ErrProfileBannedTOS   = errors.New("profile is banned for violating the Terms of Service")
)

func (c *Connection) LoginUserToGPCM(userId uint64, gsbrcd string, profileId uint32, defaultKey bool, ngDeviceId uint32, ipAddress string, ingamesn string, deviceAuth bool) (User, error) {
	var exists bool
	err := c.pool.QueryRowContext(c.ctx, DoesUserExist, userId, gsbrcd).Scan(&exists)
	if err != nil {
		return User{}, err
	}

	user := User{
		UserId:   userId,
		GsbrCode: gsbrcd,
	}

	var lastIPAddress *string

	if !exists {
		user.ProfileId = profileId
		user.NgDeviceId = ngDeviceId
		user.UniqueNick = common.Base32Encode(userId) + gsbrcd
		user.Email = user.UniqueNick + "@nds"

		// Create the GPCM account
		err := c.CreateUser(&user)
		if err != nil {
			logging.Error("DATABASE", "Error creating user:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(user.ProfileId), "\nerror:", err.Error())
			return User{}, err
		}

		logging.Notice("DATABASE", "Created new GPCM user:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(user.ProfileId))
		user.Created = true
	} else {
		var expectedNgId *uint32
		var firstName *string
		var lastName *string
		var allowDefaultKeys bool

		err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&user.ProfileId, &expectedNgId, &user.Email, &user.UniqueNick, &firstName, &lastName, &user.OpenHost, &lastIPAddress, &allowDefaultKeys)
		if err != nil {
			return User{}, err
		}

		if defaultKey && !allowDefaultKeys && !common.GetConfig().AllowDefaultDolphinKeys {
			return User{}, ErrProhibitedDeviceID
		}

		if firstName != nil {
			user.FirstName = *firstName
		}

		if lastName != nil {
			user.LastName = *lastName
		}

		if expectedNgId != nil && *expectedNgId != 0 {
			user.NgDeviceId = *expectedNgId
			if ngDeviceId != 0 && user.NgDeviceId != ngDeviceId {
				logging.Error("DATABASE", "NG device ID mismatch for profile", aurora.Cyan(user.ProfileId), "- expected", aurora.Cyan(fmt.Sprintf("%08x", user.NgDeviceId)), "but got", aurora.Cyan(fmt.Sprintf("%08x", ngDeviceId)))
				return User{}, ErrDeviceIDMismatch
			}
		} else if ngDeviceId != 0 {
			user.NgDeviceId = ngDeviceId
			_, err := c.pool.ExecContext(c.ctx, UpdateUserNGDeviceID, user.NgDeviceId, user.ProfileId)
			if err != nil {
				return User{}, err
			}
		}

		if profileId != 0 && user.ProfileId != profileId {
			err := c.UpdateProfileID(&user, profileId)
			if err != nil {
				logging.Warn("DATABASE", "Could not update", aurora.Cyan(userId), aurora.Cyan(gsbrcd), "profile ID from", aurora.Cyan(user.ProfileId), "to", aurora.Cyan(profileId))
			} else {
				logging.Notice("DATABASE", "Updated GPCM user profile ID:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(user.ProfileId))
			}
		}

		logging.Notice("DATABASE", "Log in GPCM user:", aurora.Cyan(userId), aurora.Cyan(user.GsbrCode), "-", aurora.Cyan(user.ProfileId))
	}

	// This should be set if the user already knows its own profile ID
	if profileId != 0 && user.LastName == "" {
		c.UpdateProfile(&user, map[string]string{
			"lastname": "000000000" + gsbrcd,
		})
	}

	// Update the user's last IP address and ingamesn
	if deviceAuth {
		_, err = c.pool.ExecContext(c.ctx, UpdateUserLastIPAddress, ipAddress, ingamesn, user.ProfileId)
		if err != nil {
			return User{}, err
		}
	}

	emptyString := ""
	if lastIPAddress == nil {
		lastIPAddress = &emptyString
	}

	// Find ban from device ID or IP address
	var banExists bool
	var banTOS bool
	var bannedDeviceId uint32
	var banReason string
	timeNow := time.Now().UTC()
	err = c.pool.QueryRowContext(c.ctx, SearchUserBan, user.NgDeviceId, user.ProfileId, ipAddress, *lastIPAddress, timeNow).Scan(&banExists, &banTOS, &bannedDeviceId, &banReason)

	if err != nil {
		if err != sql.ErrNoRows {
			return User{}, err
		}

		banExists = false
	}

	if banExists {
		if banTOS {
			logging.Warn("DATABASE", "Profile", aurora.Cyan(user.ProfileId), "is banned")
			return User{RestrictedDeviceId: bannedDeviceId, BanReason: banReason}, ErrProfileBannedTOS
		}

		logging.Warn("DATABASE", "Profile", aurora.Cyan(user.ProfileId), "is restricted")
		user.Restricted = true
		user.RestrictedDeviceId = bannedDeviceId
		user.BanReason = banReason
	}

	return user, nil
}

func (c *Connection) LoginUserToGameStats(userId uint64, gsbrcd string) (User, error) {
	user := User{
		UserId:   userId,
		GsbrCode: gsbrcd,
	}

	var firstName *string
	var lastName *string
	var lastIPAddress *string
	var allowDefaultKeys bool

	err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&user.ProfileId, &user.NgDeviceId, &user.Email, &user.UniqueNick, &firstName, &lastName, &user.OpenHost, &lastIPAddress, &allowDefaultKeys)
	if err != nil {
		return User{}, err
	}

	if firstName != nil {
		user.FirstName = *firstName
	}

	if lastName != nil {
		user.LastName = *lastName
	}

	return user, nil
}
