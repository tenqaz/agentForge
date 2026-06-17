package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultSupervisorPollInterval = 200 * time.Millisecond
	defaultSupervisorLeaseTTL     = 30 * time.Second
	defaultSupervisorWorkerID     = "agentforge-supervisor"
	errCodeJobCancelled           = "job_cancelled"
	errCodeUnsupportedRuntimeJob  = "unsupported_runtime_job"
	errCodeUnsupportedChannelJob  = "unsupported_channel_job"
)

type RuntimeJobProcessor interface {
	ProcessJob(ctx context.Context, jobID string) error
}

type ChannelJobProcessor interface {
	ProcessJob(ctx context.Context, jobID string) error
}

type SupervisorDependencies struct {
	RuntimeJobs   *RuntimeRepository
	ChannelJobs   *ChannelRepository
	RuntimeWorker RuntimeJobProcessor
	ChannelWorker ChannelJobProcessor
	WorkerID      string
	PollInterval  time.Duration
	LeaseTTL      time.Duration
}

type Supervisor struct {
	runtimeJobs   *RuntimeRepository
	channelJobs   *ChannelRepository
	runtimeWorker RuntimeJobProcessor
	channelWorker ChannelJobProcessor
	workerID      string
	pollInterval  time.Duration
	leaseTTL      time.Duration
}

func NewSupervisor(deps SupervisorDependencies) *Supervisor {
	workerID := strings.TrimSpace(deps.WorkerID)
	if workerID == "" {
		workerID = defaultSupervisorWorkerID
	}
	pollInterval := deps.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultSupervisorPollInterval
	}
	leaseTTL := deps.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = defaultSupervisorLeaseTTL
	}
	return &Supervisor{
		runtimeJobs:   deps.RuntimeJobs,
		channelJobs:   deps.ChannelJobs,
		runtimeWorker: deps.RuntimeWorker,
		channelWorker: deps.ChannelWorker,
		workerID:      workerID,
		pollInterval:  pollInterval,
		leaseTTL:      leaseTTL,
	}
}

func (s *Supervisor) Run(ctx context.Context) error {
	if s.runtimeJobs == nil || s.channelJobs == nil || s.runtimeWorker == nil || s.channelWorker == nil {
		return errors.New("supervisor dependencies are incomplete")
	}
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		worked, err := s.runOnce(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("run supervisor loop: %w", err)
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) runOnce(ctx context.Context) (bool, error) {
	lockedUntil := time.Now().Add(s.leaseTTL)

	runtimeJob, err := s.runtimeJobs.ClaimNextQueued(ctx, s.workerID, lockedUntil)
	switch {
	case err == nil:
		return true, s.processRuntimeJob(ctx, runtimeJob)
	case !errors.Is(err, ErrNotFound):
		return false, fmt.Errorf("claim next runtime job: %w", err)
	}

	channelJob, err := s.channelJobs.ClaimNextQueued(ctx, s.workerID, lockedUntil)
	switch {
	case err == nil:
		return true, s.processChannelJob(ctx, channelJob)
	case errors.Is(err, ErrNotFound):
		return false, nil
	default:
		return false, fmt.Errorf("claim next channel job: %w", err)
	}
}

func (s *Supervisor) processRuntimeJob(ctx context.Context, job RuntimeJob) error {
	if job.Type != TypeProvisionAgent && job.Type != TypeRestartRuntime {
		return s.runtimeJobs.MarkFailed(ctx, job.AgentID, job.ID, errCodeUnsupportedRuntimeJob, fmt.Sprintf("unsupported runtime job type %s", job.Type))
	}
	runCtx, stopLease := s.withLeaseExtender(ctx, func(leaseCtx context.Context) error {
		return s.runtimeJobs.ExtendLease(leaseCtx, job.AgentID, job.ID, s.workerID, time.Now().Add(s.leaseTTL))
	})
	defer stopLease()

	err := s.runtimeWorker.ProcessJob(runCtx, job.ID)
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if markErr := s.runtimeJobs.MarkFailed(ctx, job.AgentID, job.ID, stableErrorCode(err), err.Error()); markErr != nil && !errors.Is(markErr, ErrNotFound) {
		return fmt.Errorf("mark runtime job failed: %w", markErr)
	}
	return nil
}

func (s *Supervisor) processChannelJob(ctx context.Context, job ChannelJob) error {
	if job.Type != TypeConnectWeixin {
		return s.channelJobs.MarkFailed(ctx, job.ID, errCodeUnsupportedChannelJob, fmt.Sprintf("unsupported channel job type %s", job.Type))
	}
	runCtx, stopLease := s.withLeaseExtender(ctx, func(leaseCtx context.Context) error {
		return s.channelJobs.ExtendLease(leaseCtx, job.AgentChannelID, job.ID, s.workerID, time.Now().Add(s.leaseTTL))
	})
	defer stopLease()

	err := s.channelWorker.ProcessJob(runCtx, job.ID)
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if markErr := s.channelJobs.MarkFailed(ctx, job.ID, stableErrorCode(err), err.Error()); markErr != nil && !errors.Is(markErr, ErrNotFound) {
		return fmt.Errorf("mark channel job failed: %w", markErr)
	}
	return nil
}

func (s *Supervisor) withLeaseExtender(ctx context.Context, extend func(context.Context) error) (context.Context, func()) {
	leaseCtx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(s.leaseTTL / 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				_ = extend(context.Background())
			}
		}
	}()
	return leaseCtx, func() {
		cancel()
		<-done
	}
}

func stableErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return errCodeJobCancelled
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "job_failed"
	}
	return message
}
