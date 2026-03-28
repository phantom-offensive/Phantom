package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// TaskRecord represents a task in the database.
type TaskRecord struct {
	ID          string
	AgentID     string
	Type        int
	Args        []string
	Data        []byte
	Status      int
	CreatedAt   time.Time
	SentAt      *time.Time
	CompletedAt *time.Time
}

// TaskResultRecord represents a task result in the database.
type TaskResultRecord struct {
	TaskID     string
	AgentID    string
	Output     []byte
	Error      string
	ReceivedAt time.Time
}

// InsertTask adds a new task.
func (db *Database) InsertTask(t *TaskRecord) error {
	argsJSON, err := json.Marshal(t.Args)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		INSERT INTO tasks (id, agent_id, type, args, data, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.AgentID, t.Type, string(argsJSON), t.Data, t.Status, t.CreatedAt,
	)
	return err
}

// GetPendingTasks returns all pending tasks for an agent.
func (db *Database) GetPendingTasks(agentID string) ([]*TaskRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, agent_id, type, args, data, status, created_at
		FROM tasks WHERE agent_id = ? AND status = 0
		ORDER BY created_at ASC`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*TaskRecord
	for rows.Next() {
		t := &TaskRecord{}
		var argsJSON string
		if err := rows.Scan(&t.ID, &t.AgentID, &t.Type, &argsJSON, &t.Data, &t.Status, &t.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(argsJSON), &t.Args)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetTasksByAgent returns all tasks for an agent.
func (db *Database) GetTasksByAgent(agentID string) ([]*TaskRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, agent_id, type, args, data, status, created_at, sent_at, completed_at
		FROM tasks WHERE agent_id = ?
		ORDER BY created_at DESC`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*TaskRecord
	for rows.Next() {
		t := &TaskRecord{}
		var argsJSON string
		if err := rows.Scan(&t.ID, &t.AgentID, &t.Type, &argsJSON, &t.Data, &t.Status, &t.CreatedAt, &t.SentAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(argsJSON), &t.Args)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// UpdateTaskStatus updates a task's status and timestamp.
func (db *Database) UpdateTaskStatus(id string, status int) error {
	now := time.Now()
	switch status {
	case 1: // sent
		_, err := db.conn.Exec(`UPDATE tasks SET status = ?, sent_at = ? WHERE id = ?`, status, now, id)
		return err
	case 2, 3: // complete or error
		_, err := db.conn.Exec(`UPDATE tasks SET status = ?, completed_at = ? WHERE id = ?`, status, now, id)
		return err
	default:
		_, err := db.conn.Exec(`UPDATE tasks SET status = ? WHERE id = ?`, status, id)
		return err
	}
}

// InsertTaskResult stores a task result.
func (db *Database) InsertTaskResult(r *TaskResultRecord) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO task_results (task_id, agent_id, output, error, received_at)
		VALUES (?, ?, ?, ?, ?)`,
		r.TaskID, r.AgentID, r.Output, r.Error, r.ReceivedAt,
	)
	return err
}

// GetTaskResult retrieves the result for a task.
func (db *Database) GetTaskResult(taskID string) (*TaskResultRecord, error) {
	r := &TaskResultRecord{}
	err := db.conn.QueryRow(`SELECT task_id, agent_id, output, error, received_at FROM task_results WHERE task_id = ?`, taskID).
		Scan(&r.TaskID, &r.AgentID, &r.Output, &r.Error, &r.ReceivedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}
