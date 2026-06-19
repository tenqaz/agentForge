package agents

import (
	"context"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRepositoryDeleteRemovesRow(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusRunning)

	if err := repository.Delete(ctx, "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := repository.Get(ctx, "agent-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete err = %v, want ErrNotFound", err)
	}
}

func TestRepositoryDeleteCascadesRuntimeJobs(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusRunning)
	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'restart_runtime', 'succeeded');
	`); err != nil {
		t.Fatalf("seed runtime job: %v", err)
	}

	if err := repository.Delete(ctx, "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM runtime_jobs WHERE agent_id = ?;`, "agent-1").Scan(&count); err != nil {
		t.Fatalf("count runtime_jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("runtime_jobs count after Delete = %d, want 0", count)
	}
}

func TestRepositoryDeleteReturnsNotFoundForMissingAgent(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()

	if err := repository.Delete(ctx, "agent-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestRepositoryMarkDeleteFailedWritesErrorState(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusStopped)

	if err := repository.MarkDeleteFailed(ctx, "agent-1", DeleteFailureStop, "docker stop failed: timeout"); err != nil {
		t.Fatalf("MarkDeleteFailed returned error: %v", err)
	}

	agent, err := repository.Get(ctx, "agent-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if agent.Status != StatusError {
		t.Fatalf("agent.Status = %s, want %s", agent.Status, StatusError)
	}
	if agent.LastErrorCode != DeleteFailureStop {
		t.Fatalf("agent.LastErrorCode = %q, want %q", agent.LastErrorCode, DeleteFailureStop)
	}
	if agent.LastErrorMessage != "docker stop failed: timeout" {
		t.Fatalf("agent.LastErrorMessage = %q, unexpected", agent.LastErrorMessage)
	}
}

func TestRepositoryMarkDeleteFailedReturnsNotFound(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()

	err := repository.MarkDeleteFailed(ctx, "agent-missing", DeleteFailureStop, "x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkDeleteFailed err = %v, want ErrNotFound", err)
	}
}

func TestRepositoryMarkDeleteFailedBypassesStateMachine(t *testing.T) {
	// stopped -> error is NOT in transitions table; MarkDeleteFailed must
	// still succeed because deletion failures bypass the normal state machine.
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusStopped)

	if err := repository.MarkDeleteFailed(ctx, "agent-1", DeleteFailureHome, "rm failed"); err != nil {
		t.Fatalf("MarkDeleteFailed should bypass state machine, got err: %v", err)
	}
	agent, _ := repository.Get(ctx, "agent-1")
	if agent.Status != StatusError {
		t.Fatalf("status = %s, want error", agent.Status)
	}
}
