package agents

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"

	_ "modernc.org/sqlite"
)

// fakeRunner is a controllable mock of runtime.Runner used by Delete tests.
type fakeRunner struct {
	mu            sync.Mutex
	inspectStatus runtime.ContainerStatus
	inspectErr    error
	stopErr       error
	removeErr     error
	stopCalls     int
	removeCalls   int
	inspectCalls  int
}

func (r *fakeRunner) EnsureRunning(ctx context.Context, spec runtime.ContainerSpec) error {
	return errors.New("not implemented in fakeRunner")
}

func (r *fakeRunner) Inspect(ctx context.Context, containerName string) (runtime.ContainerStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inspectCalls++
	return r.inspectStatus, r.inspectErr
}

func (r *fakeRunner) Stop(ctx context.Context, containerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCalls++
	return r.stopErr
}

func (r *fakeRunner) Remove(ctx context.Context, containerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeCalls++
	return r.removeErr
}

// newServiceForDelete builds a Service against an in-memory DB pre-seeded
// with one agent in the requested state. dataDir is set to a temp dir so
// the agent's hermes_home_path resolves under it.
func newServiceForDelete(t *testing.T, agentID string, status Status, runner runtime.Runner) (*Service, *sql.DB, string) {
	t.Helper()
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	dataDir := t.TempDir()
	homePath := filepath.Join(dataDir, "agents", agentID, "hermes-home")
	insertAgentFixtureWithHome(t, database, agentID, "user-1", status, homePath)
	runtimeJobs := jobs.NewRuntimeRepository(database)
	svc := NewService(database, repository, runtimeJobs, runner, dataDir)
	return svc, database, homePath
}

// insertAgentFixtureWithHome is a helper similar to insertAgentFixture but
// allows setting hermes_home_path explicitly.
func insertAgentFixtureWithHome(t *testing.T, database *sql.DB, agentID, ownerUserID string, status Status, homePath string) {
	t.Helper()
	if _, err := database.ExecContext(context.Background(), `
		INSERT INTO agents (id, owner_user_id, template_id, template_version, name, status, hermes_home_path)
		VALUES (?, ?, 'template-1', 3, 'Test Agent', ?, ?);
	`, agentID, ownerUserID, status, homePath); err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
}

func TestDeleteRefusesProvisioning(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusProvisioning, runner)

	err := svc.Delete(context.Background(), "agent-1")
	if !errors.Is(err, ErrCannotDelete) {
		t.Fatalf("Delete err = %v, want ErrCannotDelete", err)
	}
	if runner.stopCalls != 0 || runner.removeCalls != 0 {
		t.Fatalf("runner should not be called when refused; stop=%d remove=%d", runner.stopCalls, runner.removeCalls)
	}
}

func TestDeleteRefusesStarting(t *testing.T) {
	runner := &fakeRunner{}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusStarting, runner)

	err := svc.Delete(context.Background(), "agent-1")
	if !errors.Is(err, ErrCannotDelete) {
		t.Fatalf("Delete err = %v, want ErrCannotDelete", err)
	}
}

func TestDeleteRefusesWhenUnfinishedJobExists(t *testing.T) {
	runner := &fakeRunner{}
	svc, db, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES ('job-1', 'agent-1', 'restart_runtime', 'running');
	`); err != nil {
		t.Fatalf("seed running job: %v", err)
	}

	err := svc.Delete(context.Background(), "agent-1")
	if !errors.Is(err, ErrHasUnfinishedJobs) {
		t.Fatalf("Delete err = %v, want ErrHasUnfinishedJobs", err)
	}
}

func TestDeleteReturnsNotFoundForMissingAgent(t *testing.T) {
	runner := &fakeRunner{}
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	runtimeJobs := jobs.NewRuntimeRepository(database)
	svc := NewService(database, repository, runtimeJobs, runner, t.TempDir())

	err := svc.Delete(context.Background(), "agent-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestDeleteSucceedsForRunningAgent(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: true},
	}
	svc, db, homePath := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	if err := os.MkdirAll(homePath, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homePath, "USER.md"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if runner.stopCalls != 1 {
		t.Errorf("stop calls = %d, want 1", runner.stopCalls)
	}
	if runner.removeCalls != 1 {
		t.Errorf("remove calls = %d, want 1", runner.removeCalls)
	}
	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Errorf("hermes home should be removed, stat err = %v", err)
	}
	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM agents WHERE id = ?;`, "agent-1").Scan(&count); err != nil {
		t.Fatalf("count agents: %v", err)
	}
	if count != 0 {
		t.Errorf("agents count after Delete = %d, want 0", count)
	}
}

func TestDeleteSkipsStopForStoppedContainer(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: false},
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusStopped, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if runner.stopCalls != 0 {
		t.Errorf("stop should not be called when container is not running, got %d calls", runner.stopCalls)
	}
	if runner.removeCalls != 1 {
		t.Errorf("remove calls = %d, want 1", runner.removeCalls)
	}
}

func TestDeleteSucceedsWhenContainerAlreadyMissing(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusError, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if runner.stopCalls != 0 || runner.removeCalls != 0 {
		t.Errorf("expected no stop/remove calls; got stop=%d remove=%d", runner.stopCalls, runner.removeCalls)
	}
}

func TestDeleteToleratesRemoveRace(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: false},
		removeErr:     runtime.ErrContainerNotFound,
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete should swallow concurrent remove race, got: %v", err)
	}
}

func TestDeleteSucceedsForCreatingAgent(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusCreating, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestDeleteSucceedsWhenHomeDirectoryAlreadyMissing(t *testing.T) {
	runner := &fakeRunner{inspectErr: runtime.ErrContainerNotFound}
	svc, _, homePath := newServiceForDelete(t, "agent-1", StatusError, runner)

	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Fatalf("expected home dir absent before test, err=%v", err)
	}

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("Delete with missing home returned error: %v", err)
	}
}

func TestDeleteFailsAndMarksErrorWhenInspectFails(t *testing.T) {
	runner := &fakeRunner{inspectErr: errors.New("docker inspect failed: bad output")}
	svc, db, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	repository := NewRepository(db)

	err := svc.Delete(context.Background(), "agent-1")
	if err == nil {
		t.Fatal("Delete should return error when Inspect fails")
	}
	agent, getErr := repository.Get(context.Background(), "agent-1")
	if getErr != nil {
		t.Fatalf("Get after failed delete: %v", getErr)
	}
	if agent.Status != StatusError {
		t.Errorf("agent status = %s, want error", agent.Status)
	}
	if agent.LastErrorCode != DeleteFailureInspect {
		t.Errorf("last_error_code = %q, want %q", agent.LastErrorCode, DeleteFailureInspect)
	}
}

func TestDeleteFailsAndMarksErrorWhenStopFails(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: true},
		stopErr:       errors.New("docker stop failed: signal denied"),
	}
	svc, db, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)
	repository := NewRepository(db)

	if err := svc.Delete(context.Background(), "agent-1"); err == nil {
		t.Fatal("Delete should fail when Stop fails")
	}
	agent, _ := repository.Get(context.Background(), "agent-1")
	if agent.Status != StatusError || agent.LastErrorCode != DeleteFailureStop {
		t.Errorf("agent state = (%s, %q), want (error, %s)", agent.Status, agent.LastErrorCode, DeleteFailureStop)
	}
}

func TestDeleteFailsAndMarksErrorWhenRemoveFailsNonNotFound(t *testing.T) {
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: false},
		removeErr:     errors.New("docker rm failed: permission denied"),
	}
	svc, db, _ := newServiceForDelete(t, "agent-1", StatusStopped, runner)
	repository := NewRepository(db)

	if err := svc.Delete(context.Background(), "agent-1"); err == nil {
		t.Fatal("Delete should fail when Remove fails")
	}
	agent, _ := repository.Get(context.Background(), "agent-1")
	if agent.LastErrorCode != DeleteFailureRemove {
		t.Errorf("last_error_code = %q, want %q", agent.LastErrorCode, DeleteFailureRemove)
	}
}

func TestDeleteRetriesAfterStopFailure(t *testing.T) {
	// First call: Stop fails. Second call: container already gone, Inspect
	// returns NotFound and Delete completes the cleanup.
	runner := &fakeRunner{
		inspectStatus: runtime.ContainerStatus{Exists: true, Running: true},
		stopErr:       errors.New("transient stop failure"),
	}
	svc, _, _ := newServiceForDelete(t, "agent-1", StatusRunning, runner)

	if err := svc.Delete(context.Background(), "agent-1"); err == nil {
		t.Fatal("first Delete should fail")
	}

	// Simulate operator (or natural state) cleaning the container.
	runner.mu.Lock()
	runner.inspectErr = runtime.ErrContainerNotFound
	runner.inspectStatus = runtime.ContainerStatus{}
	runner.stopErr = nil
	runner.mu.Unlock()

	if err := svc.Delete(context.Background(), "agent-1"); err != nil {
		t.Fatalf("retry Delete returned error: %v", err)
	}
}
