package jobs

import (
	"context"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHasUnfinishedByAgentReturnsTrueForQueued(t *testing.T) {
	database := newRuntimeJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'provision_agent', 'queued');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if !has {
		t.Fatal("expected has=true for queued job, got false")
	}
}

func TestHasUnfinishedByAgentReturnsTrueForRunning(t *testing.T) {
	database := newRuntimeJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'restart_runtime', 'running');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if !has {
		t.Fatal("expected has=true for running job, got false")
	}
}

func TestHasUnfinishedByAgentReturnsFalseForFinishedJobs(t *testing.T) {
	database := newRuntimeJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status) VALUES
			('job-1', 'agent-1', 'provision_agent', 'succeeded'),
			('job-2', 'agent-1', 'restart_runtime', 'failed'),
			('job-3', 'agent-1', 'restart_runtime', 'cancelled');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if has {
		t.Fatal("expected has=false when only finished jobs exist")
	}
}

func TestHasUnfinishedByAgentScopesByAgentID(t *testing.T) {
	database := newRuntimeJobsTestDB(t)
	repo := NewRuntimeRepository(database)
	ctx := context.Background()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-2', 'provision_agent', 'queued');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	has, err := repo.HasUnfinishedByAgent(ctx, "agent-1")
	if err != nil {
		t.Fatalf("HasUnfinishedByAgent: %v", err)
	}
	if has {
		t.Fatal("expected has=false when only other agent's jobs are queued")
	}
}
