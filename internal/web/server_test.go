package web_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/store"
	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/web"
)

func TestWorkspaceAndAPISecurityBoundary(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	t.Cleanup(func() { service.Close() })
	handler := web.New(service).Handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/workspace", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("workspace status %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), "<body>") || !strings.Contains(page.Body.String(), "壁画微生物处置放行台") {
		t.Fatal("workspace HTML incomplete")
	}
	if page.Header().Get("Content-Security-Policy") == "" || page.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("security headers missing")
	}
	badType := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cases", bytes.NewBufferString("{}"))
	handler.ServeHTTP(badType, request)
	if badType.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad type status %d", badType.Code)
	}
	if !strings.Contains(badType.Body.String(), "content_type_required") {
		t.Fatal("stable error code missing")
	}
}
