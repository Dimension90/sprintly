package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func testAPI(t *testing.T) http.Handler {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "issues.json"))
	if err != nil {
		t.Fatal(err)
	}
	return API{store: store}.routes()
}

func TestCreateAndMoveIssue(t *testing.T) {
	handler := testAPI(t)
	create := httptest.NewRequest(http.MethodPost, "/api/issues", bytes.NewBufferString(`{"title":"Проверить API","points":2}`))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var issue Issue
	if err := json.NewDecoder(created.Body).Decode(&issue); err != nil {
		t.Fatal(err)
	}

	move := httptest.NewRequest(http.MethodPatch, "/api/issues/"+issue.ID+"/status", bytes.NewBufferString(`{"status":"done"}`))
	moved := httptest.NewRecorder()
	handler.ServeHTTP(moved, move)
	if moved.Code != http.StatusOK {
		t.Fatalf("move status = %d, body = %s", moved.Code, moved.Body.String())
	}
	if err := json.NewDecoder(moved.Body).Decode(&issue); err != nil {
		t.Fatal(err)
	}
	if issue.Status != "done" {
		t.Fatalf("status = %q", issue.Status)
	}
}

func TestRejectsEmptyTitle(t *testing.T) {
	handler := testAPI(t)
	req := httptest.NewRequest(http.MethodPost, "/api/issues", bytes.NewBufferString(`{"title":"   "}`))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", res.Code)
	}
}
