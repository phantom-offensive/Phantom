package db

import (
	"database/sql"
	"time"
)

// ListenerRecord represents a listener configuration in the database.
type ListenerRecord struct {
	ID        string
	Name      string
	Type      string
	BindAddr  string
	ProfileID string
	TLSCert   string
	TLSKey    string
	Status    string
	CreatedAt time.Time
}

// InsertListener adds a new listener record.
func (db *Database) InsertListener(l *ListenerRecord) error {
	_, err := db.conn.Exec(`
		INSERT INTO listeners (id, name, type, bind_addr, profile_id, tls_cert, tls_key, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.Name, l.Type, l.BindAddr, l.ProfileID, l.TLSCert, l.TLSKey, l.Status, l.CreatedAt,
	)
	return err
}

// ListListeners returns all listener records.
func (db *Database) ListListeners() ([]*ListenerRecord, error) {
	rows, err := db.conn.Query(`SELECT id, name, type, bind_addr, profile_id, tls_cert, tls_key, status, created_at FROM listeners ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var listeners []*ListenerRecord
	for rows.Next() {
		l := &ListenerRecord{}
		if err := rows.Scan(&l.ID, &l.Name, &l.Type, &l.BindAddr, &l.ProfileID, &l.TLSCert, &l.TLSKey, &l.Status, &l.CreatedAt); err != nil {
			return nil, err
		}
		listeners = append(listeners, l)
	}
	return listeners, rows.Err()
}

// GetListenerByName retrieves a listener by name.
func (db *Database) GetListenerByName(name string) (*ListenerRecord, error) {
	l := &ListenerRecord{}
	err := db.conn.QueryRow(`SELECT id, name, type, bind_addr, profile_id, tls_cert, tls_key, status, created_at FROM listeners WHERE name = ?`, name).
		Scan(&l.ID, &l.Name, &l.Type, &l.BindAddr, &l.ProfileID, &l.TLSCert, &l.TLSKey, &l.Status, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

// UpdateListenerStatus updates a listener's status (running/stopped).
func (db *Database) UpdateListenerStatus(id, status string) error {
	_, err := db.conn.Exec(`UPDATE listeners SET status = ? WHERE id = ?`, status, id)
	return err
}

// DeleteListener removes a listener record.
func (db *Database) DeleteListener(id string) error {
	_, err := db.conn.Exec(`DELETE FROM listeners WHERE id = ?`, id)
	return err
}
