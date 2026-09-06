# Sprintly

Jira-подобный трекер задач: интерактивная канбан-доска на React и API на Go 1.23 без внешних зависимостей.

## Возможности

- поиск по ключу, названию и метке;
- создание задач через диалог;
- перенос карточек между статусами drag-and-drop;
- адаптивная доска для телефона и десктопа;
- Go REST API с атомарным сохранением в JSON;
- тесты создания и смены статуса задачи.

## Запуск через Docker Compose

```powershell
pnpm build
docker compose up --build
```

После запуска откройте `http://localhost:3000`. Данные сохраняются в Docker volume `sprintly-data` и не пропадают при перезапуске контейнеров.

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

Go API (из второго терминала):

```powershell
cd backend
go run .
```

API доступен на `http://localhost:8080`: `GET /api/issues`, `POST /api/issues`, `PATCH /api/issues/{id}/status`.
