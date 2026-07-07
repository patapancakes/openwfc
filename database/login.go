package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

var (
	ErrDeviceIDMismatch   = errors.New("NG device ID mismatch")
	ErrProhibitedDeviceID = errors.New("used prohibited NG device ID in request")
	ErrProfileBannedTOS   = errors.New("profile is banned for violating the Terms of Service")
)

func (c *Connection) LoginUserToGPCM(userId uint64, gsbrcd string, profileId uint32, ngDeviceId uint32, ipAddress string, ingamesn string) (Profile, error) {
	var exists bool
	err := c.pool.QueryRowContext(c.ctx, DoesProfileExist, userId, gsbrcd).Scan(&exists)
	if err != nil {
		return Profile{}, err
	}

	profile := Profile{
		UserID:   userId,
		GsbrCode: gsbrcd,
	}

	var lastIPAddress *string

	if !exists {
		profile.ID = profileId
		profile.NgDeviceId = ngDeviceId

		// Create the GPCM account
		err := c.CreateProfile(&profile)
		if err != nil {
			logging.Error("DATABASE", "Error creating profile:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID), "\nerror:", err.Error())
			return Profile{}, err
		}

		logging.Notice("DATABASE", "Created new GPCM profile:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID))
		profile.Created = true
	} else {
		var expectedNgId *uint32
		var firstName *string
		var lastName *string

		err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&profile.ID, &expectedNgId, &firstName, &lastName, &profile.OpenHost, &lastIPAddress)
		if err != nil {
			return Profile{}, err
		}

		if firstName != nil {
			profile.FirstName = *firstName
		}

		if lastName != nil {
			profile.LastName = *lastName
		}

		if expectedNgId != nil && *expectedNgId != 0 {
			profile.NgDeviceId = *expectedNgId
			if ngDeviceId != 0 && profile.NgDeviceId != ngDeviceId {
				logging.Error("DATABASE", "NG device ID mismatch for profile", aurora.Cyan(profile.ID), "- expected", aurora.Cyan(fmt.Sprintf("%08x", profile.NgDeviceId)), "but got", aurora.Cyan(fmt.Sprintf("%08x", ngDeviceId)))
				return Profile{}, ErrDeviceIDMismatch
			}
		} else if ngDeviceId != 0 {
			profile.NgDeviceId = ngDeviceId
			_, err := c.pool.ExecContext(c.ctx, UpdateProfileNGDeviceID, profile.NgDeviceId, profile.ID)
			if err != nil {
				return Profile{}, err
			}
		}

		if profileId != 0 && profile.ID != profileId {
			err := c.UpdateProfileID(&profile, profileId)
			if err != nil {
				logging.Warn("DATABASE", "Could not update", aurora.Cyan(userId), aurora.Cyan(gsbrcd), "profile ID from", aurora.Cyan(profile.ID), "to", aurora.Cyan(profileId))
			} else {
				logging.Notice("DATABASE", "Updated GPCM profile ID:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID))
			}
		}

		logging.Notice("DATABASE", "Log in GPCM profile:", aurora.Cyan(userId), aurora.Cyan(profile.GsbrCode), "-", aurora.Cyan(profile.ID))
	}

	// This should be set if the user already knows its own profile ID
	if profileId != 0 && profile.LastName == "" {
		c.UpdateProfile(&profile, map[string]string{
			"lastname": "000000000" + gsbrcd,
		})
	}

	// Update the user's last IP address and ingamesn
	_, err = c.pool.ExecContext(c.ctx, UpdateProfileLastIPAddress, ipAddress, ingamesn, profile.ID)
	if err != nil {
		return Profile{}, err
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
	err = c.pool.QueryRowContext(c.ctx, SearchProfileBan, profile.NgDeviceId, profile.ID, ipAddress, *lastIPAddress, timeNow).Scan(&banExists, &banTOS, &bannedDeviceId, &banReason)

	if err != nil {
		if err != sql.ErrNoRows {
			return Profile{}, err
		}

		banExists = false
	}

	if banExists {
		if banTOS {
			logging.Warn("DATABASE", "Profile", aurora.Cyan(profile.ID), "is banned")
			return Profile{RestrictedDeviceId: bannedDeviceId, BanReason: banReason}, ErrProfileBannedTOS
		}

		logging.Warn("DATABASE", "Profile", aurora.Cyan(profile.ID), "is restricted")
		profile.Restricted = true
		profile.RestrictedDeviceId = bannedDeviceId
		profile.BanReason = banReason
	}

	return profile, nil
}

func (c *Connection) LoginUserToGameStats(userId uint64, gsbrcd string) (Profile, error) {
	profile := Profile{
		UserID:   userId,
		GsbrCode: gsbrcd,
	}

	var firstName *string
	var lastName *string
	var lastIPAddress *string

	err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&profile.ID, &profile.NgDeviceId, &firstName, &lastName, &profile.OpenHost, &lastIPAddress)
	if err != nil {
		return Profile{}, err
	}

	if firstName != nil {
		profile.FirstName = *firstName
	}

	if lastName != nil {
		profile.LastName = *lastName
	}

	return profile, nil
}
