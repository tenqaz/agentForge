package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/weixin"
)

// IdleDetector periodically scans running agents and puts idle ones to
// sleep. Idle is defined as:
//   1. .heartbeat exists (gateway is alive),
//   2. gateway_state.json has active_agents == 0 (no in-flight turns), and
//   3. gateway_state.json mtime > idle_timeout (last turn ended long ago),
//   4. iLink has no pending messages (defence against the race where a
//      user sent a message but the gateway hasn't pulled it yet).
type IdleDetector struct {
	agentRepo     *agents.Repository
	channelRepo   *channels.Repository
	weixinClient  weixin.Client
	sleepFunc     func(ctx context.Context, agentID string) error
	idleTimeout   time.Duration
	checkInterval time.Duration
	maxMisses     int

	mu              sync.Mutex
	heartbeatMisses map[string]int // agentID → consecutive .heartbeat misses
}

// IdleDetectorDeps is the dependency injection container for IdleDetector.
type IdleDetectorDeps struct {
	AgentRepo     *agents.Repository
	ChannelRepo   *channels.Repository
	WeixinClient  weixin.Client
	SleepFunc     func(ctx context.Context, agentID string) error
	IdleTimeout   time.Duration
	CheckInterval time.Duration
	MaxMisses     int
}

// NewIdleDetector creates an IdleDetector.  Sensible defaults are applied
// for zero-valued durations / counts.
func NewIdleDetector(deps IdleDetectorDeps) *IdleDetector {
	interval := deps.CheckInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	timeout := deps.IdleTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	misses := deps.MaxMisses
	if misses <= 0 {
		misses = 3
	}
	return &IdleDetector{
		agentRepo:       deps.AgentRepo,
		channelRepo:     deps.ChannelRepo,
		weixinClient:    deps.WeixinClient,
		sleepFunc:       deps.SleepFunc,
		idleTimeout:     timeout,
		checkInterval:   interval,
		maxMisses:       misses,
		heartbeatMisses: map[string]int{},
	}
}

// Run starts the detection loop. It blocks until ctx is cancelled.
func (d *IdleDetector) Run(ctx context.Context) {
	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()

	slog.Info("idle_detector: started",
		"interval", d.checkInterval,
		"idle_timeout", d.idleTimeout,
		"max_misses", d.maxMisses,
	)
	for {
		select {
		case <-ctx.Done():
			slog.Info("idle_detector: stopped")
			return
		case <-ticker.C:
			d.checkOnce(ctx)
		}
	}
}

func (d *IdleDetector) checkOnce(ctx context.Context) {
	runningAgents, err := d.agentRepo.ListByStatus(ctx, agents.StatusRunning)
	if err != nil {
		slog.Error("idle_detector: list running agents failed", "error", err)
		return
	}

	for _, a := range runningAgents {
		// 1. Liveness — does .heartbeat exist?
		heartbeatPath := filepath.Join(a.HermesHomePath, ".heartbeat")
		if _, err := os.Stat(heartbeatPath); err != nil {
			misses := d.incrMisses(a.ID)
			if misses >= d.maxMisses {
				slog.Info("idle_detector: no heartbeat for N checks, sleeping",
					"agent", a.ID, "misses", misses)
				d.resetMisses(a.ID)
				d.invokeSleep(a)
			}
			continue
		}
		d.resetMisses(a.ID)

		// 2. Read gateway_state.json — is the gateway processing?
		statePath := filepath.Join(a.HermesHomePath, "gateway_state.json")
		gs, err := readGatewayState(statePath)
		if err != nil {
			// File missing or unparseable — gateway may still be
			// starting. Skip this tick.
			continue
		}
		if gs.ActiveAgents > 0 {
			continue // busy — definitely not idle
		}

		// 3. Has enough time passed since the last turn boundary?
		info, err := os.Stat(statePath)
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) <= d.idleTimeout {
			continue
		}

		// 4. Final check: are there pending iLink messages?
		hasPending, err := d.hasPendingMessages(ctx, a)
		if err != nil {
			slog.Warn("idle_detector: ilink check failed",
				"agent", a.ID, "error", err)
			continue
		}
		if hasPending {
			continue // user sent a message, gateway hasn't pulled it yet
		}

		slog.Info("idle_detector: agent idle, sleeping",
			"agent", a.ID,
			"last_activity", info.ModTime(),
		)
		d.invokeSleep(a)
	}
}

// invokeSleep calls the sleep function in a goroutine so one slow Sleep
// doesn't block the detection loop.
func (d *IdleDetector) invokeSleep(a agents.Agent) {
	go func(agentID string) {
		if err := d.sleepFunc(context.Background(), agentID); err != nil {
			slog.Error("idle_detector: sleep failed",
				"agent", agentID, "error", err)
		}
	}(a.ID)
}

// hasPendingMessages checks iLink for un-consumed messages. It reads the
// weixin credentials from the agent's .env on NAS.
func (d *IdleDetector) hasPendingMessages(ctx context.Context, a agents.Agent) (bool, error) {
	ch, err := d.channelRepo.GetByAgentID(ctx, a.ID)
	if err != nil || ch.Status != channels.StatusConnected {
		return false, err
	}
	creds, err := readWeixinCreds(a.HermesHomePath)
	if err != nil {
		return false, err
	}
	return d.weixinClient.HasPendingMessages(ctx, weixin.MsgCheckRequest{
		BaseURL: creds.BaseURL,
		Token:   creds.Token,
	})
}

// gatewayState is a minimal view of Hermes' gateway_state.json — we only
// care about active_agents.
type gatewayState struct {
	ActiveAgents int `json:"active_agents"`
}

// readGatewayState parses gateway_state.json and returns the fields we
// need for idle detection. Returns a zero-value struct if the file is
// missing or unparseable.
func readGatewayState(path string) (gatewayState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return gatewayState{}, err
	}
	var gs gatewayState
	if err := json.Unmarshal(data, &gs); err != nil {
		return gatewayState{}, err
	}
	return gs, nil
}

// --- heartbeat miss tracking (concurrency-safe) -----------------------

func (d *IdleDetector) incrMisses(agentID string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.heartbeatMisses[agentID]++
	return d.heartbeatMisses[agentID]
}

func (d *IdleDetector) resetMisses(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.heartbeatMisses[agentID] = 0
}
