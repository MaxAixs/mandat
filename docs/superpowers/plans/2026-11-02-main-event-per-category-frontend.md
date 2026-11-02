# Main Event Per Category — Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Головна новина категорії великим блоком на сторінці категорії; Hero головної — найновіша `main_event` серед категорій; перемикач «Головна новина категорії» в адмінці.

**Architecture:** Жодних змін API-контракту (поле `main_event` уже є в публічних відповідях). Мапінг `toArticle` починає нести прапор; `getHomeNews` обирає Hero максимумом за `published_at`; сторінка категорії рендерить головну великою карткою поверх незмінної сітки.

**Tech Stack:** React 19, TypeScript, react-router v7, Tailwind 3.4 (`mandat-*` класи), shadcn/ui (без правок `components/ui`).

**Spec:** `docs/superpowers/specs/2026-11-02-main-event-per-category-design.md`

## Global Constraints

- TypeScript strict; імпорти через `@/`; функціональні компоненти.
- Стилі — тільки Tailwind `mandat-*`; жодних hex-кольорів у компонентах; контент не живе в компонентах.
- НЕ редагувати `components/ui/*`, `components.json`, `tailwind.config.js`, `vite.config.ts` без необхідності (тут не потрібні).
- НЕ комітити і НЕ пушити без явної команди користувача.
- Тести пише сабагент `test-author`; основна сесія файли `tests/*` не чіпає. Кроки з тестами = диспетч `test-author`, потім запуск.
- Перед завершенням: `npm run lint` і `npm run build` зелені. Тест-раннер — як у існуючих тестах (компіляція `tsc` + `node --test`, деталі знає `test-author` з минулої задачі).

## Review Focus

- Категорія без головної: сторінка категорії без великого блока, сітка як зараз.
- Дві `main_event` в одній відповіді API (перехідний стан): Hero/сторінка беруть найновішу за `published_at`.
- `main_event` з `published_at: null`: не ламає вибір максимуму (трактується як найстаріша).
- Великий блок на сторінці категорії веде на `/news/:slug` саме головної статті.
- Перемикач адмінки не впливає на `featured`; бейдж «Подія дня» в списку адмінки лишається за `main_event`.

---

### Task 1: Hero — найновіша головна серед категорій

**Files:**
- Modify: `frontend/src/api/index.ts:110-128` (`getHomeNews`)
- Test: `frontend/tests/public-news-api.test.ts` (через `test-author`)

**Interfaces:**
- Consumes: `publicationTimestamp` (існує, null → `-Infinity`); `PublicNews.main_event`, `published_at`.
- Produces: `HomeNewsData.mainEvent` — найновіша `main_event` зі сторінки, або `null`.

- [ ] **Step 1: test-author пише падаючі тести**

  - Кілька `main_event` у відповіді → `mainEvent` той, у кого найбільший `published_at` (а не перший у відповіді бекенда, чий порядок `main_event DESC, featured DESC, ...`).
  - `main_event` з `published_at: null` програє датованій.
  - Жодної `main_event` → `mainEvent: null` (існуючий тест лишається).
  Очікувано: FAIL (зараз береться перший `find`).

- [ ] **Step 2: Запустити, переконатися що падає**

Run: раннер `test-author` для `tests/public-news-api.test.ts`
Expected: FAIL на нових тестах

- [ ] **Step 3: Змінити вибір `mainEvent` у `getHomeNews`**

Замість `response.items.find((news) => news.main_event)` — відфільтрувати `main_event` і взяти максимум за `publicationTimestamp(news.published_at)`. Решта повернення (`articles`, `latestNews`, `tickerItems`) без змін.

- [ ] **Step 4: Запустити тести**

Run: той же раннер
Expected: PASS

---

### Task 2: Прапор головної в `Article` і великий блок категорії

**Files:**
- Modify: `frontend/src/types/index.ts:8-23` (`Article`), `frontend/src/api/index.ts:52-63` (`toArticle`), `frontend/src/components/NewsCard.tsx` (новий опційний проп), `frontend/src/pages/Category.tsx:10-60` (великий блок зверху)
- Test: `frontend/tests/category-news-api.test.ts`, `frontend/tests/public-news-api.test.ts`, `frontend/tests/category-pages.test.tsx` (через `test-author`)

**Interfaces:**
- Consumes: `PublicNews.main_event: boolean`; `toArticle` мапінг; `NewsCard({ article })`.
- Produces: `Article.mainEvent?: boolean`; `NewsCard({ article, large?: boolean })` — `large` дає ті самі класи великого блока, що зараз дає `featured` (`lg:col-span-2 lg:row-span-2`, заголовок `text-[24px]`, фото `lg:flex-1`); `CategoryPage` рендерить першу статтю з `mainEvent` великою зверху, сітка нижче — всі статті без змін (дублювання за рішенням).

- [ ] **Step 1: test-author пише падаючі тести**

  - `toArticle` мапить `main_event` у `mainEvent` (обидва напрями true/false) — оновити існуючі `deepEqual`-очікування в `category-news-api` і `public-news-api`.
  - `NewsCard` з `large` має класи великого блока без `featured: true` у даних.
  - Сторінка категорії: за наявності статті з `mainEvent` зверху рендериться великий блок з її заголовком і посиланням на `/news/<її slug>`; сітка містить усі статті включно з головною; без `mainEvent` великого блока немає.
  Очікувано: FAIL (поля `mainEvent`/`large` не існує).

- [ ] **Step 2: Запустити, переконатися що падає**

Run: раннер `test-author` для трьох файлів
Expected: FAIL

- [ ] **Step 3: Реалізувати**

`Article` += `mainEvent?: boolean` з коментарем; `toArticle` += `mainEvent: news.main_event`. `NewsCard`: проп `large?: boolean`, внутрішня `const prominent = featured || large` замість прямих звернень до `featured` для розміру/типографіки (дані `featured` не підміняти). `CategoryPage`: `const main = articles.find((a) => a.mainEvent)` → перед сіткою `{main && <NewsCard article={main} large />}`; решта файла без змін.

- [ ] **Step 4: Запустити тести**

Run: той же раннер для трьох файлів
Expected: PASS

---

### Task 3: Перемикач «Головна новина категорії» в адмінці

**Files:**
- Modify: `frontend/src/components/admin/NewsEditorDialog.tsx:215-226` (лейбл)
- Test: `frontend/tests/admin-news-editor.test.tsx` (через `test-author`)

**Interfaces:**
- Consumes: існуючий `Switch id="news-main-event"`, поле `main_event: boolean` — без змін.
- Produces: той самий перемикач з лейблом «Головна новина категорії».

- [ ] **Step 1: test-author оновлює тести під новий лейбл**

Замінити рядок «Головна подія дня» на «Головна новина категорії» в тестах діалога; поведінка (`checked` ↔ `main_event`) та сама. Очікувано: FAIL до зміни компонента (тести шукають новий лейбл).

- [ ] **Step 2: Змінити лейбл у `NewsEditorDialog`**

Тільки текст лейбла; `id`, `checked`, `onCheckedChange`, поле форми — без змін.

- [ ] **Step 3: Запустити тести**

Run: раннер `test-author` для `tests/admin-news-editor.test.tsx`
Expected: PASS

---

### Task 4: Фінальна перевірка фронтенда

**Files:** жодних змін, тільки запуски.

- [ ] **Step 1: Лінт і збірка**

Run: `npm run lint && npm run build` (з `frontend/`)
Expected: обидва PASS без помилок

- [ ] **Step 2: Повний набір тестів**

Run: повний раннер `test-author` по `tests/`
Expected: PASS; виняток — уже відомі 3 падіння `admin-page.test.tsx` про бейдж «Подія дня» (pre-existing, не чіпаємо; якщо впаде щось інше — повернутися до відповідного Task)
