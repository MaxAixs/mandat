# Contacts Admin — Design

## Goal

Editors manage the contacts of the `/contacts` page («Наші контакти») from the admin
panel: add, edit, delete and reorder them. Contacts move from
`frontend/src/constants/contacts.ts` into PostgreSQL.

## Decisions (agreed with the user)

- Only the `/contacts` page reads the managed list. The footer social icons and the
  «Соціальні мережі» block of `/advertising` stay in frontend constants.
- A contact is a type picked from a fixed list plus a value; the icon and the link
  are derived from the type, and the value is validated per type.
- Ordering: explicit, changed by ↑/↓ buttons (same as ad format cards).

## Contact types

| type        | value entered                        | stored     | display      | href                               |
|-------------|--------------------------------------|------------|--------------|------------------------------------|
| `email`     | `name@example.com`                   | trimmed    | value        | `mailto:<value>`                   |
| `telegram`  | `name`, `@name`, `t.me/name` URL     | `name`     | `@name`      | `https://t.me/<name>`              |
| `instagram` | `name`, `@name`, instagram.com URL   | `name`     | `@name`      | `https://www.instagram.com/<name>` |
| `threads`   | `name`, `@name`, threads.net URL     | `name`     | `@name`      | `https://www.threads.net/@<name>`  |
| `facebook`  | `name`, facebook.com URL             | `name`     | `name`       | `https://www.facebook.com/<name>`  |
| `phone`     | `+380 44 123 45 67`                  | as entered | value        | `tel:+380441234567`                |
| `website`   | `https://…`                          | as entered | host         | value                              |

Validation (service):
- email — `net/mail` address without a display name, ≤ 254 chars;
- telegram — `[A-Za-z0-9_]{5,32}`; instagram/threads — `[A-Za-z0-9._]{1,30}`;
  facebook — `[A-Za-z0-9.]{5,50}`; URL forms are accepted only for the type's own host;
- phone — `+`, digits, spaces, `-`, `(`, `)`; 7–15 digits;
- website — absolute `https://` URL with a host, ≤ 200 chars.

Optional `label` (≤ 40 runes, trimmed) replaces the type name on the page, e.g. two
emails «Реклама» and «Редакція».

## Data

Migration `00014_contacts.sql`:

```sql
CREATE TABLE contacts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type       TEXT NOT NULL CHECK (type IN ('email','telegram','instagram','threads','facebook','phone','website')),
    value      TEXT NOT NULL CHECK (char_length(value) BETWEEN 1 AND 254),
    label      TEXT NOT NULL DEFAULT '' CHECK (char_length(label) <= 40),
    position   INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT contacts_position_key UNIQUE (position) DEFERRABLE INITIALLY DEFERRED
);
```

Up seeds the two current contacts: `email mandat@gmail.com` (1), `telegram mandat` (2).
It only inserts rows. Down drops the table.

## Backend

- `models.Contact{ID, Type, Value, Label, Position, CreatedAt, UpdatedAt}`,
  `models.ContactType` constants.
- `errs.ErrContactNotFound` → 404 «Contact not found» in `errResps`.
- Repository `postgres.ContactRepo`: `List` (by position), `Create` (appends),
  `Update` (type, value, label, updated_at), `Delete`, `Move(id, up)` — same
  transactional neighbour swap as `AdFormatRepo.Move`; no neighbour is a no-op.
- Service `ContactService`: validates and normalizes input (table above), builds
  `Href` and `Display` for responses (`ContactView` or methods on the model's
  service-level wrapper), reuses `MoveUp`/`MoveDown`.
- HTTP (admin + moderator, same middleware as ad formats):
  - `GET    /api/admin/contacts`
  - `POST   /api/admin/contacts`       JSON `{"type","value","label"}`
  - `PUT    /api/admin/contacts/:id`   same body
  - `DELETE /api/admin/contacts/:id`
  - `POST   /api/admin/contacts/:id/move` JSON `{"direction":"up"|"down"}`
  - public `GET /api/contacts` → `{"items":[...]}`
  - item: `{id, type, value, label, display, href, position}`.

## Frontend

- `Contact`, `ContactType`, `ContactFormValues` types; `src/api/contacts.ts`.
- Admin: «Контакти» section next to «Новини» and «Реклама» in the section nav.
  `ContactsPanel`: rows with icon, type/label, display value, ↑/↓, edit, delete;
  «Додати контакт» opens `ContactEditorDialog` (type select, value input with a
  per-type placeholder, optional label). Server validation messages are shown in the dialog.
- `/contacts` loads `GET /api/contacts`; `CONTACTS` constant is removed. Icons:
  `SocialIcon` for telegram/instagram/threads/facebook, lucide `Mail`/`Phone`/`Globe`
  for the rest. While loading or on error the grid is not shown.

## Out of scope

Footer and `/advertising` social links, follower counts, hiding a contact without deleting it.
