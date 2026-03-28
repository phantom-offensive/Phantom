package db

import "time"

// LootRecord represents captured data (files, screenshots, credentials).
type LootRecord struct {
	ID        string
	AgentID   string
	TaskID    string
	Type      string // "file", "screenshot", "credential"
	Name      string
	Data      []byte
	CreatedAt time.Time
}

// InsertLoot stores captured loot.
func (db *Database) InsertLoot(l *LootRecord) error {
	_, err := db.conn.Exec(`
		INSERT INTO loot (id, agent_id, task_id, type, name, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.AgentID, l.TaskID, l.Type, l.Name, l.Data, l.CreatedAt,
	)
	return err
}

// ListLoot returns all loot, optionally filtered by agent.
func (db *Database) ListLoot(agentID string) ([]*LootRecord, error) {
	var query string
	var args []interface{}

	if agentID != "" {
		query = `SELECT id, agent_id, task_id, type, name, length(data), created_at FROM loot WHERE agent_id = ? ORDER BY created_at DESC`
		args = append(args, agentID)
	} else {
		query = `SELECT id, agent_id, task_id, type, name, length(data), created_at FROM loot ORDER BY created_at DESC`
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loot []*LootRecord
	for rows.Next() {
		l := &LootRecord{}
		var dataLen int
		if err := rows.Scan(&l.ID, &l.AgentID, &l.TaskID, &l.Type, &l.Name, &dataLen, &l.CreatedAt); err != nil {
			return nil, err
		}
		loot = append(loot, l)
	}
	return loot, rows.Err()
}

// GetLoot retrieves a specific loot record with its data.
func (db *Database) GetLoot(id string) (*LootRecord, error) {
	l := &LootRecord{}
	err := db.conn.QueryRow(`SELECT id, agent_id, task_id, type, name, data, created_at FROM loot WHERE id = ?`, id).
		Scan(&l.ID, &l.AgentID, &l.TaskID, &l.Type, &l.Name, &l.Data, &l.CreatedAt)
	return l, err
}
