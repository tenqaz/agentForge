package jobs

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSupervisorClaimsAndProcessesRuntimeJob(t *testing.T) {
	database := newSupervisorTestDB(t)
	repository := NewRuntimeRepository(database)
	ctx := context.Background()
	insertSupervisorRuntimeJob(t, database, "job-runtime-1", "agent-1", TypeProvisionAgent)

	worker := &stubRuntimeJobProcessor{}
	supervisor := NewSupervisor(SupervisorDependencies{
		RuntimeJobs:   repository,
		ChannelJobs:   NewChannelRepository(database),
		RuntimeWorker: worker,
		ChannelWorker: &stubChannelJobProcessor{},
		WorkerID:      "worker-1",
		PollInterval:  time.Millisecond,
		LeaseTTL:      20 * time.Millisecond,
	})

	worked, err := supervisor.runOnce(ctx)
	if err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if !worked {
		t.Fatal("runOnce worked = false, want true")
	}
	if len(worker.calls) != 1 || worker.calls[0] != "job-runtime-1" {
		t.Fatalf("runtime worker calls = %#v", worker.calls)
	}

	job, err := repository.GetByID(ctx, "agent-1", "job-runtime-1")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if job.Status != StatusRunning || job.LockedBy != "worker-1" {
		t.Fatalf("job = %#v", job)
	}
}

func TestSupervisorClaimsAndProcessesChannelJob(t *testing.T) {
	database := newSupervisorTestDB(t)
	repository := NewChannelRepository(database)
	ctx := context.Background()
	insertSupervisorChannelJob(t, database, "job-channel-1", "channel-1", TypeConnectWeixin)

	worker := &stubChannelJobProcessor{}
	supervisor := NewSupervisor(SupervisorDependencies{
		RuntimeJobs:   NewRuntimeRepository(database),
		ChannelJobs:   repository,
		RuntimeWorker: &stubRuntimeJobProcessor{},
		ChannelWorker: worker,
		WorkerID:      "worker-1",
		PollInterval:  time.Millisecond,
		LeaseTTL:      20 * time.Millisecond,
	})

	worked, err := supervisor.runOnce(ctx)
	if err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if !worked {
		t.Fatal("runOnce worked = false, want true")
	}
	if len(worker.calls) != 1 || worker.calls[0] != "job-channel-1" {
		t.Fatalf("channel worker calls = %#v", worker.calls)
	}

	job, err := repository.GetByID(ctx, "channel-1", "job-channel-1")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if job.Status != StatusRunning || job.LockedBy != "worker-1" {
		t.Fatalf("job = %#v", job)
	}
}

func TestSupervisorExtendsLeaseWhileChannelJobRuns(t *testing.T) {
	database := newSupervisorTestDB(t)
	repository := NewChannelRepository(database)
	ctx := context.Background()
	insertSupervisorChannelJob(t, database, "job-channel-lease", "channel-1", TypeConnectWeixin)

	started := make(chan struct{})
	release := make(chan struct{})
	worker := &stubChannelJobProcessor{
		process: func(_ context.Context, _ string) error {
			close(started)
			<-release
			return nil
		},
	}
	supervisor := NewSupervisor(SupervisorDependencies{
		RuntimeJobs:   NewRuntimeRepository(database),
		ChannelJobs:   repository,
		RuntimeWorker: &stubRuntimeJobProcessor{},
		ChannelWorker: worker,
		WorkerID:      "worker-1",
		PollInterval:  time.Millisecond,
		LeaseTTL:      20 * time.Millisecond,
	})
	job, err := repository.ClaimNextQueued(ctx, "worker-1", time.Now().Add(20*time.Millisecond))
	if err != nil {
		t.Fatalf("ClaimNextQueued returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- supervisor.processChannelJob(ctx, job)
	}()

	<-started
	time.Sleep(25 * time.Millisecond)

	job, err = repository.GetByID(ctx, "channel-1", "job-channel-lease")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if job.LockedUntil == nil {
		t.Fatalf("job lease was not extended: %#v", job)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("processChannelJob returned error: %v", err)
	}
}

func TestSupervisorMarksStableErrorCodeOnRuntimeFailure(t *testing.T) {
	database := newSupervisorTestDB(t)
	repository := NewRuntimeRepository(database)
	ctx := context.Background()
	insertSupervisorRuntimeJob(t, database, "job-runtime-fail", "agent-1", TypeProvisionAgent)

	supervisor := NewSupervisor(SupervisorDependencies{
		RuntimeJobs:   repository,
		ChannelJobs:   NewChannelRepository(database),
		RuntimeWorker: &stubRuntimeJobProcessor{err: errors.New("copy_template_failed")},
		ChannelWorker: &stubChannelJobProcessor{},
		WorkerID:      "worker-1",
		PollInterval:  time.Millisecond,
		LeaseTTL:      20 * time.Millisecond,
	})

	worked, err := supervisor.runOnce(ctx)
	if err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if !worked {
		t.Fatal("runOnce worked = false, want true")
	}

	job, err := repository.GetByID(ctx, "agent-1", "job-runtime-fail")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if job.Status != StatusFailed || job.LastErrorCode != "copy_template_failed" {
		t.Fatalf("job = %#v", job)
	}
}

func TestSupervisorReturnsCleanlyOnContextCancel(t *testing.T) {
	supervisor := NewSupervisor(SupervisorDependencies{
		RuntimeJobs:   &RuntimeRepository{},
		ChannelJobs:   &ChannelRepository{},
		RuntimeWorker: &stubRuntimeJobProcessor{},
		ChannelWorker: &stubChannelJobProcessor{},
		PollInterval:  10 * time.Millisecond,
		LeaseTTL:      20 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := supervisor.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

type stubRuntimeJobProcessor struct {
	calls []string
	err   error
}

func (s *stubRuntimeJobProcessor) ProcessJob(_ context.Context, jobID string) error {
	s.calls = append(s.calls, jobID)
	return s.err
}

type stubChannelJobProcessor struct {
	calls   []string
	err     error
	process func(context.Context, string) error
}

func (s *stubChannelJobProcessor) ProcessJob(ctx context.Context, jobID string) error {
	s.calls = append(s.calls, jobID)
	if s.process != nil {
		return s.process(ctx, jobID)
	}
	return s.err
}

func newSupervisorTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:supervisor-test-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = database.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE agent_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
			version INTEGER NOT NULL DEFAULT 1,
			template_path TEXT NOT NULL,
			content_checksum TEXT NOT NULL,
			soul_md_path TEXT NOT NULL DEFAULT '',
			user_md_path TEXT NOT NULL DEFAULT '',
			soul_content TEXT NOT NULL DEFAULT '',
			user_content TEXT NOT NULL DEFAULT '',
			skills_path TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			published_at TEXT,
			FOREIGN KEY (created_by) REFERENCES users(id)
		);
		CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			template_id TEXT NOT NULL,
			template_version INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'creating' CHECK (status IN ('creating', 'provisioning', 'starting', 'running', 'stopped', 'error')),
			runtime_id TEXT NOT NULL DEFAULT '',
			hermes_home_path TEXT NOT NULL UNIQUE,
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (owner_user_id) REFERENCES users(id),
			FOREIGN KEY (template_id) REFERENCES agent_templates(id)
		);
		CREATE TABLE agent_channels (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			channel_type TEXT NOT NULL DEFAULT 'weixin' CHECK (channel_type IN ('weixin')),
			status TEXT NOT NULL DEFAULT 'not_configured' CHECK (status IN ('not_configured', 'qr_pending', 'connected', 'error', 'disconnected')),
			external_account_id TEXT NOT NULL DEFAULT '',
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		CREATE TABLE channel_pairing_sessions (
			id TEXT PRIMARY KEY,
			agent_channel_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'expired', 'failed')),
			qr_payload TEXT NOT NULL DEFAULT '',
			qr_image_path TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE
		);
		CREATE TABLE runtime_jobs (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			type TEXT NOT NULL CHECK (type IN ('provision_agent', 'start_runtime', 'stop_runtime', 'restart_runtime')),
			status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
			priority INTEGER NOT NULL DEFAULT 0,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 3,
			locked_by TEXT NOT NULL DEFAULT '',
			locked_until TEXT,
			idempotency_key TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '{}',
			result_json TEXT NOT NULL DEFAULT '{}',
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			started_at TEXT,
			finished_at TEXT,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		CREATE TABLE channel_jobs (
			id TEXT PRIMARY KEY,
			agent_channel_id TEXT NOT NULL,
			pairing_session_id TEXT,
			type TEXT NOT NULL CHECK (type IN ('connect_weixin', 'disconnect_weixin', 'refresh_weixin_pairing')),
			status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
			priority INTEGER NOT NULL DEFAULT 0,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 3,
			locked_by TEXT NOT NULL DEFAULT '',
			locked_until TEXT,
			idempotency_key TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '{}',
			result_json TEXT NOT NULL DEFAULT '{}',
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			started_at TEXT,
			finished_at TEXT,
			FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE,
			FOREIGN KEY (pairing_session_id) REFERENCES channel_pairing_sessions(id) ON DELETE SET NULL
		);
		CREATE UNIQUE INDEX idx_runtime_jobs_one_active
		ON runtime_jobs(agent_id)
		WHERE status IN ('queued', 'running');
		CREATE UNIQUE INDEX idx_channel_jobs_one_active
		ON channel_jobs(agent_channel_id)
		WHERE status IN ('queued', 'running');
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-1', 'user@example.com', 'unused', 'user');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_md_path, user_md_path, soul_content, user_content, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 1,
			'/tmp/template-1', 'checksum', '/tmp/template-1/SOUL.md',
			'/tmp/template-1/USER.md', '', '', '/tmp/template-1/skills', 'user-1'
		);
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, hermes_home_path
		) VALUES (
			'agent-1', 'user-1', 'template-1', 1, 'Agent 1', 'running', '/tmp/agent-1'
		);
		INSERT INTO agent_channels (id, agent_id, channel_type, status)
		VALUES ('channel-1', 'agent-1', 'weixin', 'not_configured');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return database
}

func insertSupervisorRuntimeJob(t *testing.T, database *sql.DB, jobID, agentID string, jobType Type) {
	t.Helper()
	_, err := database.Exec(`
		INSERT INTO runtime_jobs (id, agent_id, type, status, locked_by)
		VALUES (?, ?, ?, 'queued', '');
	`, jobID, agentID, jobType)
	if err != nil {
		t.Fatalf("insert runtime job: %v", err)
	}
}

func insertSupervisorChannelJob(t *testing.T, database *sql.DB, jobID, channelID string, jobType ChannelJobType) {
	t.Helper()
	_, err := database.Exec(`
		INSERT INTO channel_jobs (id, agent_channel_id, type, status, locked_by)
		VALUES (?, ?, ?, 'queued', '');
	`, jobID, channelID, jobType)
	if err != nil {
		t.Fatalf("insert channel job: %v", err)
	}
}
