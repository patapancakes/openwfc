package database

import (
	"time"
)

const (
	getFriends = `
		SELECT f.sender, f.authorized, f.created
		FROM friends AS f
		LEFT JOIN friends AS r
		ON r.sender = f.recipient
		AND r.recipient = f.sender
		WHERE f.recipient = ?`
	getFriendsOutgoing = `SELECT recipient, authorized, created FROM friends WHERE sender = ?`
	getFriendAuth      = `SELECT authorized FROM friends WHERE sender = ? AND recipient = ?`
	setFriendAuth      = `UPDATE friends SET authorized = ? WHERE sender = ? AND recipient = ?`
	addFriend          = `INSERT INTO friends (sender, recipient) VALUES (?, ?)`
	removeFriend       = `DELETE FROM friends WHERE sender = ? AND recipient = ?`
)

type FriendInfo struct {
	ID         uint32
	Authorized bool
	Created    time.Time
}

func (c *Connection) GetFriends(profileId uint32, outgoing bool) ([]FriendInfo, error) {
	q := getFriends
	if outgoing {
		q = getFriendsOutgoing
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
	err := c.pool.QueryRowContext(c.ctx, getFriendAuth, sender, recipient).Scan(&authorized)
	if err != nil {
		return false, err
	}

	return authorized, nil
}

func (c *Connection) AuthFriend(sender uint32, recipient uint32) error {
	_, err := c.pool.ExecContext(c.ctx, setFriendAuth, true, sender, recipient)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) AddFriend(sender uint32, recipient uint32) error {
	_, err := c.pool.ExecContext(c.ctx, addFriend, sender, recipient)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) RemoveFriend(sender uint32, recipient uint32) (bool, error) {
	res, err := c.pool.ExecContext(c.ctx, removeFriend, sender, recipient)
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
