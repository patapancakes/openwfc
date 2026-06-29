package database

import (
	"time"
)

const (
	queryGsGetPublicData    = `SELECT modified_time, pdata FROM gamestats_public_data WHERE profile_id = ? AND dindex = ? AND ptype = ?`
	queryGsInsertPublicData = `INSERT INTO gamestats_public_data (profile_id, dindex, ptype, pdata, modified_time) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP) RETURNING modified_time`
	queryGsUpdatePublicData = `UPDATE gamestats_public_data SET pdata = ?, modified_time = CURRENT_TIMESTAMP WHERE profile_id = ? AND dindex = ? AND ptype = ? RETURNING modified_time`
)

func (c *Connection) GetGameStatsPublicData(profileId uint32, dindex string, ptype string) (modifiedTime time.Time, publicData string, err error) {
	err = c.pool.QueryRowContext(c.ctx, queryGsGetPublicData, profileId, dindex, ptype).Scan(&modifiedTime, &publicData)
	return
}

func (c *Connection) CreateGameStatsPublicData(profileId uint32, dindex string, ptype string, publicData string) (modifiedTime time.Time, err error) {
	err = c.pool.QueryRowContext(c.ctx, queryGsInsertPublicData, profileId, dindex, ptype, publicData).Scan(&modifiedTime)
	return
}

func (c *Connection) UpdateGameStatsPublicData(profileId uint32, dindex string, ptype string, publicData string) (modifiedTime time.Time, err error) {
	err = c.pool.QueryRowContext(c.ctx, queryGsUpdatePublicData, publicData, profileId, dindex, ptype).Scan(&modifiedTime)
	return
}
