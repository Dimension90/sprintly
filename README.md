# Sprintly

Jira-подобный трекер задач: интерактивная канбан-доска на React, API на Go 1.23 и PostgreSQL.

## Возможности

- поиск по ключу, названию и метке;
- создание задач через диалог;
- перенос карточек между статусами drag-and-drop;
- адаптивная доска для телефона и десктопа;
- Go REST API с хранением задач в PostgreSQL;
- тесты создания и смены статуса задачи.

## Запуск через Docker Compose

```powershell
pnpm build
docker compose up --build
```

После запуска откройте `http://localhost:3000`. Данные сохраняются в Docker volume `postgres-data` и не пропадают при перезапуске контейнеров.

Остановка:

```powershell
docker compose down
```

## Запуск без Docker

Фронтенд:

```powershell
pnpm install
pnpm dev
```

Go API (из второго терминала, при доступном PostgreSQL):

```powershell
cd backend
$env:DATABASE_URL = "postgres://sprintly:sprintly@localhost:5432/sprintly?sslmode=disable"
go run .
```

API доступен на `http://localhost:8080`: `GET /api/issues`, `POST /api/issues`, `PUT /api/issues/{id}`, `DELETE /api/issues/{id}`.
