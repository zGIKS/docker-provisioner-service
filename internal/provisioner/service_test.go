package provisioner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go-service/internal/config"
)

type fakeRunner struct {
	handler func(args ...string) (string, error)
	calls   [][]string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) (string, error) {
	call := append([]string(nil), args...)
	f.calls = append(f.calls, call)
	return f.handler(args...)
}

func TestNormalizeTenantName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{name: "simple", in: "acme", out: "acme"},
		{name: "spaces and dash", in: "  acme-prod  ", out: "acme_prod"},
		{name: "path chars", in: "../acme/root", out: "acme_root"},
		{name: "only invalid", in: "////", out: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTenantName(tc.in)
			if got != tc.out {
				t.Fatalf("normalizeTenantName(%q) = %q, want %q", tc.in, got, tc.out)
			}
		})
	}
}

func TestParseDockerPort(t *testing.T) {
	t.Run("single mapping", func(t *testing.T) {
		got, err := parseDockerPort("0.0.0.0:32768")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "32768" {
			t.Fatalf("got %q, want 32768", got)
		}
	})

	t.Run("empty mapping", func(t *testing.T) {
		_, err := parseDockerPort("\n")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestProvisionTenantUsesCanonicalSecretPathAndLabels(t *testing.T) {
	runner := &fakeRunner{}
	runner.handler = func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "inspect" {
			return "", errors.New("No such object")
		}
		if len(args) >= 1 && args[0] == "run" {
			return "container-123\n", nil
		}
		if len(args) >= 1 && args[0] == "port" {
			return "0.0.0.0:54321", nil
		}
		return "", nil
	}

	svc := NewService(runner, config.Config{
		TenantDBImage:      "postgres:16-alpine",
		TenantDBNetwork:    "auth-tenants",
		TenantDBHost:       "127.0.0.1",
		TenantDBUser:       "tenant_user",
		TenantDBNamePrefix: "tenant_",
	})

	result, err := svc.ProvisionTenant(context.Background(), ProvisionRequest{
		TenantName: "../Acme-Prod",
		TenantID:   "tenant-42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DBSecretPath != "tenants/Acme_Prod/db" {
		t.Fatalf("db_secret_path = %q, want %q", result.DBSecretPath, "tenants/Acme_Prod/db")
	}
	if result.ResourceID != "container-123" {
		t.Fatalf("resource_id = %q, want container-123", result.ResourceID)
	}

	var runCall []string
	for _, call := range runner.calls {
		if len(call) > 0 && call[0] == "run" {
			runCall = call
			break
		}
	}
	if len(runCall) == 0 {
		t.Fatal("expected docker run call")
	}

	runJoined := strings.Join(runCall, " ")
	if !strings.Contains(runJoined, "--label tenant_name=Acme_Prod") {
		t.Fatalf("missing tenant_name label in run args: %v", runCall)
	}
	if !strings.Contains(runJoined, "--label tenant_id=tenant-42") {
		t.Fatalf("missing tenant_id label in run args: %v", runCall)
	}
}

func TestProvisionTenantReturnsAlreadyProvisionedConflict(t *testing.T) {
	runner := &fakeRunner{}
	runner.handler = func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "inspect" {
			return "existing-container-id\n", nil
		}
		return "", nil
	}

	svc := NewService(runner, config.Config{TenantDBNamePrefix: "tenant_"})

	_, err := svc.ProvisionTenant(context.Background(), ProvisionRequest{TenantName: "acme"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var conflictErr *ErrAlreadyProvisioned
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ErrAlreadyProvisioned, got %T (%v)", err, err)
	}
	if conflictErr.ResourceID != "existing-container-id" {
		t.Fatalf("resource id = %q, want existing-container-id", conflictErr.ResourceID)
	}

	for _, call := range runner.calls {
		if len(call) > 0 && call[0] == "run" {
			t.Fatalf("did not expect docker run call when container already exists; calls=%v", runner.calls)
		}
	}
}
