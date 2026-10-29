# Featured News and Hero Height Design

## Intent

The home page has one clearly promoted published article, always in the large top-left grid position, and a taller hero section matching the article-cover proportions.

## Approved behavior

- At most one **published** article may have `featured = true` at any time.
- Creating a published featured article or updating an article so it becomes published and featured automatically clears `featured` on the previously published featured article.
- Drafts do not participate in the uniqueness restriction. Publishing a featured draft replaces the current published featured article.
- Unpublishing or editing without featured status does not promote a replacement automatically.
- Transfer of featured status and the requested create/update must be atomic. A partial unique index in PostgreSQL enforces the invariant at the persistence boundary.
- Existing duplicate published-featured rows are deduplicated during migration; the most recently published row remains featured, with deterministic tie-breaking by creation time and ID.
- The public repository orders published results by `featured DESC` and then publication date, so the featured article is included at the start of the first page even when it is older than the 20 newest publications. Other articles keep their relative publication-date order.
- The public home-page article grid places the featured article first, in the existing large `lg:col-span-2 lg:row-span-2` position at the top left. Latest-news and ticker ordering do not change: their mapping explicitly restores publication-date order from the response.
- `Hero.tsx` uses minimum responsive heights `240px`, `360px` at `sm`, and `480px` at `lg`, matching the article cover. Existing content and gradients remain, and content may expand the section instead of being clipped.

## Implementation boundaries

- Preserve the existing service and HTTP API contracts; no admin workflow changes or new frontend dependencies.
- Implement atomic featured transfer inside existing repository create/update operations, with the service continuing to own the publication/featured business inputs.
- Add a backward and forward Goose migration; do not modify the original migration.
- Reuse existing `PublicNews`/`Article` mapping and card markup.
- Preserve unrelated dirty-tree changes and previous feature work.
- No commits or pushes unless explicitly requested.

## Verification

- Service tests cover create/update replacement and the draft distinction using the existing in-memory repository.
- PostgreSQL integration tests cover atomic transfer on create/update and the single-published-featured constraint; they may skip when `DATABASE_URL` is unset.
- Frontend tests prove stable featured-first grid data, the preserved two-column/two-row card classes, and Hero's exact responsive minimum-height classes.
- Backend `go test ./...`, `go vet ./...`, frontend `npm run lint`, `npm run build`, and both Node test suites pass.
