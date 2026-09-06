package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Issue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Assignee    string    `json:"assignee"`
	Points      int       `json:"points"`
	Comments    int       `json:"comments"`
	Attachments int       `json:"attachments"`
	Label       string    `json:"label,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type IssueStore interface {
	List(context.Context) ([]Issue, error)
	Create(context.Context, Issue) (Issue, error)
	UpdateStatus(context.Context, string, string) (Issue, error)
}

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	var pool *pgxpool.Pool
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		pool, err = pgxpool.New(ctx, databaseURL)
		if err == nil {
			err = pool.Ping(ctx)
		}
		if err == nil {
			break
		}
		if pool != nil {
			pool.Close()
		}
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	store := &PostgresStore{pool: pool}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() { s.pool.Close() }

func (s *PostgresStore) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE SEQUENCE IF NOT EXISTS issue_number_seq START WITH 171`,
		`CREATE TABLE IF NOT EXISTS issues (
			id text PRIMARY KEY,
			title text NOT NULL CHECK (length(trim(title)) > 0),
			status text NOT NULL CHECK (status IN ('backlog','progress','review','done')),
			priority text NOT NULL DEFAULT 'Средний',
			assignee text NOT NULL DEFAULT 'АК',
			points integer NOT NULL DEFAULT 3 CHECK (points >= 0),
			comments integer NOT NULL DEFAULT 0 CHECK (comments >= 0),
			attachments integer NOT NULL DEFAULT 0 CHECK (attachments >= 0),
			label text NOT NULL DEFAULT '',
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS issues_status_created_idx ON issues(status, created_at)`,
	}
	for _, statement := range statements {
		if _, err := s.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM issues`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		for _, issue := range seedIssues() {
			_, err := s.pool.Exec(ctx, `INSERT INTO issues (id,title,status,priority,assignee,points,comments,attachments,label,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, issue.ID, issue.Title, issue.Status, issue.Priority, issue.Assignee, issue.Points, issue.Comments, issue.Attachments, issue.Label, issue.CreatedAt)
			if err != nil {
				return fmt.Errorf("seed: %w", err)
			}
		}
	}
	return nil
}

func (s *PostgresStore) List(ctx context.Context) ([]Issue, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,title,status,priority,assignee,points,comments,attachments,label,created_at FROM issues ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := make([]Issue, 0, 32)
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.Title, &issue.Status, &issue.Priority, &issue.Assignee, &issue.Points, &issue.Comments, &issue.Attachments, &issue.Label, &issue.CreatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (s *PostgresStore) Create(ctx context.Context, issue Issue) (Issue, error) {
	issue.Title = strings.TrimSpace(issue.Title)
	if issue.Title == "" {
		return Issue{}, errors.New("title is required")
	}
	if issue.Priority == "" {
		issue.Priority = "Средний"
	}
	if issue.Assignee == "" {
		issue.Assignee = "АК"
	}
	if issue.Points == 0 {
		issue.Points = 3
	}
	if issue.Label == "" {
		issue.Label = "Новая задача"
	}
	issue.Status = "backlog"
	err := s.pool.QueryRow(ctx, `INSERT INTO issues (id,title,status,priority,assignee,points,comments,attachments,label) VALUES ('ORB-' || nextval('issue_number_seq'),$1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at`, issue.Title, issue.Status, issue.Priority, issue.Assignee, issue.Points, issue.Comments, issue.Attachments, issue.Label).Scan(&issue.ID, &issue.CreatedAt)
	return issue, err
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, id, status string) (Issue, error) {
	if !validStatus(status) {
		return Issue{}, errors.New("invalid status")
	}
	var issue Issue
	err := s.pool.QueryRow(ctx, `UPDATE issues SET status=$2 WHERE id=$1 RETURNING id,title,status,priority,assignee,points,comments,attachments,label,created_at`, id, status).Scan(&issue.ID, &issue.Title, &issue.Status, &issue.Priority, &issue.Assignee, &issue.Points, &issue.Comments, &issue.Attachments, &issue.Label, &issue.CreatedAt)
	if err != nil && strings.Contains(err.Error(), "no rows") {
		return Issue{}, errors.New("issue not found")
	}
	return issue, err
}

func validStatus(status string) bool {
	return status == "backlog" || status == "progress" || status == "review" || status == "done"
}

type API struct{ store IssueStore }

func (api API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		_, err := api.store.List(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "postgresql"})
	})
	mux.HandleFunc("GET /api/issues", api.listIssues)
	mux.HandleFunc("POST /api/issues", api.createIssue)
	mux.HandleFunc("PATCH /api/issues/{id}/status", api.updateStatus)
	return withLogging(mux)
}

func (api API) listIssues(w http.ResponseWriter, r *http.Request) {
	issues, err := api.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issues")
		return
	}
	writeJSON(w, http.StatusOK, issues)
}

func (api API) createIssue(w http.ResponseWriter, r *http.Request) {
	var input Issue
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	issue, err := api.store.Create(r.Context(), input)
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
	issue, err := api.store.UpdateStatus(r.Context(), r.PathValue("id"), input.Status)
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
	now := time.Now().UTC()
	return []Issue{
		{ID: "ORB-142", Title: "Обновить онбординг для новых команд", Status: "backlog", Priority: "Высокий", Assignee: "АК", Points: 5, Comments: 8, Attachments: 2, Label: "Продукт", CreatedAt: now},
		{ID: "ORB-156", Title: "Добавить быстрые фильтры на доску", Status: "backlog", Priority: "Средний", Assignee: "МЛ", Points: 3, Comments: 3, Label: "UX", CreatedAt: now.Add(time.Second)},
		{ID: "ORB-161", Title: "Тексты пустых состояний", Status: "backlog", Priority: "Низкий", Assignee: "ЕС", Points: 2, Comments: 1, Attachments: 1, Label: "Контент", CreatedAt: now.Add(2 * time.Second)},
		{ID: "ORB-133", Title: "Новый экран аналитики спринта", Status: "progress", Priority: "Высокий", Assignee: "ДР", Points: 8, Comments: 12, Attachments: 4, Label: "Дизайн", CreatedAt: now.Add(3 * time.Second)},
		{ID: "ORB-149", Title: "Оптимизировать загрузку карточек", Status: "progress", Priority: "Средний", Assignee: "АК", Points: 5, Comments: 5, Attachments: 1, Label: "Backend", CreatedAt: now.Add(4 * time.Second)},
		{ID: "ORB-152", Title: "Поддержка drag-and-drop на мобильных", Status: "progress", Priority: "Средний", Assignee: "МЛ", Points: 3, Comments: 2, Label: "Frontend", CreatedAt: now.Add(5 * time.Second)},
		{ID: "ORB-137", Title: "Сценарий приглашения участников", Status: "review", Priority: "Высокий", Assignee: "ЕС", Points: 5, Comments: 7, Attachments: 3, Label: "Продукт", CreatedAt: now.Add(6 * time.Second)},
		{ID: "ORB-145", Title: "Экспорт отчёта в CSV", Status: "review", Priority: "Низкий", Assignee: "ДР", Points: 3, Comments: 4, Attachments: 1, Label: "Backend", CreatedAt: now.Add(7 * time.Second)},
		{ID: "ORB-121", Title: "Единая система уведомлений", Status: "done", Priority: "Средний", Assignee: "АК", Points: 8, Comments: 10, Attachments: 2, Label: "Platform", CreatedAt: now.Add(8 * time.Second)},
		{ID: "ORB-129", Title: "Профиль и часовой пояс", Status: "done", Priority: "Низкий", Assignee: "МЛ", Points: 3, Comments: 2, Label: "Frontend", CreatedAt: now.Add(9 * time.Second)},
	}
}

func main() {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://sprintly:sprintly@localhost:5432/sprintly?sslmode=disable"
	}
	store, err := NewPostgresStore(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	addr := os.Getenv("SPRINTLY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: API{store: store}.routes(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Sprintly API listening on http://localhost%s", addr)
	log.Fatal(server.ListenAndServe())
}
