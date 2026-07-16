package database

import (
	"owfc/common"
)

type MarioKartWiiTopTenRanking struct {
	Score      int
	PID        int
	PlayerInfo string
}

const (
	getTopTenRankingsQuery = "" +
		"SELECT score, pid, playerinfo " +
		"FROM mario_kart_wii_sake " +
		"WHERE (? = 0 OR regionid = ?) " +
		"AND courseid = ? " +
		"ORDER BY score ASC " +
		"LIMIT 10"
	getGhostDataQuery = "" +
		"SELECT id " +
		"FROM mario_kart_wii_sake " +
		"WHERE courseid = ? " +
		"AND score < ? " +
		"ORDER BY score DESC " +
		"LIMIT 1"
	getStoredGhostDataQuery = "" +
		"SELECT pid, id " +
		"FROM mario_kart_wii_sake " +
		"WHERE (? = 0 OR regionid = ?) " +
		"AND courseid = ? " +
		"ORDER BY score ASC " +
		"LIMIT 1"
	getFileQuery = "" +
		"SELECT ghost " +
		"FROM mario_kart_wii_sake " +
		"WHERE id = ? " +
		"LIMIT 1"
	getGhostFileQuery = "" +
		"SELECT ghost " +
		"FROM mario_kart_wii_sake " +
		"WHERE courseid = ? " +
		"AND score < ? " +
		"AND pid <> ? " +
		"ORDER BY score DESC " +
		"LIMIT 1"
	insertGhostFileStatement = "" +
		"INSERT INTO mario_kart_wii_sake (regionid, courseid, score, pid, playerinfo, ghost, upload_time) " +
		"VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP) " +
		"ON DUPLICATE KEY UPDATE " +
		"regionid = VALUES(regionid), score = VALUES(score), playerinfo = VALUES(playerinfo), ghost = VALUES(ghost), upload_time = CURRENT_TIMESTAMP"
)

func (c *Connection) GetMarioKartWiiTopTenRankings(regionId common.MarioKartWiiLeaderboardRegionId,
	courseId common.MarioKartWiiCourseId) ([]MarioKartWiiTopTenRanking, error) {
	rows, err := c.pool.QueryContext(c.ctx, getTopTenRankingsQuery, regionId, regionId, courseId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topTenRankings := make([]MarioKartWiiTopTenRanking, 0, 10)
	for rows.Next() {
		var topTenRanking MarioKartWiiTopTenRanking
		err = rows.Scan(&topTenRanking.Score, &topTenRanking.PID, &topTenRanking.PlayerInfo)
		if err != nil {
			return nil, err
		}

		topTenRankings = append(topTenRankings, topTenRanking)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return topTenRankings, nil
}

func (c *Connection) GetMarioKartWiiGhostData(courseId common.MarioKartWiiCourseId, time int) (int, error) {
	row := c.pool.QueryRowContext(c.ctx, getGhostDataQuery, courseId, time)

	var fileId int
	if err := row.Scan(&fileId); err != nil {
		return 0, err
	}

	return fileId, nil
}

func (c *Connection) GetMarioKartWiiStoredGhostData(regionId common.MarioKartWiiLeaderboardRegionId,
	courseId common.MarioKartWiiCourseId) (int, int, error) {
	row := c.pool.QueryRowContext(c.ctx, getStoredGhostDataQuery, regionId, regionId, courseId)

	var pid int
	var fileId int
	if err := row.Scan(&pid, &fileId); err != nil {
		return 0, 0, err
	}

	return pid, fileId, nil
}

func (c *Connection) GetMarioKartWiiFile(fileId int) ([]byte, error) {
	row := c.pool.QueryRowContext(c.ctx, getFileQuery, fileId)

	var file []byte
	if err := row.Scan(&file); err != nil {
		return nil, err
	}

	return file, nil
}

func (c *Connection) GetMarioKartWiiGhostFile(courseId common.MarioKartWiiCourseId, time int, pid int) ([]byte, error) {
	row := c.pool.QueryRowContext(c.ctx, getGhostFileQuery, courseId, time, pid)

	var ghost []byte
	if err := row.Scan(&ghost); err != nil {
		return nil, err
	}

	return ghost, nil
}

func (c *Connection) InsertMarioKartWiiGhostFile(regionId common.MarioKartWiiLeaderboardRegionId,
	courseId common.MarioKartWiiCourseId, score int, pid int, playerInfo string, ghost []byte) error {
	_, err := c.pool.ExecContext(c.ctx, insertGhostFileStatement, regionId, courseId, score, pid, playerInfo, ghost)

	return err
}
