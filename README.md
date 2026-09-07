# Sprintly

Jira-подобный трекер задач: интерактивная канбан-доска на React, API на Go 1.23 и PostgreSQL.

## Возможности

- поиск по ключу, названию и метке;
- светлая и тёмная темы с сохранением выбора;
- фильтры по статусу, приоритету и исполнителю;
- представления «Доска» и «Список»;
- создание задач через диалог;
- перенос карточек между статусами drag-and-drop;
- адаптивная доска для телефона и десктопа;
- Go REST API с хранением задач в PostgreSQL;
- тесты создания и смены статуса задачи.

## Запуск через Docker Compose

```powershell
docker compose up --build
```

После запуска откройте `http://localhost:3000`. Данные сохраняются в Docker volume `postgres-data` и не пропадают при перезапуске контейнеров.

Остановка:

```powershell
docker compose down
```

API доступен на `http://localhost:8080`: `GET /api/issues`, `POST /api/issues`, `PUT /api/issues/{id}`, `DELETE /api/issues/{id}`.
