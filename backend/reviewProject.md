# Ревʼю бекенду Mandat

Дата: 2026-11-17. Охоплено: `cmd/`, `internal/` (service, repository, transport, imgdownloader), `pkg/storage/r2`, `migrations/`, `config.yaml`, Docker.
Тести не запускались — це статичне ревʼю коду.

Пріоритети: 🔴 критично (діра / втрата даних) · 🟠 баг або суттєвий ризик · 🟡 покращення · ⚪ рефакторинг/стиль.

## Статус виправлень (2026-11-17)

План: `docs/superpowers/plans/2026-11-17-review-fixes.md`. Тести — `test-author`, ревʼю — `reviewer` (APPROVED, зауваження виправлені).
Перевірка: `gofmt` чисто, `go vet ./...` чисто, `go test -count=1 ./...` зелений; інтеграційні тести PostgreSQL і Redis прогнані на одноразових контейнерах — зелені.

| П. | Статус | Що зроблено |
|----|--------|-------------|
| 1 | ✅ | Справжній bcrypt-хеш (`sync.OnceValue`, `DefaultCost`) для неіснуючих логінів |
| 2 | ✅ | In-memory лімітер видалено; Redis-ліміт `login:ip` (middleware) + `login:user` (сервіс), конфіг `login_rate_limit` |
| 3 | ✅ | Один резолв → дзвінок на перевірену IP; розширений список заборонених діапазонів (`netip`) |
| 4 | ✅ | Ключі `covers/<slug>-<8 hex>.<ext>`; при збої видаляється лише новий об'єкт |
| 5 | ✅ | Slug незмінний при оновленні |
| 6 | ✅ | Вміст обкладинки перевіряється до запису; збій upload після вставки → відкат рядка, 503 |
| 7 | ✅ | `write_timeout` 30s; старт падає, якщо `download.timeout >= write_timeout` |
| 8 | ✅ | `HTTP_SECURE_COOKIES` / `HTTP_ALLOWED_ORIGINS`; старт падає при не-локальному origin без secure cookies |
| 9 | ✅ | nosniff, X-Frame-Options, Referrer-Policy, CSP, HSTS (з secure cookies), `no-store` для admin/auth |
| 10 | ✅ | Origin guard на POST/PUT/PATCH/DELETE під `/api/admin`, `/api/auth` → 403 «Untrusted request origin» |
| 11 | ✅ | 429 «Too many requests»; `errs.RetryAfterError` → `Retry-After` через `WriteError` |
| 12 | ✅ | `errResps` — впорядкований slice |
| 13 | ✅ | Очікувані помилки не логуються як ERROR; `fc` у всіх логах |
| 14 | ⏸ відкладено | Очистка `news_viewers` — деструктивна операція, не погоджена |
| 15 | ✅ | Міграція `00012_news_title_trgm.sql` (pg_trgm + GIN) |
| 16 | ✅ | `has_more` у списках; `ListFilter{Limit, Offset}` |
| 17 | ✅ | Per-user set сесій (з авто-очисткою мертвих), `POST /api/auth/logout-all` |
| 18 | — без змін | Рішення: модератор зберігає DELETE |
| 19 | ✅ | Порти БД/Redis на `127.0.0.1`, `restart: unless-stopped`, `HEALTHCHECK` |
| 20 | ✅ | `rates.refresh_interval`, `rates.http_timeout` у конфігу |
| 21 | ✅ | Фонові горутини в `sync.WaitGroup`, чекаються до закриття Redis/pool |
| 22 | ✅ | `Authorization` прибрано з CORS |
| 23 | ✅ (документація) | Рішення: `status` лишається у відповіді; AGENTS.md §5 оновлено |
| 24–26 | ✅ | `collectNews`, `normalizePage`/`pageFilter`/`pageResult`, `Update` розбитий на хелпери |
| 27–29 | ✅ | `Me` через `CurrentUser`; middleware пишуть помилки через `WriteError`; пакет `routes`; CORS і body-limit перенесені в `httptransport` |
| 30–31 | ✅ | Форматування параметрів і порядок функцій |
| 32–34 | ✅ | `ImageURL`; `pkg/imagetype`; `pkg/storage/r2` → `internal/storage/r2` |
| 35 | ✅ | Коментар `NewNewsService` виправлено; nil storage з обкладинкою → 503 до запису |

---

## 🔴 Діри та втрата даних

### 1. Невалідний `dummyBcryptHash` → енумерація логінів через таймінг
`internal/service/auth.go:22` — рядок має 57 символів, bcrypt-хеш має бути 60. `bcrypt.CompareHashAndPassword` для неіснуючого логіна миттєво повертає `ErrHashTooShort`, а для існуючого рахує bcrypt (~50–100 мс). Різниця в часі відповіді показує, чи існує акаунт.
**Фікс:** згенерувати справжній хеш (`bcrypt.GenerateFromPassword` з тим самим cost, що в реальних акаунтах) один раз при старті або константою з 60 символів; додати тест, що `bcrypt.Cost(dummy)` не повертає помилку.

### 2. Login rate limiter легко блокує всіх (DoS адмінки)
`internal/transport/http/middleware/login_rate_limit.go:14` — `maxTrackedLoginIPs = 10`. Як тільки за 5 хвилин з'являються 10 різних IP, **кожна нова IP отримує 429**. Атакувальник з 10 адресами (або просто 10 ботів) повністю вимикає вхід для редакції.
Крім того: лімітер in-memory (не працює при >1 інстансі, скидається рестартом), а коментар «ten attempts» не відповідає `loginAttemptLimit = 5`.
**Фікс:** перенести на вже готовий `RateLimitStore` у Redis з окремим префіксом (`login:`) і власним лімітом; прибрати глобальну стелю IP. Додатково — ліміт по логіну (`login:user:<login>`), щоб гасити розподілений перебір пароля одного акаунта.

### 3. SSRF: DNS rebinding + неповний фільтр IP
`internal/imgdownloader/img_downloader.go:113-135`
- `dialGuard` резолвить хост, перевіряє IP, а потім `dialer.DialContext(ctx, network, addr)` **резолвить ще раз** по імені. Зловмисний DNS з TTL=0 віддає публічну IP на першу перевірку і `127.0.0.1`/`169.254.169.254` на друге з'єднання.
- `isPublicIP` пропускає `100.64.0.0/10` (CGNAT), `64:ff9b::/96` (NAT64), `2002::/16` (6to4) та інші спеціальні діапазони.

**Фікс:** дзвонити на вже перевірену IP: `dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))`; фільтрувати через `netip.Addr` з явним списком заборонених префіксів. Альтернатива — `net.Dialer.Control`, який перевіряє фактичну IP сокета (захищає і від rebinding, і від редиректів).

### 4. Оновлення обкладинки з тим самим розширенням видаляє стару обкладинку
`internal/service/news.go:383-399` — ключ `covers/<slug>.<ext>` той самий, що й у поточної обкладинки. `Upload` **перезаписує** старий об'єкт; якщо далі `repo.Update` падає, компенсація `storage.Delete(key)` видаляє файл, на який досі посилається стаття → битий cover у продакшні.
Окрім цього, однаковий URL ламає кеш CDN/браузера (нова картинка не видна).
**Фікс:** унікальний ключ на кожне завантаження: `covers/<slug>-<shortid>.<ext>` (або `covers/<uuid>.<ext>`); стару видаляти лише після успішного коміту (так уже зроблено, коли ключі різні).

### 5. Редагування статті зі slug-суфіксом неможливе / slug «стрибає»
`internal/service/news.go:359` — `Update` будує статтю через `buildNews`, який заново робить `slugify(title)`, і **не зберігає `existing.Slug`**.
- Стаття `foo-2` (створена після колізії) при будь-якому редагуванні отримує slug `foo` → `409 Conflict`, бо `foo` вже існує. Статтю фактично неможливо відредагувати.
- Зміна заголовка змінює URL опублікованої статті → биті посилання, SEO, лічильник `news_viewers` по slug.

**Фікс:** при оновленні зберігати `existing.Slug`; якщо потрібна зміна slug — окреме явне поле з перевіркою унікальності (і тим самим циклом суфіксів, що в `Create`).

---

## 🟠 Баги та суттєві ризики

### 6. Помилка завантаження обкладинки при створенні тихо ковтається
`internal/service/news.go:478-494` (`storeImg`) — якщо `Upload` повертає validation error (наприклад, `.jpg`, а всередині PNG, або не картинка), клієнт отримує `201 Created` без обкладинки і без пояснення.
**Фікс:** валідувати вміст (sniff + розширення) **до** вставки в БД, а помилки `ErrInvalidInput`/`ValidationError` повертати клієнту. Для інфраструктурних збоїв R2 — або відкат створення, або явне поле `warnings` у відповіді.

### 7. `WriteTimeout` (10 с) менший за таймаут завантаження картинки (15 с)
`config.yaml` `write_timeout: 10s` vs `download.timeout: 15s` + час на upload у R2. Сервер обрізає відповідь, а стаття при цьому вже створена → адмінка бачить мережеву помилку, натискає ще раз → дубль `foo-2`.
**Фікс:** або піднімати `write_timeout` для адмінських маршрутів (окремий `http.TimeoutHandler`/`ResponseController.SetWriteDeadline`), або тримати download timeout + upload з запасом нижче `write_timeout`.

### 8. `secure_cookies: false` вшито в Docker-образ
`Dockerfile` копіює `config.yaml` з `secure_cookies: false` і `allowed_origins: localhost`. Якщо прод запуститься без override, сесійна кука піде по HTTP.
**Фікс:** env-override (`HTTP_SECURE_COOKIES`, `HTTP_ALLOWED_ORIGINS`) і fail-fast при старті, якщо `secure_cookies=false`, а origin не localhost.

### 9. Немає security headers
AGENTS.md (§11) вимагає, але middleware відсутнє. Додати мінімум: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` / `frame-ancestors 'none'`, `Referrer-Policy: strict-origin-when-cross-origin`, `Strict-Transport-Security` (коли `secure_cookies`), `Cache-Control: no-store` для `/api/admin` і `/api/auth`.

### 10. CSRF покладається лише на `SameSite=Lax`
Lax не захищає від запитів з **того ж сайту** (будь-який субдомен, напр. майбутній `blog.mandat.*`). Мутуючі адмін-маршрути приймають `multipart/form-data`, який браузер шле без preflight.
**Фікс:** перевірка `Origin`/`Sec-Fetch-Site` на POST/PUT/DELETE під `/api/admin` і `/api/auth` (дешево і достатньо), або `SameSite=Strict` для адмін-сесії.

### 11. Повідомлення 429 для глобального ліміту — «Too many login attempts»
`internal/transport/http/errors.go:27` — тому `RateLimit` middleware пише відповідь вручну. Після переносу логін-лімітера (п.2) зробити одне загальне повідомлення «Too many requests» і передавати `Retry-After` через `WriteError` (напр. typed error з `RetryAfter`), прибравши дублювання формату.

### 12. Порядок перебору `errResps` недетермінований
`errResp` ітерує `map`; якщо помилка обгортає два sentinel-и (наприклад `ErrInvalidInput` + `ErrPayloadTooLarge` через `errors.Join`), статус випадковий. Замінити `map` на впорядкований slice.

### 13. Логування очікуваних помилок як `ERROR`
`GetBySlug`, `GetPublishedBySlug`, `Update`, `Delete`, `Create` (fetch) пишуть `ErrorContext` навіть на `ErrNewsNotFound` / validation. Будь-який бот, що перебирає slug-и, засмічує логи. Логувати `ERROR` лише для неочікуваних збоїв (як уже зроблено в `ViewService`/`AuthService`).
Також у `storeImg` відсутній `fc` у логах.

---

## 🟡 Покращення

14. **`news_viewers` росте без меж** — індекс по `last_viewed_at` є, а очистки немає. Періодичний `DELETE ... WHERE last_viewed_at < now() - interval '3 hours'` (раз на годину в фоні) або партиціювання. *(Деструктивна операція — потребує окремого погодження.)*
15. **Пошук `ILIKE '%q%'` без індексу** — seq scan по всій таблиці. Міграція з `pg_trgm` + `GIN (title gin_trgm_ops)`.
16. **OFFSET-пагінація** і відсутність `total`/`has_more` у відповіді — фронт не знає, чи є наступна сторінка. Мінімум: запитувати `limit+1` і віддавати `has_more`; на майбутнє — keyset (`published_at, id`).
17. **Сесії**: лише абсолютний TTL 12 год, немає списку сесій користувача → неможливо «вийти з усіх пристроїв» або відкликати сесії при зміні пароля/ролі. Тримати `admin:user:<id>:sessions` (Redis set).
18. **Ролі**: модератор має `DELETE`. Якщо це не задумано — дозволити видалення лише `RoleAdmin`.
19. **Docker Compose**: Postgres `5432` і Redis `6379` (без пароля) опубліковані на хост — на VPS це відкриті порти. Прив'язати до `127.0.0.1:` або прибрати `ports`. Додати `restart: unless-stopped` і `HEALTHCHECK` для `app`.
20. **Магічні числа в `main.go`**: `10 * time.Second` (http client), `24 * time.Hour` (refresh) → у конфіг (`rates.refresh_interval`, `rates.http_timeout`).
21. **Фонові горутини** (`StartAutoRefresh`, warmup) не чекаються при shutdown — використати `errgroup`/`sync.WaitGroup`, щоб Redis/pool не закривались під ними.
22. **CORS** дозволяє заголовок `Authorization`, який не використовується (авторизація через cookie) — прибрати.
23. **`dto` формат помилки** містить `status`, хоча AGENTS.md §5 визначає лише `message`. Узгодити документацію або відповідь.

---

## ⚪ Рефакторинг і стиль

24. **Дублювання сканування рядків** у `List`, `ListPublished`, `SearchPublished` (`repository/postgres/news.go`) — винести в `collectNews(rows)` або `pgx.CollectRows(rows, pgx.RowToAddrOfStructByPos)`.
25. **Дублювання валідації пагінації** в `NewsService.List` / `ListPublished` — хелпер `normalizePage(page, limit) (int, int, error)`.
26. **`Update` у сервісі** (90 рядків) — змішані: злиття з existing, cover, компенсація. Розбити: `mergeExisting`, `uploadCover`, `replaceCover`; спільний з `storeImg` шлях завантаження.
27. **`AuthHandler.Me`** дублює `middleware.CurrentUser` — використати його.
28. **Формат помилок у middleware** (`abortWithAuthError`, `LoginRateLimiter`, `RateLimit`, `LimitBodySize`) повторює JSON-структуру вручну 4 рази. Викликати `httptransport.WriteError` (або спільний `writeJSONError` в окремому пакеті без циклу імпортів).
29. **Дубльовані константи шляхів**: `uploadPathPrefix` і `healthPathPrefix` у middleware дублюють handler-и через цикл імпортів. Винести route-константи в маленький пакет `transport/http/routes`.
30. **Правило форматування параметрів (AGENTS.md §3)** порушене: `NewsService.List` (5 параметрів в одному рядку), `NewNewsService` тіло-рядок >120, `resolvePastedURL`, `storeImg` (>120 колонок), `GetAdminSessionByTokenHash`.
31. **Порядок функцій**: `ownedCoverKey` (метод) стоїть після хелперів `buildNews`/`coverExt`; `insertNews` над `GetBySlug` — ок, але `updateNews`/`scanNews` розкидані. Привести до правила «методи → хелпери за порядком виклику».
32. **`NewsInput.ImageUrl`** → `ImageURL` (Go naming для абревіатур), так само в моделі/DTO.
33. **`sniffMIME`** продубльований в `imgdownloader` і `r2`; `fetchedExt` у сервісі дублює `mimeForExt` в r2 — одна таблиця підтримуваних форматів у `models`.
34. **`r2` пакет у `pkg/`** імпортує `internal/errs` (коментар це визнає) — перенести в `internal/storage/r2`, як вимагає AGENTS.md для app-specific інтеграцій.
35. **`NewNewsService`**: коментар каже «nil fetcher keeps legacy behavior», а `storage` фактично обов'язковий — валідувати в конструкторі або спростити коментар.

---

## Рекомендований порядок робіт

1. п.1, п.2, п.3 — безпека (невеликі, ізольовані зміни).
2. п.4, п.5, п.6 — цілісність даних статей/обкладинок.
3. п.7–п.10 — конфіг і заголовки перед продом.
4. п.11–п.13, потім 🟡 і ⚪ окремими `[*]` комітами.

Кожен пункт 🔴/🟠 — через TDD: спочатку `test-author` пише тест, що відтворює проблему.
