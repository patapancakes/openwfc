package database

import (
	"owfc/common"
	"slices"
	"strings"
	"time"
)

type DwcRankingEntry struct {
	ProfileID uint32
	Region    int
	Category  int
	Score     int
	Data      []byte
	Created   time.Time

	Order int // only set for the user's own row
}

const (
	insertDwcRanking   = `REPLACE INTO dwc_rankings (gamename, pid, region, category, score, data) VALUES (?, ?, ?, ?, ?, ?)`
	getDwcRanking      = `SELECT pid, region, category, score, data, updated FROM dwc_rankings WHERE gamename = ? AND pid = ? AND region & ? AND category = ? AND updated >= ?`
	getDwcRankingOrder = `SELECT COUNT(*) + 1 FROM dwc_rankings WHERE gamename = ? AND region & ? AND category = ? AND updated >= ?`
)

func (c *Connection) InsertDwcRanking(gamename string, pid uint32, region int, category int, score int, data []byte) error {
	_, err := c.pool.ExecContext(c.ctx, insertDwcRanking, gamename, pid, region, category, score, data)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) GetDwcRanking(gamename string, pid uint32, region int, category int, desc bool, since time.Time) (DwcRankingEntry, error) {
	var entry DwcRankingEntry
	err := c.pool.QueryRowContext(c.ctx, getDwcRanking, gamename, pid, region, category, since).Scan(&entry.ProfileID, &entry.Region, &entry.Category, &entry.Score, &entry.Data, &entry.Created)
	if err != nil {
		return DwcRankingEntry{}, err
	}

	sort := " AND score < ?"
	if desc {
		sort = " AND score > ?"
	}

	err = c.pool.QueryRowContext(c.ctx, getDwcRankingOrder+sort, gamename, region, category, since, entry.Score).Scan(&entry.Order)
	if err != nil {
		return DwcRankingEntry{}, err
	}

	return entry, nil
}

func (c *Connection) GetDwcRankings(gamename string, pid uint32, region int, category int, mode int, desc bool, since time.Time, limit int, friends []uint32) ([]DwcRankingEntry, int, error) {
	// not checking since.IsZero is okay since it'll return all anyway
	q := "SELECT pid, region, category, score, data, updated FROM dwc_rankings WHERE gamename = ? AND region & ? AND category = ? AND updated >= ?"
	args := []any{gamename, region, category, since}

	sort := " ORDER BY score ASC"
	if desc {
		sort = " ORDER BY score DESC"
	}
	var sortArgs []any

	var entries []DwcRankingEntry

	// near and friends return the user's own score at the top
	var entry DwcRankingEntry
	if mode != common.DwcRankOrder && mode != common.DwcRankTop {
		q += " AND pid != ?"
		args = append(args, pid)

		var err error
		entry, err = c.GetDwcRanking(gamename, pid, region, category, desc, since)
		if err != nil {
			return nil, 0, err
		}

		entries = append(entries, entry)

		limit--
	}

	switch mode {
	//case common.DwcRankOrder: // will interfere with total, do it later
	//case common.DwcRankTop: // already covered?
	case common.DwcRankNear, common.DwcRankNearHigh, common.DwcRankNearLow:
		// TODO: wish the appends weren't duplicated
		switch mode {
		case common.DwcRankNear:
			sort = strings.ReplaceAll(sort, "score", "ABS(? - score)")
			sortArgs = append(sortArgs, entry.Score)
		case common.DwcRankNearHigh:
			q += " AND score >= ?"
			args = append(args, entry.Score)
		case common.DwcRankNearLow:
			q += " AND score <= ?"
			args = append(args, entry.Score)
		}
	case common.DwcRankFriends:
		q += " AND pid IN (" + strings.Join(slices.Repeat([]string{"?"}, len(friends)), ",") + ")"
		for _, pid := range friends {
			args = append(args, pid)
		}
	}

	// HACK: get the total
	var total int
	err := c.pool.QueryRowContext(c.ctx, strings.ReplaceAll(q, "pid, region, category, score, data, updated", "COUNT(*)"), args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// TODO: might be worth its own function due to being different
	if mode == common.DwcRankOrder {
		entry, err := c.GetDwcRanking(gamename, pid, region, category, desc, since)
		if err != nil {
			return nil, 0, err
		}

		return []DwcRankingEntry{entry}, total, nil
	}

	q += sort
	args = append(args, sortArgs...)

	q += " LIMIT ?"
	args = append(args, limit)

	rows, err := c.pool.QueryContext(c.ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	for rows.Next() {
		var entry DwcRankingEntry
		err = rows.Scan(&entry.ProfileID, &entry.Region, &entry.Category, &entry.Score, &entry.Data, &entry.Created)
		if err != nil {
			return nil, 0, err
		}

		entries = append(entries, entry)
	}

	return entries, total, nil
}
