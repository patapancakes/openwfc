package database

import (
	"time"
)

const (
	GetFriends = `
		SELECT f.sender, f.authorized, f.created
		FROM friends AS f
		LEFT JOIN friends AS r
		ON r.sender = f.recipient
		AND r.recipient = f.sender
		WHERE f.recipient = ?`
	GetFriendsOutgoing = `SELECT recipient, authorized, created FROM friends WHERE sender = ?`
	GetFriendAuth      = `SELECT authorized FROM friends WHERE sender = ? AND recipient = ?`
	SetFriendAuth      = `UPDATE friends SET authorized = ? WHERE sender = ? AND recipient = ?`
	AddFriend          = `INSERT INTO friends (sender, recipient) VALUES (?, ?)`
	RemoveFriend       = `DELETE FROM friends WHERE sender = ? AND recipient = ?`
)

type FriendInfo struct {
	ID         uint32
	Authorized bool
	Created    time.Time
}

func (c *Connection) GetFriends(profileId uint32, outgoing bool) ([]FriendInfo, error) {
	q := GetFriends
	if outgoing {
		q = GetFriendsOutgoing
	}

	rows, err := c.pool.QueryContext(c.ctx, q, profileId)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var friends []FriendInfo
	for rows.Next() {
		var friend FriendInfo
		err := rows.Scan(&friend.ID, &friend.Authorized, &friend.Created)
		if err != nil {
			return nil, err
		}

		friends = append(friends, friend)
	}

	return friends, nil
}

func (c *Connection) GetFriendAuth(sender uint32, recipient uint32) (bool, error) {
	var authorized bool
	err := c.pool.QueryRowContext(c.ctx, GetFriendAuth, sender, recipient).Scan(&authorized)
	if err != nil {
		return false, err
	}

	return authorized, nil
}

func (c *Connection) AuthFriend(sender uint32, recipient uint32) error {
	_, err := c.pool.ExecContext(c.ctx, SetFriendAuth, true, sender, recipient)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) AddFriend(sender uint32, recipient uint32) error {
	_, err := c.pool.ExecContext(c.ctx, AddFriend, sender, recipient)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) RemoveFriend(sender uint32, recipient uint32) (bool, error) {
	res, err := c.pool.ExecContext(c.ctx, RemoveFriend, sender, recipient)
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}

	return true, nil
}
