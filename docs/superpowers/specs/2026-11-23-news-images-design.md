# News Images — Design

## Goal

An article gets up to 10 extra images besides its cover. The editor adds them as files
from the computer or as image links. The public page spreads them through the text
automatically.

## Decisions (agreed with the user)

- The cover stays as it is (one image, set by a link). Images are an additional gallery.
- At most 10 images per article, no captions; the order is set by the editor.
- Sources: files (direct upload to R2) and links (downloaded by the backend, like the cover).
- Placement: automatic between paragraphs, no markers. The video keeps its place in the middle.
- Temporary uploads of images and videos share one prefix `pending/`, so a single R2
  lifecycle rule cleans both. This changes the uncommitted video design: `videos/pending/`
  becomes `pending/videos/`.

## Data

Migration `00016_news_images.sql` (adds a column only, no data changes):

```sql
ALTER TABLE news
    ADD COLUMN image_urls TEXT[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT news_image_urls_max CHECK (cardinality(image_urls) <= 10);
```

Down drops the constraint and the column.

Domain: `models.News` gains `ImageURLs []string` (nil or empty means no images; the order is
the display order).

## Storage

- The service-owned `VideoStorage` interface becomes `MediaStorage` (`PresignPut`, `Upload`,
  `Inspect`, `Copy`, `Delete`), and `WithVideoStorage` becomes `WithMediaStorage`. The same
  `r2.Client` implements it. Every video and image object is created and deleted through
  `MediaStorage`; `ImageStorage` stays for covers.
- Pending keys:
  - videos: `pending/videos/<32 hex>.<mp4|webm>`;
  - images: `pending/images/<32 hex>.<jpg|png|webp|gif>`.
- Article keys: `videos/<slug>-<8 hex>.<ext>` (unchanged) and `images/<slug>-<8 hex>.<ext>`.
- R2 lifecycle: one rule, prefix `pending/`, delete after 1 day.

## Upload flow (files)

1. `POST /api/admin/news/image-uploads` (admin auth) with `{"content_type", "size"}`.
   Image types come from `pkg/imagetype`, and `0 < size ≤ 10 MB` (`MaxCoverBytes`). The
   response is `{"key", "upload_url", "expires_at"}`; the URL lives 15 minutes.
2. The browser PUTs each file to its `upload_url`, several in parallel, with progress.
3. On save, for every pending image key the service checks the key format and inspects the
   object (exists, size, sniffed image type matches the extension). A rejected existing
   pending object is deleted right away, as for video.
4. The service copies the object to `images/<slug>-<8 hex>.<ext>`, saves the article, then
   deletes the pending objects.

## Links

A image given as an external `http(s)` image URL is downloaded by the existing
`ImageFetcher` at save time. It is checked like a cover (size ≤ 10 MB, image MIME) and
uploaded through `MediaStorage.Upload` to `images/<slug>-<8 hex>.<ext>`. Once saved, it is our own
URL, so later saves do not download it again.

## Request semantics

`NewsRequest` gains `images: string[]` (JSON). Multipart forms take repeated `images` fields.
Each item is exactly one of:

| item | meaning |
|---|---|
| `pending/images/<32 hex>.<ext>` | new uploaded file → copy into place |
| one of the article's current `image_urls` | keep as is |
| other `http(s)` URL outside our storage | download and store |

Rules:
- More than 10 items → 400.
- A duplicate item → 400.
- A URL into our storage that is not one of the article's current images → 400.
- Any other string → 400.
- A missing `images` field (nil) keeps the current images; an explicit list, including the
  empty list `[]`, replaces them. The editor always sends the full list.
- With an explicit list, update is full-replace: the request list becomes the new list. Images that drop out are
  deleted from R2 after a successful save. Deleting the article deletes all its owned images.
- All checks and downloads happen before any DB write. On create the row is inserted first
  (object keys need the final slug), then the objects are stored and the row is updated; a
  failure removes the row again. On a failure after that, every
  object stored in this call is removed and the previous images stay untouched (same
  ordering as cover and video).

## HTTP API

- `NewsResponse` (admin) and `PublicNewsDetailResponse` gain `images: string[]` (always an
  array, empty when none). List/feed responses are unchanged.
- `POST /api/admin/news/image-uploads` uses the default 1 MB body limit and the admin
  middlewares, the same as `video-uploads`.

## Frontend

- Editor block «Фото»:
  - multi-file picker (`accept` = image types) and a link input with an «Додати» button;
  - thumbnails in order, with upload progress, ↑/↓ reorder and remove buttons;
  - a counter «N / 10»; adding more than 10 is blocked client-side;
  - each file is checked client-side for type and 10 MB size;
  - saving is disabled while any upload is in progress.
- The form sends `images` as the ordered list of pending keys, current URLs and new links.
- Article page placement (pure function in `src/lib/video-placement.ts`, renamed to
  `src/lib/media-placement.ts`):
  - n paragraphs (non-empty lines), k images;
  - image i (1-based) goes after paragraph `round(i * n / (k + 1))`, clamped to `1..n`;
  - images that land on the same gap stay together in their order;
  - the video keeps its middle position (after paragraph `ceil(n / 2)`); at a shared gap
    the video comes first;
  - with no text, the video and then the images are shown.
- Images render as `<img loading="lazy">` with the article title as `alt`. A click opens a
  simple full-screen viewer (close by button, backdrop click or Escape).

## Operations (README)

- R2 lifecycle: replace the `videos/pending/` rule with one rule on `pending/`.
- R2 CORS: the existing `PUT` rule covers image uploads too.

## Testing

- Service:
  - presign validation for images;
  - each item kind (pending, keep, external);
  - rejected cases: format, foreign storage URL, duplicates, more than 10;
  - inspect failures and immediate delete of rejected pending objects;
  - ordering: copy → save → delete obsolete and pending;
  - cleanup on failures and on article delete;
  - video pending prefix moved to `pending/videos/`.
- Repository integration: `image_urls` round-trip, the max-10 check.
- Handler: `image-uploads` endpoint; `images` in JSON and multipart requests; `images` in
  admin and public detail responses, absent from list items.
- Frontend:
  - placement function (images only, with video, more images than gaps, empty text);
  - API helpers;
  - editor states: thumbnails, progress, limit, reorder, remove, link add;
  - article rendering and the viewer markup.
