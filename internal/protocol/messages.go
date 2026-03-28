package protocol

// RegisterRequest is sent by the agent during initial registration.
type RegisterRequest struct {
	Hostname    string `msgpack:"hostname"`
	Username    string `msgpack:"username"`
	OS          string `msgpack:"os"`
	Arch        string `msgpack:"arch"`
	PID         int    `msgpack:"pid"`
	ProcessName string `msgpack:"process_name"`
	InternalIP  string `msgpack:"internal_ip"`
}

// RegisterResponse is returned by the server after successful registration.
type RegisterResponse struct {
	AgentID string `msgpack:"agent_id"`
	Name    string `msgpack:"name"`
	Sleep   int    `msgpack:"sleep"`
	Jitter  int    `msgpack:"jitter"`
}

// CheckInRequest is sent by the agent on each check-in.
type CheckInRequest struct {
	AgentID string        `msgpack:"agent_id"`
	Results []TaskResult  `msgpack:"results,omitempty"`
}

// CheckInResponse is returned by the server with pending tasks.
type CheckInResponse struct {
	Tasks []Task `msgpack:"tasks"`
}

// Task represents a task assigned to an agent.
type Task struct {
	ID   string   `msgpack:"id"`
	Type uint8    `msgpack:"type"`
	Args []string `msgpack:"args,omitempty"`
	Data []byte   `msgpack:"data,omitempty"`
}

// TaskResult is sent by the agent after executing a task.
type TaskResult struct {
	TaskID  string `msgpack:"task_id"`
	AgentID string `msgpack:"agent_id"`
	Output  []byte `msgpack:"output,omitempty"`
	Error   string `msgpack:"error,omitempty"`
}
