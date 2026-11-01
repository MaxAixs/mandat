# Category Pages Design

## Intent

Let visitors open dedicated Sport and International News pages from the site header and see only published articles from the selected rubric.

## Approved behavior

- «Спорт» links to `/sport`; «Міжнародні новини» links to `/international`.
- Each page keeps the existing public Header and Footer and displays the selected rubric as its heading.
- The page loads only published articles whose category exactly matches the selected rubric.
- Filtering happens in PostgreSQL through `GET /api/news?category=<rubric>`, not by filtering the first general-feed page in the browser.
- The page supports loading, retryable errors, an empty-rubric message, and pagination using the existing backend page/limit contract.
- Article cards still link to `/news/:slug`.
- The home page, Hero, admin panel, other header links, and requests without `category` retain their current behavior.

## Implementation constraints

- Accept only the existing `models.Category` values; an unknown category returns `400 Invalid input`.
- `category` is optional. Omitting it preserves the current public ordering and response shape.
- Use the existing `Article`, `NewsCard`, and public-news API mapping rather than adding a parallel card model.
- Do not add dependencies or commit/push without an explicit command.
