package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/eci"
)

type eciRunner struct {
	client          *eci.Client
	region          string
	accessKeyID     string
	accessKeySecret string
	securityGroupID string
	vSwitchID       string
	imageCacheID    string // 镜像缓存 ID，用于加速冷启动
	eipInstanceID   string // 弹性公网 IP ID
	nasHost         string // NAS 挂载点地址
	nasPath         string // NAS 上根路径
	nasFileSystemID string // NAS 文件系统 ID
}

// ECIConfig holds Alibaba Cloud ECI configuration.
type ECIConfig struct {
	Region          string
	AccessKeyID     string
	AccessKeySecret string
	SecurityGroupID string
	VSwitchID       string
	ImageCacheID    string // 可选，镜像缓存 ID
	EIPInstanceID   string // 可选，弹性公网 IP，绑定到容器组

	// NAS mount configuration for persistent storage.
	NASHost         string // NAS 挂载点地址
	NASPath         string // NAS 上根路径，默认 "/"
	NASFileSystemID string // NAS 文件系统 ID，用于 CreateDir API
}

// NewECIRunner creates a Runner backed by Alibaba Cloud ECI.
func NewECIRunner(cfg ECIConfig) (Runner, error) {
	client, err := eci.NewClientWithAccessKey(cfg.Region, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("eci: create client: %w", err)
	}
	nasPath := cfg.NASPath
	if nasPath == "" {
		nasPath = "/"
	}
	return &eciRunner{
		client:          client,
		region:          cfg.Region,
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
		securityGroupID: cfg.SecurityGroupID,
		vSwitchID:       cfg.VSwitchID,
		imageCacheID:    cfg.ImageCacheID,
		eipInstanceID:   cfg.EIPInstanceID,
		nasHost:         cfg.NASHost,
		nasPath:         nasPath,
		nasFileSystemID: cfg.NASFileSystemID,
	}, nil
}

func (r *eciRunner) EnsureRunning(ctx context.Context, spec ContainerSpec) error {
	if strings.TrimSpace(spec.AgentID) == "" {
		return errors.New("eci: agent id is required")
	}

	containerName := DefaultContainerName(spec.AgentID)

	// Check if already running.
	status, err := r.Inspect(ctx, containerName)
	if err == nil {
		if status.Running {
			return nil
		}
		// Exists but not running — delete and recreate.
		// ECI containers that have Succeeded/Failed cannot be restarted.
		if err := r.Remove(ctx, containerName); err != nil && !errors.Is(err, ErrContainerNotFound) {
			return err
		}
	} else if !errors.Is(err, ErrContainerNotFound) {
		return err
	}

	req := eci.CreateCreateContainerGroupRequest()
	req.ContainerGroupName = containerName
	req.SecurityGroupId = r.securityGroupID
	req.VSwitchId = r.vSwitchID
	req.ZoneId = r.region + "-j"
	if r.eipInstanceID != "" {
		req.EipInstanceId = r.eipInstanceID
	}
	req.RestartPolicy = "Always"
	req.Memory = requests.NewFloat(toECIMemoryFloat(spec.Memory))
	req.Cpu = requests.NewFloat(toECICPUFloat(spec.CPUs))

	// Image cache for cold-start acceleration.
	if r.imageCacheID != "" {
		req.ImageSnapshotId = r.imageCacheID
	}

	envVars := []eci.CreateContainerGroupEnvironmentVar{
		{Key: "HERMES_HOME", Value: "/opt/data"},
		{Key: "HERMES_GATEWAY_NO_SUPERVISE", Value: "1"},
	}
	// Inject credentials and model config from local hermes-home files
	// (also on NAS, but env vars guarantee availability immediately).
	envFile := spec.HermesHome + "/.env"
	if extra, err := readEnvFile(envFile); err == nil {
		for k, v := range extra {
			if strings.HasPrefix(k, "WEIXIN_") || strings.HasPrefix(k, "GATEWAY_") {
				envVars = append(envVars, eci.CreateContainerGroupEnvironmentVar{Key: k, Value: v})
			}
		}
	}
	// Also inject the API server's own model config so Hermes can call the LLM.
	for _, kv := range []struct{ k, v string }{
		{"HERMES_MODEL_API_KEY", os.Getenv("AGENTFORGE_MODEL_API_KEY")},
		{"HERMES_MODEL_BASE_URL", os.Getenv("AGENTFORGE_MODEL_BASE_URL")},
		{"HERMES_MODEL_DEFAULT", os.Getenv("AGENTFORGE_MODEL_DEFAULT")},
		{"HERMES_MODEL_PROVIDER", os.Getenv("AGENTFORGE_MODEL_PROVIDER")},
		{"HERMES_MODEL_API_MODE", os.Getenv("AGENTFORGE_MODEL_API_MODE")},
	} {
		if kv.v != "" {
			envVars = append(envVars, eci.CreateContainerGroupEnvironmentVar{Key: kv.k, Value: kv.v})
		}
	}

	req.Container = &[]eci.CreateContainerGroupContainer{
		{
			Image:           spec.Image,
			Name:            "hermes",
			Memory:          requests.NewFloat(toECIMemoryFloat(spec.Memory)),
			Cpu:             requests.NewFloat(toECICPUFloat(spec.CPUs)),
			WorkingDir:      "/opt/data",
			Command: []string{
					"/init",
					"sh", "-c",
					// Wait for the NFS mount to be ready before starting Hermes.
					// On first boot the .env is written before the ECI is
					// created, but NFS can lag. On restart (after pairing)
					// the new .env is already on NAS.
					"while [ ! -f /opt/data/.env ]; do sleep 0.5; done; chmod -R 777 /opt/data/weixin 2>/dev/null; exec /opt/hermes/docker/main-wrapper.sh gateway run",
				},
			EnvironmentVar: &envVars,
			VolumeMount: &[]eci.CreateContainerGroupVolumeMount{
				{
					Name:      "hermes-data",
					MountPath: "/opt/data",
				},
			},
		},
	}

	// NAS: create per-agent subdirectory, then mount via NFS.
	nasDir := r.nasPath
	if !strings.HasSuffix(nasDir, "/") {
		nasDir += "/"
	}
	nasDir += spec.AgentID

	slog.Info("ECI: ensuring NAS dir", "path", nasDir)
	if err := r.createNASDir(nasDir); err != nil {
		slog.Error("ECI: CreateDir failed", "path", nasDir, "error", err)
	}

	// Remove stale gateway state so Hermes does a fresh platform scan on
	// every boot instead of inheriting a cached platforms:{} from before.
	for _, f := range []string{"gateway_state.json", "gateway.lock", "gateway.pid"} {
		_ = os.Remove(filepath.Join(spec.HermesHome, f))
	}

	req.Volume = &[]eci.CreateContainerGroupVolume{
		{
			Name: "hermes-data",
			Type: "NFSVolume",
			NFSVolume: eci.CreateContainerGroupNFSVolume{
				Server: r.nasHost,
				Path:   nasDir,
			},
		},
	}

	slog.Info("ECI CreateContainerGroup", "name", containerName, "image", spec.Image,
		"memory", spec.Memory, "cpus", spec.CPUs, "nasDir", nasDir)
	resp, err := r.client.CreateContainerGroup(req)
	if err != nil {
		return fmt.Errorf("eci: create container group: %w", err)
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("eci: create container group failed: %s", resp.GetHttpStatus())
	}

	return nil
}

func (r *eciRunner) Stop(ctx context.Context, containerName string) error {
	return r.Remove(ctx, containerName)
}

func (r *eciRunner) Remove(ctx context.Context, containerName string) error {
	// Resolve name → ContainerGroupId first.
	groupID, err := r.containerGroupIDByName(containerName)
	if err != nil {
		if errors.Is(err, ErrContainerNotFound) {
			return ErrContainerNotFound
		}
		return err
	}

	req := eci.CreateDeleteContainerGroupRequest()
	req.ContainerGroupId = groupID

	resp, err := r.client.DeleteContainerGroup(req)
	if err != nil {
		trimmed := strings.TrimSpace(err.Error())
		if isECINotFound(trimmed) {
			return ErrContainerNotFound
		}
		return fmt.Errorf("eci: delete container group: %w", err)
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("eci: delete container group failed: %s", resp.GetHttpStatus())
	}
	return nil
}

func (r *eciRunner) Inspect(ctx context.Context, containerName string) (ContainerStatus, error) {
	req := eci.CreateDescribeContainerGroupsRequest()
	req.ContainerGroupName = containerName
	req.Limit = requests.NewInteger(1)

	resp, err := r.client.DescribeContainerGroups(req)
	if err != nil {
		return ContainerStatus{}, fmt.Errorf("eci: describe container groups: %w", err)
	}

	groups := resp.ContainerGroups
	if len(groups) == 0 {
		return ContainerStatus{}, ErrContainerNotFound
	}

	g := groups[0]
	status := ContainerStatus{Exists: true}

	groupState := strings.ToLower(g.Status)
	// Check the actual container state, not just the group status.
	// A group can be "Running" while a container inside is crashing.
	containerRunning := false
	if len(g.Containers) > 0 {
		c := g.Containers[0]
		containerState := strings.ToLower(c.CurrentState.State)
		if containerState == "running" {
			containerRunning = true
		}
		// Use container state for more accurate status.
		if containerState != "" {
			groupState = containerState
		}
	}

	switch groupState {
	case "running":
		status.Running = true
		status.Status = "running"
	case "pending", "waiting":
		status.Running = false
		status.Status = "pending"
	case "terminated", "succeeded", "failed":
		status.Running = false
		status.Status = "stopped"
	default:
		status.Running = containerRunning
		status.Status = groupState
	}

	return status, nil
}

// containerGroupIDByName resolves the ContainerGroupId from a name.
func (r *eciRunner) containerGroupIDByName(containerName string) (string, error) {
	req := eci.CreateDescribeContainerGroupsRequest()
	req.ContainerGroupName = containerName
	req.Limit = requests.NewInteger(1)

	resp, err := r.client.DescribeContainerGroups(req)
	if err != nil {
		return "", fmt.Errorf("eci: describe container groups: %w", err)
	}
	if len(resp.ContainerGroups) == 0 {
		return "", ErrContainerNotFound
	}
	return resp.ContainerGroups[0].ContainerGroupId, nil
}

// toECIMemoryFloat converts memory strings to GiB for ECI.
// Supports: "500m" (MB), "2Gi" (GiB), "1G" (GiB), "1024Mi" (MiB), plain float (GiB).
func toECIMemoryFloat(mem string) float64 {
	mem = strings.TrimSpace(mem)
	lower := strings.ToLower(mem)

	switch {
	case strings.HasSuffix(lower, "mi"):
		// MiB → GiB
		return parseFloat(strings.TrimSuffix(lower, "mi")) / 1024
	case strings.HasSuffix(lower, "m"):
		// MB → GiB
		return parseFloat(strings.TrimSuffix(lower, "m")) / 1000
	case strings.HasSuffix(lower, "gi"), strings.HasSuffix(lower, "g"):
		// Already GiB
		val := strings.TrimSuffix(lower, "gi")
		val = strings.TrimSuffix(val, "g")
		return parseFloat(val)
	default:
		// Assume raw GiB value
		return parseFloat(lower)
	}
}

// toECICPUFloat converts to float for the Container spec.
func toECICPUFloat(cpu string) float64 {
	return parseFloat(strings.TrimSpace(cpu))
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0.5 // safe default
	}
	return f
}

func isECINotFound(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "not found") ||
		strings.Contains(lower, "invalidcontainergroupid") ||
		strings.Contains(lower, "does not exist")
}

// createNASDir ensures the given path exists on the NAS file system
// by calling the Alibaba Cloud NAS CreateDir API.
func (r *eciRunner) createNASDir(path string) error {
	if r.nasFileSystemID == "" {
		return errors.New("nas file system id not configured")
	}

	cli, err := sdk.NewClientWithAccessKey(r.region, r.accessKeyID, r.accessKeySecret)
	if err != nil {
		return fmt.Errorf("nas: create client: %w", err)
	}

	req := requests.NewCommonRequest()
	req.Method = "POST"
	req.Domain = "nas." + r.region + ".aliyuncs.com"
	req.Version = "2017-06-26"
	req.ApiName = "CreateDir"
	req.QueryParams["FileSystemId"] = r.nasFileSystemID
	req.QueryParams["RootDirectory"] = path
	req.QueryParams["Recursion"] = "true"
	req.QueryParams["OwnerUserId"] = "0"
	req.QueryParams["OwnerGroupId"] = "0"
	req.QueryParams["Permission"] = "0755"

	resp, err := cli.ProcessCommonRequest(req)
	if err != nil {
		return fmt.Errorf("nas: create dir: %w", err)
	}
	if !resp.IsSuccess() {
		// "AlreadyExists" error is OK — the dir already exists.
		if strings.Contains(resp.String(), "AlreadyExists") ||
			strings.Contains(resp.String(), "PathAlreadyExists") {
			return nil
		}
		return fmt.Errorf("nas: create dir failed: %s", resp.GetHttpStatus())
	}
	return nil
}

// readEnvFile reads a .env file and returns its key-value pairs.
func readEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if k != "" {
			result[k] = v
		}
	}
	return result, nil
}
