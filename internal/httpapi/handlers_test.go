package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"go-service/internal/config"
	"go-service/internal/provisioner"
)

type testRunner struct {
	handler func(args ...string) (string, error)
}

func (r *testRunner) Run(_ context.Context, args ...string) (string, error) {
	return r.handler(args...)
}

func newTestApp(t *testing.T, runner provisioner.DockerRunner) *fiber.App {
	t.Helper()
	svc := provisioner.NewService(runner, config.Config{
		TenantDBImage:      "postgres:16-alpine",
		TenantDBNetwork:    "auth-tenants",
		TenantDBHost:       "127.0.0.1",
		TenantDBUser:       "tenant_user",
		TenantDBNamePrefix: "tenant_",
	})
	h := NewHandler(svc)
	app := fiber.New()
	h.Register(app)
	return app
}

func performRequest(t *testing.T, app *fiber.App, method, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	return string(b)
}

func TestProvisionTenantInvalidBody(t *testing.T) {
	app := newTestApp(t, &testRunner{handler: func(args ...string) (string, error) {
		return "", nil
	}})

	resp := performRequest(t, app, http.MethodPost, "/api/v1/provision/tenants", "{bad-json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestProvisionTenantAlreadyProvisioned(t *testing.T) {
	runner := &testRunner{handler: func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "inspect" {
			return "existing-id\n", nil
		}
		return "", nil
	}}
	app := newTestApp(t, runner)

	resp := performRequest(t, app, http.MethodPost, "/api/v1/provision/tenants", `{"tenant_name":"acme"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "already provisioned") || !strings.Contains(body, "existing-id") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestProvisionTenantCreated(t *testing.T) {
	runner := &testRunner{handler: func(args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == "inspect":
			return "", fmt.Errorf("No such object")
		case len(args) > 0 && args[0] == "run":
			return "container-abc\n", nil
		case len(args) > 0 && args[0] == "port":
			return "0.0.0.0:50001", nil
		default:
			return "", nil
		}
	}}
	app := newTestApp(t, runner)

	resp := performRequest(t, app, http.MethodPost, "/api/v1/provision/tenants", `{"tenant_name":"acme"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, `"status":"provisioned"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestDeprovisionInvalidBody(t *testing.T) {
	app := newTestApp(t, &testRunner{handler: func(args ...string) (string, error) {
		return "", nil
	}})

	resp := performRequest(t, app, http.MethodPost, "/api/v1/provision/deprovision", "{bad-json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestDeprovisionOK(t *testing.T) {
	runner := &testRunner{handler: func(args ...string) (string, error) {
		return "", nil
	}}
	app := newTestApp(t, runner)

	resp := performRequest(t, app, http.MethodDelete, "/api/v1/provision/resources/container-123", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
