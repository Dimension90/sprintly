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

	"github.com/jackc/pgx/v5"
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
	SprintID    string    `json:"sprintId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Sprint struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal"`
	State     string    `json:"state"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

type IssueStore interface {
	List(context.Context) ([]Issue, error)
	ListSprints(context.Context) ([]Sprint, error)
	CreateSprint(context.Context, Sprint) (Sprint, error)
	StartSprint(context.Context, string) (Sprint, error)
	CompleteSprint(context.Context, string) (Sprint, error)
	Create(context.Context, Issue) (Issue, error)
	Update(context.Context, string, Issue) (Issue, error)
	Delete(context.Context, string) error
}

type PostgresStore struct{ pool *pgxpool.Pool }

var (
	errNotFound     = errors.New("not found")
	errInvalidState = errors.New("invalid state")
	errActiveSprint = errors.New("an active sprint already exists")
)

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
		`CREATE SEQUENCE IF NOT EXISTS sprint_number_seq START WITH 25`,
		`CREATE TABLE IF NOT EXISTS sprints (
			id text PRIMARY KEY,
			name text NOT NULL,
			goal text NOT NULL DEFAULT '',
			state text NOT NULL CHECK (state IN ('planned','active','completed')),
			start_date timestamptz NOT NULL,
			end_date timestamptz NOT NULL
		)`,
		`INSERT INTO sprints (id,name,goal,state,start_date,end_date) VALUES ('SPR-24','Спринт 24','Стабильный релиз продукта','active','2026-09-02T00:00:00Z','2026-09-15T23:59:59Z') ON CONFLICT (id) DO NOTHING`,
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
		`ALTER TABLE issues ADD COLUMN IF NOT EXISTS sprint_id text DEFAULT 'SPR-24'`,
		`ALTER TABLE issues ALTER COLUMN sprint_id DROP DEFAULT`,
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
			_, err := s.pool.Exec(ctx, `INSERT INTO issues (id,title,status,priority,assignee,points,comments,attachments,label,sprint_id,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, issue.ID, issue.Title, issue.Status, issue.Priority, issue.Assignee, issue.Points, issue.Comments, issue.Attachments, issue.Label, issue.SprintID, issue.CreatedAt)
			if err != nil {
				return fmt.Errorf("seed: %w", err)
			}
		}
	}
	return nil
}

func (s *PostgresStore) List(ctx context.Context) ([]Issue, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,title,status,priority,assignee,points,comments,attachments,label,COALESCE(sprint_id,''),created_at FROM issues ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := make([]Issue, 0, 32)
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.Title, &issue.Status, &issue.Priority, &issue.Assignee, &issue.Points, &issue.Comments, &issue.Attachments, &issue.Label, &issue.SprintID, &issue.CreatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (s *PostgresStore) ListSprints(ctx context.Context) ([]Sprint, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,goal,state,start_date,end_date FROM sprints ORDER BY start_date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sprints []Sprint
	for rows.Next() {
		var sprint Sprint
		if err := rows.Scan(&sprint.ID, &sprint.Name, &sprint.Goal, &sprint.State, &sprint.StartDate, &sprint.EndDate); err != nil {
			return nil, err
		}
		sprints = append(sprints, sprint)
	}
	return sprints, rows.Err()
}

func (s *PostgresStore) CreateSprint(ctx context.Context, sprint Sprint) (Sprint, error) {
	sprint.Name = strings.TrimSpace(sprint.Name)
	sprint.Goal = strings.TrimSpace(sprint.Goal)
	if sprint.Name == "" {
		return Sprint{}, errors.New("name is required")
	}
	if sprint.StartDate.IsZero() || sprint.EndDate.IsZero() || !sprint.EndDate.After(sprint.StartDate) {
		return Sprint{}, errors.New("invalid sprint dates")
	}
	sprint.State = "planned"
	err := s.pool.QueryRow(ctx, `INSERT INTO sprints (id,name,goal,state,start_date,end_date) VALUES ('SPR-' || nextval('sprint_number_seq'),$1,$2,$3,$4,$5) RETURNING id`, sprint.Name, sprint.Goal, sprint.State, sprint.StartDate, sprint.EndDate).Scan(&sprint.ID)
	return sprint, err
}

func (s *PostgresStore) StartSprint(ctx context.Context, id string) (Sprint, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Sprint{}, err
	}
	defer tx.Rollback(ctx)
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM sprints WHERE state='active' AND id<>$1`, id).Scan(&active); err != nil {
		return Sprint{}, err
	}
	if active > 0 {
		return Sprint{}, errActiveSprint
	}
	var sprint Sprint
	err = tx.QueryRow(ctx, `UPDATE sprints SET state='active' WHERE id=$1 AND state='planned' RETURNING id,name,goal,state,start_date,end_date`, id).Scan(&sprint.ID, &sprint.Name, &sprint.Goal, &sprint.State, &sprint.StartDate, &sprint.EndDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Sprint{}, errInvalidState
	}
	if err != nil {
		return Sprint{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Sprint{}, err
	}
	return sprint, nil
}

func (s *PostgresStore) CompleteSprint(ctx context.Context, id string) (Sprint, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Sprint{}, err
	}
	defer tx.Rollback(ctx)
	var sprint Sprint
	err = tx.QueryRow(ctx, `UPDATE sprints SET state='completed' WHERE id=$1 AND state='active' RETURNING id,name,goal,state,start_date,end_date`, id).Scan(&sprint.ID, &sprint.Name, &sprint.Goal, &sprint.State, &sprint.StartDate, &sprint.EndDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Sprint{}, errInvalidState
	}
	if err != nil {
		return Sprint{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE issues SET sprint_id=NULL WHERE sprint_id=$1 AND status<>'done'`, id); err != nil {
		return Sprint{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Sprint{}, err
	}
	return sprint, nil
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
	err := s.pool.QueryRow(ctx, `INSERT INTO issues (id,title,status,priority,assignee,points,comments,attachments,label,sprint_id) VALUES ('ORB-' || nextval('issue_number_seq'),$1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')) RETURNING id,created_at`, issue.Title, issue.Status, issue.Priority, issue.Assignee, issue.Points, issue.Comments, issue.Attachments, issue.Label, issue.SprintID).Scan(&issue.ID, &issue.CreatedAt)
	return issue, err
}

func (s *PostgresStore) Update(ctx context.Context, id string, input Issue) (Issue, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Issue{}, errors.New("title is required")
	}
	if !validStatus(input.Status) {
		return Issue{}, errors.New("invalid status")
	}
	var issue Issue
	err := s.pool.QueryRow(ctx, `UPDATE issues SET title=$2,status=$3,priority=$4,assignee=$5,points=$6,label=$7,sprint_id=NULLIF($8,'') WHERE id=$1 RETURNING id,title,status,priority,assignee,points,comments,attachments,label,COALESCE(sprint_id,''),created_at`, id, input.Title, input.Status, input.Priority, input.Assignee, input.Points, input.Label, input.SprintID).Scan(&issue.ID, &issue.Title, &issue.Status, &issue.Priority, &issue.Assignee, &issue.Points, &issue.Comments, &issue.Attachments, &issue.Label, &issue.SprintID, &issue.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Issue{}, errors.New("issue not found")
	}
	return issue, err
}

func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM issues WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("issue not found")
	}
	return nil
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
	mux.HandleFunc("GET /api/sprints", api.listSprints)
	mux.HandleFunc("POST /api/sprints", api.createSprint)
	mux.HandleFunc("POST /api/sprints/{id}/start", api.startSprint)
	mux.HandleFunc("POST /api/sprints/{id}/complete", api.completeSprint)
	mux.HandleFunc("POST /api/issues", api.createIssue)
	mux.HandleFunc("PUT /api/issues/{id}", api.updateIssue)
	mux.HandleFunc("DELETE /api/issues/{id}", api.deleteIssue)
	return withLogging(mux)
}

func (api API) listSprints(w http.ResponseWriter, r *http.Request) {
	sprints, err := api.store.ListSprints(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sprints")
		return
	}
	writeJSON(w, http.StatusOK, sprints)
}

func (api API) createSprint(w http.ResponseWriter, r *http.Request) {
	var input Sprint
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	sprint, err := api.store.CreateSprint(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sprint)
}

func (api API) startSprint(w http.ResponseWriter, r *http.Request) {
	sprint, err := api.store.StartSprint(r.Context(), r.PathValue("id"))
	if err != nil {
		writeSprintError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sprint)
}

func (api API) completeSprint(w http.ResponseWriter, r *http.Request) {
	sprint, err := api.store.CompleteSprint(r.Context(), r.PathValue("id"))
	if err != nil {
		writeSprintError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sprint)
}

func writeSprintError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, errInvalidState), errors.Is(err, errActiveSprint):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "sprint operation failed")
	}
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

func (api API) updateIssue(w http.ResponseWriter, r *http.Request) {
	var input Issue
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	issue, err := api.store.Update(r.Context(), r.PathValue("id"), input)
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

func (api API) deleteIssue(w http.ResponseWriter, r *http.Request) {
	err := api.store.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "issue not found" {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		{ID: "ORB-142", Title: "Обновить онбординг для новых команд", Status: "backlog", Priority: "Высокий", Assignee: "АК", Points: 5, Comments: 8, Attachments: 2, Label: "Продукт", SprintID: "SPR-24", CreatedAt: now},
		{ID: "ORB-156", Title: "Добавить быстрые фильтры на доску", Status: "backlog", Priority: "Средний", Assignee: "МЛ", Points: 3, Comments: 3, Label: "UX", SprintID: "SPR-24", CreatedAt: now.Add(time.Second)},
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
