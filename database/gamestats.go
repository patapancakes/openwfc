package database

import (
	"database/sql"
	"owfc/common"
	"slices"
	"time"
)

const (
	getPersistData = `SELECT data, modified FROM gamestats_persistdata WHERE pid = ? AND dindex = ? AND ptype = ?`
	setPersistData = `INSERT INTO gamestats_persistdata (pid, ptype, dindex, data) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE data = VALUES(data), modified = UTC_TIMESTAMP RETURNING modified`
)

func (c *Connection) GetGameStatsPersistData(profileId uint32, ptype int, dindex int) (data string, modified time.Time, err error) {
	err = c.pool.QueryRowContext(c.ctx, getPersistData, profileId, ptype, dindex).Scan(&data, &modified)
	return
}

func (c *Connection) SetGameStatsPersistData(profileId uint32, ptype int, dindex int, data string) (modified time.Time, err error) {
	err = c.pool.QueryRowContext(c.ctx, setPersistData, profileId, ptype, dindex, data).Scan(&modified)
	return
}

func (c *Connection) GetGameStatsPersistDataKV(profileId uint32, ptype int, dindex int, keys []string) (kv common.KeyValues, modified time.Time, err error) {
	var data string
	data, modified, err = c.GetGameStatsPersistData(profileId, ptype, dindex)
	if err != nil {
		if err != sql.ErrNoRows {
			return
		}

		err = nil
	}

	if keys != nil {
		kv = slices.DeleteFunc(common.KeyValuesFromString(data), func(kv common.KV) bool {
			return !slices.Contains(keys, kv.K)
		})
	}

	return
}

func (c *Connection) SetGameStatsPersistDataKV(profileId uint32, ptype int, dindex int, kvs common.KeyValues) (modified time.Time, err error) {
	var data common.KeyValues
	data, modified, err = c.GetGameStatsPersistDataKV(profileId, ptype, dindex, nil)
	if err != nil {
		if err != sql.ErrNoRows {
			return
		}

		err = nil
	}

	for _, kv := range kvs {
		data.Set(kv.K, kv.V)
	}

	modified, err = c.SetGameStatsPersistData(profileId, ptype, dindex, data.Encode())
	return
}
