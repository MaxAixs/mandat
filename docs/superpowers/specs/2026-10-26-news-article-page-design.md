# News Article Page Design

## Intent

A reader can open a published article from the home page and read its full text without losing the site's familiar navigation.

## Approved behavior

- Every news card on the home page links to `/news/:slug` using the article's existing slug.
- The article page keeps the existing main `Header` unchanged in appearance and content, including its logo, navigation, and search. The existing `Footer` also remains.
- Below the header, the cover image spans the full width of the article content area, matching the home page's visual language.
- Below the image are the article title, publication date, and view count, followed by the full article text.
- Newline-separated paragraphs in the stored plain text remain visually separated. Text is rendered as text, never as HTML.
- Opening the page directly by URL loads the article from the backend; it does not depend on having visited the home page first.
- Loading, request failure with retry, and missing-article states are visible to the reader.
- Only published articles are publicly accessible. Drafts and unknown slugs produce the same not-found response.
- The logo links to the home page so it works from article URLs as expected.
- Existing admin routes and public article-card responses remain compatible.

## Implementation boundaries

- Add `GET /api/news/:slug`, returning public article fields plus `text`; do not expose administrative-only fields.
- Reuse the existing repository's slug lookup and enforce publication status in the service layer.
- Do not increment views as part of this feature; display the stored value.
- Reuse React Router, Tailwind, lucide-react, and the existing API/request conventions. Add no dependencies.
- Preserve all unrelated user changes already present in the working tree.
- No commits or pushes unless explicitly requested.

## Verification

- Backend tests cover published detail, draft/missing detail, public fields, and repository failure handling.
- Frontend API tests cover URL slug encoding, response mapping, paragraphs' source text, and request errors.
- `go test ./...`, `go vet ./...`, `npm run lint`, and `npm run build` pass in their respective directories.
