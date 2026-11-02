# Main Event Per Category — Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Одна головна новина (`main_event`) на категорію: нова міграція, ротація в межах категорії в репозиторії.

**Architecture:** Частковий унікальний індекс `(category) WHERE status='published' AND main_event=TRUE` як страховка від гонок; проактивне гасіння прапора попередньої головної тієї ж категорії в транзакції `withPublishedFlagsLock`. Сервіс і транспорт не змінюються (поле вже проходить наскрізь).

**Tech Stack:** Go 1.27, pgx/v5, goose, PostgreSQL 16.

**Spec:** `docs/superpowers/specs/2026-11-02-main-event-per-category-design.md`

## Global Constraints

- Clean Architecture: бізнес-логіка не залежить від інфраструктури; репозиторій без бізнес-логіки понад ротацію прапорів (існуючий патерн).
- Кожна функція service/repo шарів починається з `const fc = "..."`; помилки wrap через `%w` з `fc`.
- Домена класифікація помилок через `internal/errs` (`ErrConflict` уже існує); жодних деталей БД назовні.
- Логування тільки в сервісному шарі (`log/slog`); репозиторій не логує.
- Параметризовані запити; жодної конкатенації в SQL.
- НЕ комітити і НЕ пушити без явної команди користувача (кроки комітів у план не входять).
- Тести пише сабагент `test-author`; основна сесія тестові файли (`*_test.go`) не чіпає. Кроки з тестами = диспетч `test-author`, потім запуск.
- Go: `gofmt`, `go vet ./...`, `go test ./...` зелені. Інтеграційні тести репозиторію — лише проти окремої БД `mandat_test` через `DATABASE_URL` (вони truncate `news`).

## Review Focus

- Два паралельні `Create` з `main_event=TRUE` в одній категорії: друга перемагає без 500, перша гасне (гонка ловиться індексом + транзакцією).
- `Update`, що одночасно міняє категорію і ставить `main_event=TRUE`: гаситься слот НОВОЇ категорії, стара лишається вакантною (рішення зі спеки).
- `Update` чернетки з `main_event=TRUE`: прапор зберігається на рядку, ротації інших не відбувається (статус не published).
- Пряма вставка другого published main в тій же категорії через SQL (в обхід ротації): `ErrConflict` зі страховки індексу, дані не змінюються.
- Існуюча поведінка `featured` (глобальна ротація) не змінюється.

---

### Task 1: Міграція 00008

**Files:**
- Create: `backend/migrations/00008_main_event_per_category.sql`
- Test: `backend/internal/repository/postgres/news_integration_test.go` (через `test-author`)

**Interfaces:**
- Consumes: goose-формат існуючих міграцій (`-- +goose Up/Down`).
- Produces: застосований індекс `idx_news_single_published_main_event_per_category`.

- [ ] **Step 1: test-author пише падаючий інтеграційний тест**

Тест `TestMainEventPerCategory` у `news_integration_test.go`: створити опубліковані main в «Спорт» і в «Новини» → обидва лишаються `main_event=TRUE`; створити другий опублікований main в «Спорт» → перший спортивний згас, новинний не чіпано. Очікувано: FAIL (немає міграції 00008 / стара глобальна унікальність).

- [ ] **Step 2: Запустити новий тест, переконатися що падає**

Run: `docker compose up -d db` (з `backend/`), потім `DATABASE_URL=postgres://mandat:mandat@localhost:5432/mandat_test?sslmode=disable go test ./internal/repository/postgres/ -run TestMainEventPerCategory -v`
Expected: FAIL

- [ ] **Step 3: Створити `migrations/00008_main_event_per_category.sql`**

Up: `DROP INDEX IF EXISTS idx_news_single_published_main_event;` + `CREATE UNIQUE INDEX idx_news_single_published_main_event_per_category ON news (category) WHERE status = 'published' AND main_event = TRUE;`. Down: навпаки (drop нового, create старого глобального `ON news (status) WHERE ...` — текст взяти з `00007`).

- [ ] **Step 4: Запустити тест знову**

Run: та ж команда з кроку 2 (міграції застосовуються автоматично при старті тестового пулу — як у існуючому файлі).
Expected: PASS

---

### Task 2: Ротація main_event в межах категорії

**Files:**
- Modify: `backend/internal/repository/postgres/news.go` (`Create:40-57`, `Update:197-214`, `withPublishedFlagsLock:240-282`)
- Test: `backend/internal/repository/postgres/news_integration_test.go` (через `test-author`)

**Interfaces:**
- Consumes: індекс з Task 1; `models.Category`, `models.StatusPublished`; `mapError` (23505 → `ErrConflict`) без змін.
- Produces: `withPublishedFlagsLock(ctx context.Context, category models.Category, featured bool, mainEvent bool, write func(pgx.Tx) error)`.

- [ ] **Step 1: test-author пише падаючі тести ротації**

  - `Create` другого published main в тій же категорії гасить перший, інша категорія не чіпається.
  - `Update` зі зміною категорії + `main_event=TRUE` гасить слот нової категорії.
  - `Update` чернетки з `main_event=TRUE` нікого не гасить.
  - Прямий SQL-`INSERT` другого published main в тій же категорії повертає `ErrConflict` (перевірка індексу-страховки).
  - `featured=TRUE` як і раніше гасить глобально (регресія не ламається).
  Очікувано: FAIL (сигнатура/поведінка стара).

- [ ] **Step 2: Запустити, переконатися що падає**

Run: `DATABASE_URL=... go test ./internal/repository/postgres/ -v`
Expected: FAIL на нових тестах

- [ ] **Step 3: Змінити `withPublishedFlagsLock` у `news.go`**

Нова сигнатура з `category models.Category` другим параметром. Гасіння featured — без змін (глобально). Гасіння main_event — тільки своя категорія:
`UPDATE news SET main_event = FALSE WHERE status = 'published' AND main_event = TRUE AND category = $1` з аргументом `category`. Колбеки `Create`/`Update` передають `news.Category`. `fc`-константи і wrap помилок — за існуючим стилем.

- [ ] **Step 4: Запустити тести**

Run: `gofmt -l internal/ && go vet ./... && DATABASE_URL=... go test ./internal/repository/postgres/ -v`
Expected: PASS, `gofmt` без виводу

---

### Task 3: Фінальна перевірка бекенда

**Files:** жодних змін, тільки запуски.

- [ ] **Step 1: Повний юніт-набір**

Run: `go test ./...` (з `backend/`, без `DATABASE_URL` — інтеграційні пропускаються)
Expected: PASS

- [ ] **Step 2: Інтеграційні тести репозиторію**

Run: `DATABASE_URL=postgres://mandat:mandat@localhost:5432/mandat_test?sslmode=disable go test ./...`
Expected: PASS (БД `mandat_test` — окрема, дані `news` в ній тестові)
