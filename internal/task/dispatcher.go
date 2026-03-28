package task

import (
	"time"

	"github.com/google/uuid"
	"github.com/phantom-c2/phantom/internal/db"
	"github.com/phantom-c2/phantom/internal/protocol"
)

// Dispatcher manages task creation, queuing, and result processing.
type Dispatcher struct {
	database *db.Database
}

// NewDispatcher creates a new task dispatcher.
func NewDispatcher(database *db.Database) *Dispatcher {
	return &Dispatcher{database: database}
}

// CreateTask creates a new task for an agent.
func (d *Dispatcher) CreateTask(agentID string, taskType uint8, args []string, data []byte) (*db.TaskRecord, error) {
	task := &db.TaskRecord{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		Type:      int(taskType),
		Args:      args,
		Data:      data,
		Status:    int(protocol.StatusPending),
		CreatedAt: time.Now(),
	}

	if err := d.database.InsertTask(task); err != nil {
		return nil, err
	}

	return task, nil
}

// GetPendingTasks retrieves all pending tasks for an agent and marks them as sent.
func (d *Dispatcher) GetPendingTasks(agentID string) ([]protocol.Task, error) {
	records, err := d.database.GetPendingTasks(agentID)
	if err != nil {
		return nil, err
	}

	var tasks []protocol.Task
	for _, r := range records {
		tasks = append(tasks, protocol.Task{
			ID:   r.ID,
			Type: uint8(r.Type),
			Args: r.Args,
			Data: r.Data,
		})

		// Mark as sent
		d.database.UpdateTaskStatus(r.ID, int(protocol.StatusSent))
	}

	return tasks, nil
}

// ProcessResult stores a task result and updates task status.
func (d *Dispatcher) ProcessResult(result *protocol.TaskResult) error {
	status := int(protocol.StatusComplete)
	if result.Error != "" {
		status = int(protocol.StatusError)
	}

	// Store result
	record := &db.TaskResultRecord{
		TaskID:     result.TaskID,
		AgentID:    result.AgentID,
		Output:     result.Output,
		Error:      result.Error,
		ReceivedAt: time.Now(),
	}

	if err := d.database.InsertTaskResult(record); err != nil {
		return err
	}

	return d.database.UpdateTaskStatus(result.TaskID, status)
}

// GetTaskHistory returns all tasks for an agent.
func (d *Dispatcher) GetTaskHistory(agentID string) ([]*db.TaskRecord, error) {
	return d.database.GetTasksByAgent(agentID)
}

// GetResult retrieves the result for a specific task.
func (d *Dispatcher) GetResult(taskID string) (*db.TaskResultRecord, error) {
	return d.database.GetTaskResult(taskID)
}
