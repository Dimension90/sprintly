package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type memoryStore struct{ issues []Issue }

func (s *memoryStore) List(context.Context) ([]Issue, error) {
	return append([]Issue(nil), s.issues...), nil
}
func (s *memoryStore) Create(_ context.Context, issue Issue) (Issue, error) {
	if strings.TrimSpace(issue.Title) == "" {
		return Issue{}, errors.New("title is required")
	}
	issue.ID, issue.Status, issue.CreatedAt = "ORB-171", "backlog", time.Now()
	s.issues = append([]Issue{issue}, s.issues...)
	return issue, nil
}
func (s *memoryStore) Update(_ context.Context, id string, input Issue) (Issue, error) {
	if !validStatus(input.Status) {
		return Issue{}, errors.New("invalid status")
	}
	for i := range s.issues {
		if s.issues[i].ID == id {
			input.ID, input.CreatedAt = id, s.issues[i].CreatedAt
			s.issues[i] = input
			return s.issues[i], nil
		}
	}
	return Issue{}, errors.New("issue not found")
}
func (s *memoryStore) Delete(_ context.Context, id string) error {
	for i := range s.issues {
		if s.issues[i].ID == id {
			s.issues = append(s.issues[:i], s.issues[i+1:]...)
			return nil
		}
	}
	return errors.New("issue not found")
}

func testAPI() http.Handler { return API{store: &memoryStore{issues: seedIssues()}}.routes() }

func TestCreateAndMoveIssue(t *testing.T) {
	handler := testAPI()
	create := httptest.NewRequest(http.MethodPost, "/api/issues", bytes.NewBufferString(`{"title":"Проверить API","points":2}`))
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var issue Issue
	if err := json.NewDecoder(created.Body).Decode(&issue); err != nil {
		t.Fatal(err)
	}
	issue.Title, issue.Status, issue.Priority, issue.Assignee = "Проверить API", "done", "Средний", "АК"
	payload, _ := json.Marshal(issue)
	move := httptest.NewRequest(http.MethodPut, "/api/issues/"+issue.ID, bytes.NewReader(payload))
	moved := httptest.NewRecorder()
	handler.ServeHTTP(moved, move)
	if moved.Code != http.StatusOK {
		t.Fatalf("move status = %d, body = %s", moved.Code, moved.Body.String())
	}
}

func TestRejectsEmptyTitle(t *testing.T) {
	res := httptest.NewRecorder()
	testAPI().ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/issues", bytes.NewBufferString(`{"title":"   "}`)))
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestRejectsInvalidStatus(t *testing.T) {
	res := httptest.NewRecorder()
	testAPI().ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/api/issues/ORB-142", bytes.NewBufferString(`{"title":"Test","status":"unknown"}`)))
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestDeleteIssue(t *testing.T) {
	res := httptest.NewRecorder()
	testAPI().ServeHTTP(res, httptest.NewRequest(http.MethodDelete, "/api/issues/ORB-142", nil))
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d", res.Code)
	}
}
