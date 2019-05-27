package storage

import (
	// This line is must for working MySQL database
	_ "github.com/go-sql-driver/mysql"
	"github.com/maddevsio/tgsbot/model"
)

// CreateGroup creates Group
func (m *MySQL) CreateGroup(group *model.Group) (*model.Group, error) {
	res, err := m.conn.Exec(
		"INSERT INTO `groups` (chat_id, title, username, description, standup_deadline, tz) VALUES (?, ?, ?, ?, ?, ?)",
		group.ChatID, group.Title, group.Username, group.Description, group.StandupDeadline, group.TZ,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	group.ID = id
	return group, nil
}

// UpdateGroup updates Group entry in database
func (m *MySQL) UpdateGroup(group *model.Group) (*model.Group, error) {
	m.conn.Exec(
		"UPDATE `groups` SET title=?, tz=?, username=?, description=?, standup_deadline=? WHERE id=?",
		group.Title, group.TZ, group.Username, group.Description, group.StandupDeadline, group.ID,
	)
	err := m.conn.Get(group, "SELECT * FROM `groups` WHERE id=?", group.ID)
	return group, err
}

// SelectGroup selects Group entry from database
func (m *MySQL) SelectGroup(id int64) (*model.Group, error) {
	group := &model.Group{}
	err := m.conn.Get(group, "SELECT * FROM `groups` WHERE id=?", id)
	return group, err
}

// FindGroup selects Group entry from database
func (m *MySQL) FindGroup(chatID int64) (*model.Group, error) {
	group := &model.Group{}
	err := m.conn.Get(group, "SELECT * FROM `groups` WHERE chat_id=?", chatID)
	return group, err
}

// ListGroups returns array of Group entries from database filtered by chat
func (m *MySQL) ListGroups() ([]*model.Group, error) {
	groups := []*model.Group{}
	err := m.conn.Select(&groups, "SELECT * FROM `groups`")
	return groups, err
}

// DeleteGroup deletes Group entry from database
func (m *MySQL) DeleteGroup(id int64) error {
	_, err := m.conn.Exec("DELETE FROM `groups` WHERE id=?", id)
	return err
}