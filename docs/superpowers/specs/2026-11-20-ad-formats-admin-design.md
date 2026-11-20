# Ad Formats Admin — Design

## Goal

Editors manage the «Формати, з якими ми працюємо» cards of `/advertising` from the
admin panel: add, delete, edit title and description, replace or remove the image,
and reorder with ↑/↓. The cards move from frontend constants into PostgreSQL.

## Decisions (agreed with the user)

- Entity: the ad format cards only (no site banners).
- Admin placement: a separate «Формати реклами» section in the admin sidebar
  (a tab row on mobile); picking it swaps the news table for the ad formats table.
  The news category filter offers news rubrics only.
- Image editing: upload a replacement or remove it; no in-browser cropping.
- Ordering: explicit, changed by ↑/↓ buttons.

## Data

Migration `00013_ad_formats.sql`:

```sql
CREATE TABLE ad_formats (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 80),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 240),
    image_url   TEXT NOT NULL DEFAULT '',
    position    INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ad_formats_position_key UNIQUE (position) DEFERRABLE INITIALLY DEFERRED
);
```

The Up step seeds the 8 cards currently in `frontend/src/constants/advertising.ts`
(positions 1–8, no images). It only inserts rows. Down drops the table.

## Backend

- `models.AdFormat{ID, Title, Description, ImageURL, Position, CreatedAt, UpdatedAt}`.
- `errs.ErrAdFormatNotFound` → 404 «Ad format not found» in `errResps`.
- Repository `postgres.AdFormatRepo`: `List` (by position), `GetByID`, `Create`
  (appends: position = max + 1), `Update` (title, description, image_url, updated_at),
  `Delete`, `Move(id, up)` — swaps positions with the neighbour in one transaction;
  no neighbour is a no-op.
- Service `AdFormatService` (owns `AdFormatRepository`, reuses `ImageStorage`):
  - validation: title trimmed, 1–80 runes; description ≤ 240 runes; direction `up|down`;
  - images reuse the cover checks (`readCover`: extension + sniffed content, 10 MB);
  - keys `ad-formats/<id>-<8 hex>.<ext>`, a new key per upload;
  - create: validate image → insert → upload → save URL; failure removes row/object;
  - update: replace = upload new, save, then delete old owned object;
    `remove_image` = clear URL, save, delete old owned object;
  - delete: remove row, then the owned object (failure only logged).
- HTTP (admin + moderator, same middleware as news):
  - `GET    /api/admin/ad-formats`
  - `POST   /api/admin/ad-formats` multipart: `title`, `description`, file `image`
  - `PUT    /api/admin/ad-formats/:id` multipart: same + `remove_image=true`
  - `DELETE /api/admin/ad-formats/:id`
  - `POST   /api/admin/ad-formats/:id/move` JSON `{"direction":"up"|"down"}`
  - public `GET /api/ad-formats` → `{"items":[...]}`
  - response item: `{id, title, description, image_url, position}`.

## Frontend

- `src/api/adFormats.ts` + `AdFormat` type.
- Admin: «Формати реклами» section in the sidebar nav (admin-only, not a news rubric).
  Selected → `AdFormatsPanel`: table with thumbnail, title, description, ↑/↓,
  edit, delete; «Додати формат» opens `AdFormatEditorDialog` (title, description,
  image upload with preview, «Прибрати зображення»).
- `/advertising` loads cards from `GET /api/ad-formats`; `ADVERTISING_FORMATS`
  constant is removed. While loading or on error the formats grid is not shown.

## Out of scope

Cropping, drafts/hiding of cards, card links other than the existing mailto.
