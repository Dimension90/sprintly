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

type memoryStore struct {
	issues  []Issue
	sprints []Sprint
}

func (s *memoryStore) List(context.Context) ([]Issue, error) {
	return append([]Issue(nil), s.issues...), nil
}
func (s *memoryStore) ListSprints(context.Context) ([]Sprint, error) {
	if len(s.sprints) == 0 {
		s.sprints = []Sprint{{ID: "SPR-24", Name: "Спринт 24", State: "active"}}
	}
	return append([]Sprint(nil), s.sprints...), nil
}
func (s *memoryStore) CreateSprint(_ context.Context, sprint Sprint) (Sprint, error) {
	if strings.TrimSpace(sprint.Name) == "" {
		return Sprint{}, errors.New("name is required")
	}
	if sprint.StartDate.IsZero() || !sprint.EndDate.After(sprint.StartDate) {
		return Sprint{}, errors.New("invalid sprint dates")
	}
	sprint.ID, sprint.State = "SPR-25", "planned"
	s.sprints = append(s.sprints, sprint)
	return sprint, nil
}
func (s *memoryStore) StartSprint(_ context.Context, id string) (Sprint, error) {
	for _, sprint := range s.sprints {
		if sprint.State == "active" && sprint.ID != id {
			return Sprint{}, errActiveSprint
		}
	}
	for i := range s.sprints {
		if s.sprints[i].ID == id && s.sprints[i].State == "planned" {
			s.sprints[i].State = "active"
			return s.sprints[i], nil
		}
	}
	return Sprint{}, errInvalidState
}
func (s *memoryStore) CompleteSprint(_ context.Context, id string) (Sprint, error) {
	for i := range s.sprints {
		if s.sprints[i].ID == id && s.sprints[i].State == "active" {
			s.sprints[i].State = "completed"
			for j := range s.issues {
				if s.issues[j].SprintID == id && s.issues[j].Status != "done" {
					s.issues[j].SprintID = ""
				}
			}
			return s.sprints[i], nil
		}
	}
	return Sprint{}, errInvalidState
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

func testAPI() http.Handler {
	return API{store: &memoryStore{issues: seedIssues(), sprints: []Sprint{{ID: "SPR-24", Name: "Спринт 24", State: "active"}}}}.routes()
}

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

func TestListSprints(t *testing.T) {
	res := httptest.NewRecorder()
	testAPI().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/sprints", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestSprintLifecycle(t *testing.T) {
	store := &memoryStore{issues: seedIssues(), sprints: []Sprint{{ID: "SPR-24", Name: "Спринт 24", State: "active"}}}
	handler := API{store: store}.routes()
	createBody := `{"name":"Спринт 25","goal":"Релиз","startDate":"2026-09-16T00:00:00Z","endDate":"2026-09-29T00:00:00Z"}`
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/api/sprints", bytes.NewBufferString(createBody)))
	if created.Code != http.StatusCreated {
		t.Fatalf("create sprint status = %d, body = %s", created.Code, created.Body.String())
	}
	blocked := httptest.NewRecorder()
	handler.ServeHTTP(blocked, httptest.NewRequest(http.MethodPost, "/api/sprints/SPR-25/start", nil))
	if blocked.Code != http.StatusConflict {
		t.Fatalf("start with active sprint status = %d", blocked.Code)
	}
	completed := httptest.NewRecorder()
	handler.ServeHTTP(completed, httptest.NewRequest(http.MethodPost, "/api/sprints/SPR-24/complete", nil))
	if completed.Code != http.StatusOK {
		t.Fatalf("complete status = %d", completed.Code)
	}
	started := httptest.NewRecorder()
	handler.ServeHTTP(started, httptest.NewRequest(http.MethodPost, "/api/sprints/SPR-25/start", nil))
	if started.Code != http.StatusOK {
		t.Fatalf("start status = %d, body = %s", started.Code, started.Body.String())
	}
}
