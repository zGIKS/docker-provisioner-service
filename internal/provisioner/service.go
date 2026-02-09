package provisioner

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go-service/internal/config"
)

var (
	invalidTenantChars = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	ErrInvalidTenant   = errors.New("invalid tenant_name")
	ErrInvalidResource = errors.New("resource_id is required")
)

type ErrAlreadyProvisioned struct {
	TenantName string
	ResourceID string
}

func (e *ErrAlreadyProvisioned) Error() string {
	return fmt.Sprintf("tenant %q is already provisioned", e.TenantName)
}

type DockerRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

type Service struct {
	runner DockerRunner
	cfg    config.Config
}

type Limits struct {
	MemoryMB *int64   `json:"memory_mb,omitempty"`
	CPUCores *float64 `json:"cpu_cores,omitempty"`
}

type ProvisionRequest struct {
	TenantName string  `json:"tenant_name"`
	TenantID   string  `json:"tenant_id,omitempty"`
	Limits     *Limits `json:"limits,omitempty"`
}

type ProvisionResult struct {
	Status           string `json:"status"`
	ResourceID       string `json:"resource_id"`
	ConnectionString string `json:"connection_string"`
	DBSecretPath     string `json:"db_secret_path"`
}

func NewService(runner DockerRunner, cfg config.Config) *Service {
	return &Service{runner: runner, cfg: cfg}
}

func (s *Service) ProvisionTenant(ctx context.Context, req ProvisionRequest) (ProvisionResult, error) {
	tenantName := strings.TrimSpace(req.TenantName)
	tenantID := strings.TrimSpace(req.TenantID)
	safeTenantName := normalizeTenantName(tenantName)
	if safeTenantName == "" {
		return ProvisionResult{}, ErrInvalidTenant
	}

	containerName := "tenant-db-" + safeTenantName
	existingResourceID, err := s.lookupContainerID(ctx, containerName)
	if err != nil {
		return ProvisionResult{}, err
	}
	if existingResourceID != "" {
		return ProvisionResult{}, &ErrAlreadyProvisioned{
			TenantName: safeTenantName,
			ResourceID: existingResourceID,
		}
	}

	if err := s.ensureNetwork(ctx); err != nil {
		return ProvisionResult{}, err
	}

	if err := s.pullImage(ctx); err != nil {
		return ProvisionResult{}, err
	}

	dbName := s.cfg.TenantDBNamePrefix + safeTenantName

	password, err := generatePassword(32)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("generate password: %w", err)
	}

	memoryMB := s.cfg.DefaultMemoryMB
	cpuCores := s.cfg.DefaultCPUCores
	if req.Limits != nil {
		if req.Limits.MemoryMB != nil {
			memoryMB = req.Limits.MemoryMB
		}
		if req.Limits.CPUCores != nil {
			cpuCores = req.Limits.CPUCores
		}
	}

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", s.cfg.TenantDBNetwork,
		"--label", "managed_by=iam-provisioner",
		"--label", "tenant_name=" + safeTenantName,
		"-e", "POSTGRES_USER=" + s.cfg.TenantDBUser,
		"-e", "POSTGRES_PASSWORD=" + password,
		"-e", "POSTGRES_DB=" + dbName,
		"-p", "0:5432",
	}
	if tenantID != "" {
		args = append(args, "--label", "tenant_id="+tenantID)
	}

	if memoryMB != nil && *memoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", *memoryMB))
	}
	if cpuCores != nil && *cpuCores > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%g", *cpuCores))
	}

	args = append(args, s.cfg.TenantDBImage)

	containerIDRaw, err := s.runner.Run(ctx, args...)
	if err != nil {
		return ProvisionResult{}, err
	}
	containerID := strings.TrimSpace(containerIDRaw)

	portMapping, err := s.runner.Run(ctx, "port", containerID, "5432/tcp")
	if err != nil {
		_ = s.Deprovision(ctx, containerID)
		return ProvisionResult{}, err
	}

	port, err := parseDockerPort(portMapping)
	if err != nil {
		_ = s.Deprovision(ctx, containerID)
		return ProvisionResult{}, err
	}

	connectionString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		s.cfg.TenantDBUser,
		password,
		s.cfg.TenantDBHost,
		port,
		dbName,
	)

	return ProvisionResult{
		Status:           "provisioned",
		ResourceID:       containerID,
		ConnectionString: connectionString,
		DBSecretPath:     fmt.Sprintf("tenants/%s/db", safeTenantName),
	}, nil
}

func (s *Service) Deprovision(ctx context.Context, resourceID string) error {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return ErrInvalidResource
	}

	_, err := s.runner.Run(ctx, "rm", "-f", resourceID)
	if err != nil {
		if strings.Contains(err.Error(), "No such container") {
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) ensureNetwork(ctx context.Context) error {
	_, err := s.runner.Run(ctx, "network", "inspect", s.cfg.TenantDBNetwork)
	if err == nil {
		return nil
	}
	_, err = s.runner.Run(ctx, "network", "create", s.cfg.TenantDBNetwork)
	return err
}

func (s *Service) pullImage(ctx context.Context) error {
	_, err := s.runner.Run(ctx, "pull", s.cfg.TenantDBImage)
	return err
}

func (s *Service) lookupContainerID(ctx context.Context, containerName string) (string, error) {
	out, err := s.runner.Run(ctx, "inspect", "--type", "container", "--format", "{{.Id}}", containerName)
	if err != nil {
		if strings.Contains(err.Error(), "No such object") || strings.Contains(err.Error(), "No such container") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func normalizeTenantName(tenantName string) string {
	trimmed := strings.TrimSpace(tenantName)
	replaced := strings.ReplaceAll(trimmed, "-", "_")
	replaced = invalidTenantChars.ReplaceAllString(replaced, "_")
	replaced = strings.Trim(replaced, "_")
	return replaced
}

func generatePassword(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if length <= 0 {
		return "", errors.New("length must be positive")
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	var b strings.Builder
	b.Grow(length)
	for _, v := range bytes {
		b.WriteByte(alphabet[int(v)%len(alphabet)])
	}

	return b.String(), nil
}

func parseDockerPort(raw string) (string, error) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.LastIndex(line, ":")
		if idx == -1 || idx+1 >= len(line) {
			continue
		}
		port := strings.TrimSpace(line[idx+1:])
		if port != "" {
			return port, nil
		}
	}
	return "", fmt.Errorf("failed to parse docker port output: %q", raw)
}
