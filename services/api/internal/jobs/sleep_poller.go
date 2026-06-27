package jobs

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/weixin"
)

// SleepPoller periodically checks sleeping agents for new iLink messages
// and wakes them when a user sends a message.
type SleepPoller struct {
	agentRepo    *agents.Repository
	channelRepo  *channels.Repository
	weixinClient weixin.Client
	wakeFunc     func(ctx context.Context, agentID string) error
	pollInterval time.Duration
}

// SleepPollerDeps is the dependency injection container for SleepPoller.
type SleepPollerDeps struct {
	AgentRepo    *agents.Repository
	ChannelRepo  *channels.Repository
	WeixinClient weixin.Client
	WakeFunc     func(ctx context.Context, agentID string) error
	PollInterval time.Duration
}

// NewSleepPoller creates a SleepPoller. If pollInterval is <= 0 it
// defaults to 5 seconds.
func NewSleepPoller(deps SleepPollerDeps) *SleepPoller {
	interval := deps.PollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &SleepPoller{
		agentRepo:    deps.AgentRepo,
		channelRepo:  deps.ChannelRepo,
		weixinClient: deps.WeixinClient,
		wakeFunc:     deps.WakeFunc,
		pollInterval: interval,
	}
}

// Run starts the poll loop. It blocks until ctx is cancelled.
func (p *SleepPoller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	slog.Info("sleep_poller: started", "interval", p.pollInterval)
	for {
		select {
		case <-ctx.Done():
			slog.Info("sleep_poller: stopped")
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

func (p *SleepPoller) pollOnce(ctx context.Context) {
	sleepingAgents, err := p.agentRepo.ListByStatus(ctx, agents.StatusSleeping)
	if err != nil {
		slog.Error("sleep_poller: list sleeping agents failed", "error", err)
		return
	}
	if len(sleepingAgents) == 0 {
		return
	}

	for _, a := range sleepingAgents {
		ch, err := p.channelRepo.GetByAgentID(ctx, a.ID)
		if err != nil || ch.Status != channels.StatusConnected {
			continue
		}

		creds, err := readWeixinCreds(a.HermesHomePath)
		if err != nil {
			slog.Warn("sleep_poller: read weixin creds failed",
				"agent", a.ID, "error", err)
			continue
		}

		hasNew, err := p.weixinClient.HasPendingMessages(ctx, weixin.MsgCheckRequest{
			BaseURL: creds.BaseURL,
			Token:   creds.Token,
		})
		if err != nil {
			slog.Warn("sleep_poller: ilink check failed",
				"agent", a.ID, "error", err)
			continue
		}
		if !hasNew {
			continue
		}

		slog.Info("sleep_poller: waking agent", "agent", a.ID)
		// Wake asynchronously so one slow wake doesn't block the poll loop.
		go func(agentID string) {
			if err := p.wakeFunc(context.Background(), agentID); err != nil {
				slog.Error("sleep_poller: wake failed",
					"agent", agentID, "error", err)
			}
		}(a.ID)
	}
}

// weixinCreds holds the minimal credentials needed for an iLink
// getupdates call.
type weixinCreds struct {
	BaseURL string
	Token   string
}

// readWeixinCreds reads WEIXIN_BASE_URL and WEIXIN_TOKEN from the
// agent's .env file on NAS.
func readWeixinCreds(homePath string) (weixinCreds, error) {
	envMap, err := runtime.ReadEnvFile(filepath.Join(homePath, ".env"))
	if err != nil {
		return weixinCreds{}, err
	}
	return weixinCreds{
		BaseURL: envMap["WEIXIN_BASE_URL"],
		Token:   envMap["WEIXIN_TOKEN"],
	}, nil
}
