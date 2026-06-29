package database

import (
	"encoding/json"
	"errors"
	"strings"
	"wwfc/common"
	"wwfc/filter"

	mysqlerrnum "github.com/bombsimon/mysql-error-numbers/v3"
)

const (
	SakeFieldTypeByte          = 0
	SakeFieldTypeShort         = 1
	SakeFieldTypeInt           = 2
	SakeFieldTypeFloat         = 3
	SakeFieldTypeAsciiString   = 4
	SakeFieldTypeUnicodeString = 5
	SakeFieldTypeBoolean       = 6
	SakeFieldTypeDateAndTime   = 7
	SakeFieldTypeBinaryData    = 8
	SakeFieldTypeInt64         = 9
)

type SakeFieldType int

type SakeField struct {
	Type  SakeFieldType `json:"type"`
	Value string        `json:"value"`
}

type SakeRecord struct {
	GameId   int
	OwnerId  int32
	TableId  string
	RecordId int32
	Fields   map[string]SakeField
}

const (
	getSakeRecordsQuery = `
		SELECT owner_id, record_id, fields 
		FROM sake_records 
		WHERE game_id = ? 
		  AND table_id = ?`

	updateSakeRecordQuery = `
        UPDATE sake_records 
        SET 
            fields = CASE WHEN owner_id = ? THEN CONCAT(fields, ?) ELSE fields END, 
            update_time = CASE WHEN owner_id = ? THEN CURRENT_TIMESTAMP ELSE update_time END 
        WHERE game_id = ? 
          AND table_id = ? 
          AND record_id = ? 
        RETURNING owner_id`

	insertSakeRecordQuery = `
		INSERT INTO sake_records (game_id, table_id, owner_id, fields) 
		VALUES (?, ?, ?, ?)
		RETURNING record_id`

	deleteSakeRecordQuery = `
		DELETE FROM sake_records 
		WHERE game_id = ? 
		  AND table_id = ? 
		  AND record_id = ? 
		  AND owner_id = ?`

	checkMaxSakeRecordsQuery = `
		SELECT COUNT(*) 
		FROM sake_records 
		WHERE owner_id = ?`
)

var (
	ErrSakeNotOwned           = errors.New("record is not owned by the specified owner ID")
	ErrSakeFieldLimitExceeded = errors.New("record has too many fields")
)

var _ = common.MaybeUnused(deleteSakeRecordQuery)

func parseSakeFieldsFromJson(fieldsJson []byte) (map[string]SakeField, error) {
	var fields map[string]SakeField
	err := json.Unmarshal(fieldsJson, &fields)
	if err != nil {
		return nil, err
	}

	return fields, nil
}

func (c *Connection) GetSakeRecords(gameId int, ownerIds []int32, tableId string, recordIds []int32, fields []string, filterExpr string) ([]SakeRecord, error) {
	if fields == nil {
		fields = []string{}
	}
	common.MaybeUnused(fields)
	if ownerIds == nil {
		ownerIds = []int32{}
	}
	if recordIds == nil {
		recordIds = []int32{}
	}

	query := getSakeRecordsQuery
	args := []any{gameId, tableId}
	if len(recordIds) > 0 {
		query += " AND record_id IN (?" + strings.Repeat(", ?", len(recordIds)-1) + ")"
		for _, id := range recordIds {
			args = append(args, id)
		}
	}
	if len(ownerIds) > 0 {
		query += " AND owner_id IN (?" + strings.Repeat(", ?", len(ownerIds)-1) + ")"
		for _, id := range ownerIds {
			args = append(args, id)
		}
	}
	if filterExpr != "" {
		tree, err := filter.Parse(filterExpr)
		if err != nil {
			return nil, err
		}

		filterQuery, filterArgs, err := createSqlFilter(tree)
		if err != nil {
			return nil, err
		}

		// This filter has been entirely rewritten by our filter code,
		// based on the expression supplied by the user. This should be safe!!!
		query += " AND (" + filterQuery + ")"
		args = append(args, filterArgs...)
	}

	rows, err := c.pool.QueryContext(c.ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []SakeRecord
	for rows.Next() {
		record := SakeRecord{
			GameId:  gameId,
			TableId: tableId,
		}
		var fieldsJson []byte
		if err := rows.Scan(&record.OwnerId, &record.RecordId, &fieldsJson); err != nil {
			return nil, err
		}
		fields, err := parseSakeFieldsFromJson(fieldsJson)
		if err != nil {
			return nil, err
		}
		record.Fields = fields

		records = append(records, record)
	}

	return records, nil
}

func (c *Connection) UpdateSakeRecord(record SakeRecord, ownerId int32) error {
	fieldsJson, err := json.Marshal(record.Fields)
	if err != nil {
		return err
	}
	var existingOwnerId int32
	err = c.pool.QueryRowContext(c.ctx, updateSakeRecordQuery, ownerId, fieldsJson, ownerId, record.GameId, record.TableId, record.RecordId).Scan(&existingOwnerId)
	if err != nil {
		if mysqlerrnum.FromError(err) == mysqlerrnum.ErrCheckConstraintViolated {
			return ErrSakeFieldLimitExceeded
		}
		return err
	}
	if ownerId != 0 && existingOwnerId != ownerId {
		return ErrSakeNotOwned
	}

	return nil
}

func (c *Connection) InsertSakeRecord(record SakeRecord) (recordId int32, err error) {
	fieldsJson, err := json.Marshal(record.Fields)
	if err != nil {
		return 0, err
	}

	for i := 0; i < 10; i++ {
		err = c.pool.QueryRowContext(c.ctx, insertSakeRecordQuery, record.GameId, record.TableId, record.OwnerId, fieldsJson).Scan(&recordId)
		if err == nil {
			break
		}
		if mysqlerrnum.FromError(err) != mysqlerrnum.ErrDupUnique {
			break
		}
		// Retry if unique violation occurred, as the record ID is generated randomly
	}
	return recordId, err
}

func (c *Connection) IsMaxSakeRecordsReached(profileId uint32, maxRecords int) (bool, error) {
	var count int
	err := c.pool.QueryRowContext(c.ctx, checkMaxSakeRecordsQuery, profileId).Scan(&count)
	if err != nil {
		return false, err
	}
	return count >= maxRecords, nil
}
