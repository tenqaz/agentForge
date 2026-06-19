package agents

// Delete failure codes are written to agent.last_error_code when a
// deletion attempt fails at a specific stage. Frontend clients use these
// to surface targeted error messages and suggest retries.
const (
	DeleteFailureInspect = "delete_inspect_failed"
	DeleteFailureStop    = "delete_stop_failed"
	DeleteFailureRemove  = "delete_remove_failed"
	DeleteFailureHome    = "delete_home_failed"
)
