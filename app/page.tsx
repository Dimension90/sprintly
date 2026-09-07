'use client';

import { useEffect, useMemo, useState, type DragEvent } from 'react';
import {
  BarChart3, Bell, CheckCircle2, ChevronDown, Circle, Clock3, Eye, Filter,
  Inbox, Layers3, LayoutDashboard, Menu, MessageSquare, MoreHorizontal,
  Paperclip, Plus, Rocket, Search, Settings, Trash2, Users,
} from 'lucide-react';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import {
  Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle, DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';

type Status = 'backlog' | 'progress' | 'review' | 'done';
type Priority = 'Высокий' | 'Средний' | 'Низкий';
type Issue = { id: string; title: string; status: Status; priority: Priority; assignee: string; points: number; comments: number; attachments: number; label?: string };

type ModelTool = { name: string; title: string; description: string; inputSchema: object; annotations: { readOnlyHint: boolean; untrustedContentHint: boolean }; execute(input: unknown): unknown };
declare global { interface Document { modelContext?: { registerTool(tool: ModelTool, options?: { signal?: AbortSignal }): void | Promise<void> } } }

const columns: { id: Status; title: string; hint: string }[] = [
  { id: 'backlog', title: 'К работе', hint: 'Приоритеты спринта' },
  { id: 'progress', title: 'В работе', hint: 'Лимит: 4 задачи' },
  { id: 'review', title: 'На проверке', hint: 'Нужна реакция' },
  { id: 'done', title: 'Готово', hint: 'За эту неделю' },
];

const initialIssues: Issue[] = [
  { id: 'ORB-142', title: 'Обновить онбординг для новых команд', status: 'backlog', priority: 'Высокий', assignee: 'АК', points: 5, comments: 8, attachments: 2, label: 'Продукт' },
  { id: 'ORB-156', title: 'Добавить быстрые фильтры на доску', status: 'backlog', priority: 'Средний', assignee: 'МЛ', points: 3, comments: 3, attachments: 0, label: 'UX' },
  { id: 'ORB-161', title: 'Тексты пустых состояний', status: 'backlog', priority: 'Низкий', assignee: 'ЕС', points: 2, comments: 1, attachments: 1, label: 'Контент' },
  { id: 'ORB-133', title: 'Новый экран аналитики спринта', status: 'progress', priority: 'Высокий', assignee: 'ДР', points: 8, comments: 12, attachments: 4, label: 'Дизайн' },
  { id: 'ORB-149', title: 'Оптимизировать загрузку карточек', status: 'progress', priority: 'Средний', assignee: 'АК', points: 5, comments: 5, attachments: 1, label: 'Backend' },
  { id: 'ORB-152', title: 'Поддержка drag-and-drop на мобильных', status: 'progress', priority: 'Средний', assignee: 'МЛ', points: 3, comments: 2, attachments: 0, label: 'Frontend' },
  { id: 'ORB-137', title: 'Сценарий приглашения участников', status: 'review', priority: 'Высокий', assignee: 'ЕС', points: 5, comments: 7, attachments: 3, label: 'Продукт' },
  { id: 'ORB-145', title: 'Экспорт отчёта в CSV', status: 'review', priority: 'Низкий', assignee: 'ДР', points: 3, comments: 4, attachments: 1, label: 'Backend' },
  { id: 'ORB-121', title: 'Единая система уведомлений', status: 'done', priority: 'Средний', assignee: 'АК', points: 8, comments: 10, attachments: 2, label: 'Platform' },
  { id: 'ORB-129', title: 'Профиль и часовой пояс', status: 'done', priority: 'Низкий', assignee: 'МЛ', points: 3, comments: 2, attachments: 0, label: 'Frontend' },
];

const avatarColors: Record<string, string> = { АК: 'bg-[#d7ff64] text-[#203100]', МЛ: 'bg-[#ffd5eb] text-[#7e1a50]', ЕС: 'bg-[#c9e2ff] text-[#104d80]', ДР: 'bg-[#ded5ff] text-[#3f2b8a]' };

function BrandMark() { return <div className="brand-mark" aria-hidden="true"><span /><span /><span /></div>; }

function IssueCard({ issue, onDragStart, onOpen }: { issue: Issue; onDragStart: (event: DragEvent, id: string) => void; onOpen: (issue: Issue) => void }) {
  const priorityClass = issue.priority === 'Высокий' ? 'priority-high' : issue.priority === 'Средний' ? 'priority-medium' : 'priority-low';
  return (
    <article className="issue-card" draggable onDragStart={(event) => onDragStart(event, issue.id)} onClick={() => onOpen(issue)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') onOpen(issue); }} tabIndex={0}>
      <div className="issue-card-top"><span className="issue-kind" title="Задача"><CheckCircle2 /></span><span className="issue-id">{issue.id}</span><span className={`priority-badge ${priorityClass}`}>{issue.priority}</span><button className="icon-button compact" onClick={(event) => event.stopPropagation()} aria-label={`Действия задачи ${issue.id}`}><MoreHorizontal /></button></div>
      <h3>{issue.title}</h3>
      {issue.label && <span className="issue-label">{issue.label}</span>}
      <div className="issue-card-bottom"><Avatar size="sm"><AvatarFallback className={avatarColors[issue.assignee]}>{issue.assignee}</AvatarFallback></Avatar><span className="story-points">{issue.points}</span><span className="meta"><MessageSquare />{issue.comments}</span>{issue.attachments > 0 && <span className="meta"><Paperclip />{issue.attachments}</span>}</div>
    </article>
  );
}

function IssueDetails({ issue, onSave, onDelete, onClose }: { issue: Issue; onSave: (issue: Issue) => Promise<void>; onDelete: (id: string) => Promise<void>; onClose: () => void }) {
  const [draft, setDraft] = useState(issue);
  const [saving, setSaving] = useState(false);
  useEffect(() => setDraft(issue), [issue]);
  const save = async () => { setSaving(true); try { await onSave(draft); } finally { setSaving(false); } };
  return (
    <Sheet open onOpenChange={(open) => { if (!open) onClose(); }}>
      <SheetContent className="issue-sheet sm:max-w-[620px]">
        <SheetHeader className="issue-sheet-header"><span className="issue-breadcrumb">Orbit App / {issue.id}</span><SheetTitle>Задача</SheetTitle><SheetDescription>Редактирование сохраняется в PostgreSQL.</SheetDescription></SheetHeader>
        <div className="issue-editor">
          <label className="editor-field full">Название<Input value={draft.title} onChange={(event) => setDraft({ ...draft, title: event.target.value })} /></label>
          <div className="editor-section-title">Сведения</div>
          <label className="editor-field">Статус<Select value={draft.status} onValueChange={(value) => setDraft({ ...draft, status: value as Status })}><SelectTrigger className="editor-select"><SelectValue /></SelectTrigger><SelectContent>{columns.map((column) => <SelectItem key={column.id} value={column.id}>{column.title}</SelectItem>)}</SelectContent></Select></label>
          <label className="editor-field">Приоритет<Select value={draft.priority} onValueChange={(value) => setDraft({ ...draft, priority: value as Priority })}><SelectTrigger className="editor-select"><SelectValue /></SelectTrigger><SelectContent>{(['Высокий','Средний','Низкий'] as Priority[]).map((priority) => <SelectItem key={priority} value={priority}>{priority}</SelectItem>)}</SelectContent></Select></label>
          <label className="editor-field">Исполнитель<Select value={draft.assignee} onValueChange={(value) => setDraft({ ...draft, assignee: value as string })}><SelectTrigger className="editor-select"><SelectValue /></SelectTrigger><SelectContent>{Object.keys(avatarColors).map((person) => <SelectItem key={person} value={person}>{person}</SelectItem>)}</SelectContent></Select></label>
          <label className="editor-field">Оценка<Input type="number" min="0" max="100" value={draft.points} onChange={(event) => setDraft({ ...draft, points: Number(event.target.value) })} /></label>
          <label className="editor-field full">Метка<Input value={draft.label ?? ''} onChange={(event) => setDraft({ ...draft, label: event.target.value })} /></label>
        </div>
        <div className="issue-sheet-footer">
          <AlertDialog><AlertDialogTrigger render={<Button variant="destructive" />}><Trash2 />Удалить</AlertDialogTrigger><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Удалить {issue.id}?</AlertDialogTitle><AlertDialogDescription>Задача будет удалена без возможности восстановления.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Отмена</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={() => void onDelete(issue.id)}>Удалить</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
          <Button onClick={() => void save()} disabled={saving || !draft.title.trim()}>{saving ? 'Сохраняем…' : 'Сохранить'}</Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}

export default function Home() {
  const [issues, setIssues] = useState(initialIssues);
  const [query, setQuery] = useState('');
  const [newTitle, setNewTitle] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [mobileNav, setMobileNav] = useState(false);
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    fetch('/api/issues', { signal: controller.signal })
      .then((response) => response.ok ? response.json() : Promise.reject())
      .then((data: Issue[]) => { if (Array.isArray(data) && data.length) setIssues(data); })
      .catch(() => undefined);
    return () => controller.abort();
  }, []);
  const visibleIssues = useMemo(() => { const value = query.trim().toLowerCase(); return value ? issues.filter((issue) => `${issue.id} ${issue.title} ${issue.label}`.toLowerCase().includes(value)) : issues }, [issues, query]);
  const moveIssue = (id: string, status: Status) => {
    const issue = issues.find((item) => item.id === id);
    if (!issue) return;
    const updated = { ...issue, status };
    setIssues((current) => current.map((item) => item.id === id ? updated : item));
    void fetch(`/api/issues/${encodeURIComponent(id)}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(updated) }).catch(() => undefined);
  };
  const onDrop = (event: DragEvent, status: Status) => { event.preventDefault(); const id = event.dataTransfer.getData('text/plain'); if (id) moveIssue(id, status); };
  const createIssue = async () => {
    const title = newTitle.trim(); if (!title) return;
    let created: Issue = { id: `ORB-${170 + issues.length}`, title, status: 'backlog', priority: 'Средний', assignee: 'АК', points: 3, comments: 0, attachments: 0, label: 'Новая задача' };
    try {
      const response = await fetch('/api/issues', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(created) });
      if (response.ok) created = await response.json() as Issue;
    } catch { /* Интерфейс остаётся рабочим, даже если локальный API остановлен. */ }
    setIssues((current) => [created, ...current]);
    setNewTitle(''); setDialogOpen(false);
  };
  const saveIssue = async (draft: Issue) => {
    const response = await fetch(`/api/issues/${encodeURIComponent(draft.id)}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(draft) });
    if (!response.ok) throw new Error('Не удалось сохранить задачу');
    const saved = await response.json() as Issue;
    setIssues((current) => current.map((issue) => issue.id === saved.id ? saved : issue));
    setSelectedIssue(saved);
  };
  const deleteIssue = async (id: string) => {
    const response = await fetch(`/api/issues/${encodeURIComponent(id)}`, { method: 'DELETE' });
    if (!response.ok) throw new Error('Не удалось удалить задачу');
    setIssues((current) => current.filter((issue) => issue.id !== id));
    setSelectedIssue(null);
  };

  useEffect(() => {
    const context = document.modelContext;
    if (!context?.registerTool) return;
    const lifecycle = new AbortController();
    const register = (tool: ModelTool) => { void Promise.resolve(context.registerTool(tool, { signal: lifecycle.signal })).catch(() => undefined); };
    register({
      name: 'create_issue', title: 'Создать задачу', description: 'Создаёт новую задачу в колонке «К работе» на текущей доске.',
      inputSchema: { type: 'object', properties: { title: { type: 'string', minLength: 1 } }, required: ['title'], additionalProperties: false },
      annotations: { readOnlyHint: false, untrustedContentHint: false },
      async execute(input) {
        const title = typeof input === 'object' && input !== null && 'title' in input ? String((input as { title: unknown }).title).trim() : '';
        if (!title) throw new Error('Название задачи обязательно');
        let created: Issue = { id: `ORB-${170 + issues.length}`, title, status: 'backlog', priority: 'Средний', assignee: 'АК', points: 3, comments: 0, attachments: 0, label: 'Новая задача' };
        const response = await fetch('/api/issues', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(created) });
        if (response.ok) created = await response.json() as Issue;
        setIssues((current) => [created, ...current]);
        return { id: created.id, status: created.status };
      },
    });
    register({
      name: 'move_issue', title: 'Переместить задачу', description: 'Меняет статус задачи на текущей канбан-доске.',
      inputSchema: { type: 'object', properties: { id: { type: 'string' }, status: { type: 'string', enum: ['backlog', 'progress', 'review', 'done'] } }, required: ['id', 'status'], additionalProperties: false },
      annotations: { readOnlyHint: false, untrustedContentHint: false },
      async execute(input) {
        const value = input as { id?: unknown; status?: unknown };
        const id = String(value?.id ?? ''); const status = String(value?.status ?? '') as Status;
        if (!issues.some((issue) => issue.id === id)) throw new Error('Задача не найдена');
        if (!columns.some((column) => column.id === status)) throw new Error('Неизвестный статус');
        const current = issues.find((issue) => issue.id === id)!;
        const response = await fetch(`/api/issues/${encodeURIComponent(id)}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...current, status }) });
        if (!response.ok) throw new Error('Не удалось изменить статус');
        setIssues((current) => current.map((issue) => issue.id === id ? { ...issue, status } : issue));
        return { id, status };
      },
    });
    return () => lifecycle.abort();
  }, [issues]);

  return (
    <main className="app-shell">
      <aside className={`sidebar ${mobileNav ? 'sidebar-open' : ''}`}>
        <div className="brand"><BrandMark /><span>Sprintly</span><button className="mobile-close" onClick={() => setMobileNav(false)} aria-label="Закрыть меню">×</button></div>
        <button className="workspace-switcher"><span className="workspace-logo">O</span><span><strong>Orbit Labs</strong><small>Software project</small></span><ChevronDown /></button>
        <nav className="main-nav" aria-label="Основная навигация">
          <a className="active" href="#board"><LayoutDashboard />Доска</a><a href="#backlog"><Inbox />Бэклог<span className="nav-count">18</span></a><a href="#reports"><BarChart3 />Отчёты</a><a href="#releases"><Rocket />Релизы<span className="nav-dot" /></a><a href="#team"><Users />Команда</a>
        </nav>
        <div className="nav-section-label">Рабочие пространства</div>
        <nav className="project-nav" aria-label="Проекты"><a href="#orbit"><span className="project-symbol coral">O</span>Orbit App</a><a href="#website"><span className="project-symbol mint">W</span>Website 2.0</a><a href="#platform"><span className="project-symbol lilac">P</span>Platform</a><button><Plus />Новый проект</button></nav>
        <div className="sidebar-footer"><a href="#settings"><Settings />Настройки</a><div className="profile"><Avatar><AvatarFallback className="bg-[#d7ff64] text-[#203100]">АК</AvatarFallback></Avatar><span><strong>Алексей К.</strong><small>Product lead</small></span><MoreHorizontal /></div></div>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <button className="menu-button" onClick={() => setMobileNav(true)} aria-label="Открыть меню"><Menu /></button>
          <div className="search-box"><Search /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск задач, проектов…" aria-label="Поиск задач" /><kbd>⌘ K</kbd></div>
          <div className="topbar-actions">
            <button className="icon-button has-alert" aria-label="Уведомления"><Bell /></button>
            <div className="team-stack" aria-label="Участники команды"><Avatar size="sm"><AvatarFallback className={avatarColors['ЕС']}>ЕС</AvatarFallback></Avatar><Avatar size="sm"><AvatarFallback className={avatarColors['МЛ']}>МЛ</AvatarFallback></Avatar><Avatar size="sm"><AvatarFallback className={avatarColors['ДР']}>ДР</AvatarFallback></Avatar><span>+5</span></div>
            <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
              <DialogTrigger render={<Button className="create-button" size="lg" />}><Plus />Создать</DialogTrigger>
              <DialogContent className="create-dialog sm:max-w-lg"><DialogHeader><DialogTitle>Новая задача</DialogTitle><DialogDescription>Она появится в колонке «К работе».</DialogDescription></DialogHeader><label className="dialog-field">Название<Input autoFocus value={newTitle} onChange={(event) => setNewTitle(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') createIssue(); }} placeholder="Например, добавить импорт из CSV" /></label><div className="dialog-grid"><label className="dialog-field">Тип<button className="fake-select"><Layers3 />Задача<ChevronDown /></button></label><label className="dialog-field">Приоритет<button className="fake-select"><span className="priority-dot priority-medium" />Средний<ChevronDown /></button></label></div><DialogFooter><DialogClose render={<Button variant="ghost" />}>Отмена</DialogClose><Button onClick={createIssue} disabled={!newTitle.trim()}>Создать задачу</Button></DialogFooter></DialogContent>
            </Dialog>
          </div>
        </header>

        <div className="board-header">
          <div className="eyebrow"><span>Проекты</span><span>/</span><strong>Orbit App</strong></div>
          <div className="title-row"><div><h1>Разработка продукта</h1><p>Спринт 24 · 2–15 сентября</p></div><div className="header-actions"><Button variant="outline"><Filter />Фильтр</Button><Button variant="outline"><MoreHorizontal /></Button></div></div>
          <div className="sprint-strip"><div className="metric"><span className="metric-icon blue"><Clock3 /></span><span><strong>9 дней</strong><small>до завершения</small></span></div><div className="metric"><span className="metric-icon pink"><Layers3 /></span><span><strong>{issues.reduce((sum, issue) => sum + issue.points, 0)} points</strong><small>в текущем спринте</small></span></div><div className="metric progress-metric"><div className="metric-label"><span><strong>68%</strong><small>прогресс спринта</small></span><span>34 / 50</span></div><div className="progress-track"><span /></div></div><div className="view-toggle"><button className="active"><LayoutDashboard />Доска</button><button><Layers3 />Список</button></div></div>
        </div>

        <section className="kanban" id="board" aria-label="Канбан-доска">
          {columns.map((column) => { const columnIssues = visibleIssues.filter((issue) => issue.status === column.id); const ColumnIcon = column.id === 'done' ? CheckCircle2 : column.id === 'review' ? Eye : column.id === 'progress' ? Clock3 : Circle; return (
            <div className={`kanban-column column-${column.id}`} key={column.id} onDragOver={(event) => event.preventDefault()} onDrop={(event) => onDrop(event, column.id)}>
              <div className="column-heading"><div><ColumnIcon /><strong>{column.title}</strong><span>{columnIssues.length}</span></div><button aria-label={`Добавить в ${column.title}`} onClick={() => setDialogOpen(true)}><Plus /></button></div><p className="column-hint">{column.hint}</p><div className="card-stack">{columnIssues.map((issue) => <IssueCard key={issue.id} issue={issue} onOpen={setSelectedIssue} onDragStart={(event, id) => event.dataTransfer.setData('text/plain', id)} />)}{columnIssues.length === 0 && <div className="empty-column">Перетащите задачу сюда</div>}</div><button className="add-issue" onClick={() => setDialogOpen(true)}><Plus />Добавить задачу</button>
            </div> ); })}
        </section>
      </section>
      {selectedIssue && <IssueDetails issue={selectedIssue} onSave={saveIssue} onDelete={deleteIssue} onClose={() => setSelectedIssue(null)} />}
      {mobileNav && <button className="sidebar-backdrop" onClick={() => setMobileNav(false)} aria-label="Закрыть меню" />}
    </main>
  );
}
