package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Issue struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
	Points      int    `json:"points"`
	Comments    int    `json:"comments"`
	Attachments int    `json:"attachments"`
	Label       string `json:"label,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type Store struct {
	mu     sync.RWMutex
	path   string
	issues []Issue
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.issues = seedIssues()
		return s.saveLocked()
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.issues)
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.issues, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) List() []Issue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Issue(nil), s.issues...)
}

func (s *Store) Create(issue Issue) (Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	issue.Title = strings.TrimSpace(issue.Title)
	if issue.Title == "" {
		return Issue{}, errors.New("title is required")
	}
	issue.ID = fmt.Sprintf("ORB-%d", 171+len(s.issues))
	issue.Status = "backlog"
	if issue.Priority == "" {
		issue.Priority = "Средний"
	}
	if issue.Assignee == "" {
		issue.Assignee = "АК"
	}
	if issue.Points == 0 {
		issue.Points = 3
	}
	issue.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	s.issues = append([]Issue{issue}, s.issues...)
	return issue, s.saveLocked()
}

func (s *Store) UpdateStatus(id, status string) (Issue, error) {
	if !validStatus(status) {
		return Issue{}, errors.New("invalid status")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.issues {
		if s.issues[i].ID == id {
			s.issues[i].Status = status
			return s.issues[i], s.saveLocked()
		}
	}
	return Issue{}, errors.New("issue not found")
}

func validStatus(status string) bool {
	return status == "backlog" || status == "progress" || status == "review" || status == "done"
}

type API struct{ store *Store }

func (api API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/issues", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, api.store.List()) })
	mux.HandleFunc("POST /api/issues", api.createIssue)
	mux.HandleFunc("PATCH /api/issues/{id}/status", api.updateStatus)
	return withCORS(withLogging(mux))
}

func (api API) createIssue(w http.ResponseWriter, r *http.Request) {
	var input Issue
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	issue, err := api.store.Create(input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, issue)
}

func (api API) updateStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	issue, err := api.store.UpdateStatus(r.PathValue("id"), input.Status)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if err.Error() == "issue not found" {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:3000")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func seedIssues() []Issue {
	now := time.Now().UTC().Format(time.RFC3339)
	return []Issue{
		{ID: "ORB-142", Title: "Обновить онбординг для новых команд", Status: "backlog", Priority: "Высокий", Assignee: "АК", Points: 5, Comments: 8, Attachments: 2, Label: "Продукт", CreatedAt: now},
		{ID: "ORB-156", Title: "Добавить быстрые фильтры на доску", Status: "backlog", Priority: "Средний", Assignee: "МЛ", Points: 3, Comments: 3, Label: "UX", CreatedAt: now},
		{ID: "ORB-161", Title: "Тексты пустых состояний", Status: "backlog", Priority: "Низкий", Assignee: "ЕС", Points: 2, Comments: 1, Attachments: 1, Label: "Контент", CreatedAt: now},
		{ID: "ORB-133", Title: "Новый экран аналитики спринта", Status: "progress", Priority: "Высокий", Assignee: "ДР", Points: 8, Comments: 12, Attachments: 4, Label: "Дизайн", CreatedAt: now},
		{ID: "ORB-149", Title: "Оптимизировать загрузку карточек", Status: "progress", Priority: "Средний", Assignee: "АК", Points: 5, Comments: 5, Attachments: 1, Label: "Backend", CreatedAt: now},
		{ID: "ORB-152", Title: "Поддержка drag-and-drop на мобильных", Status: "progress", Priority: "Средний", Assignee: "МЛ", Points: 3, Comments: 2, Label: "Frontend", CreatedAt: now},
		{ID: "ORB-137", Title: "Сценарий приглашения участников", Status: "review", Priority: "Высокий", Assignee: "ЕС", Points: 5, Comments: 7, Attachments: 3, Label: "Продукт", CreatedAt: now},
		{ID: "ORB-145", Title: "Экспорт отчёта в CSV", Status: "review", Priority: "Низкий", Assignee: "ДР", Points: 3, Comments: 4, Attachments: 1, Label: "Backend", CreatedAt: now},
		{ID: "ORB-121", Title: "Единая система уведомлений", Status: "done", Priority: "Средний", Assignee: "АК", Points: 8, Comments: 10, Attachments: 2, Label: "Platform", CreatedAt: now},
		{ID: "ORB-129", Title: "Профиль и часовой пояс", Status: "done", Priority: "Низкий", Assignee: "МЛ", Points: 3, Comments: 2, Label: "Frontend", CreatedAt: now},
	}
}

func main() {
	dataPath := os.Getenv("SPRINTLY_DATA")
	if dataPath == "" {
		dataPath = filepath.Join("data", "issues.json")
	}
	store, err := NewStore(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("SPRINTLY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: API{store: store}.routes(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Sprintly API listening on http://localhost%s", addr)
	log.Fatal(server.ListenAndServe())
}
