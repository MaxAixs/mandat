# Main Event Hero Design

## Intent

Let an editor choose one published news article as the main event of the day and use that article in the existing home-page Hero.

## Approved behavior

- The news editor has a separate switch labeled «Головна подія дня». The existing «На головній» switch keeps its current meaning.
- Exactly one published article may be the main event. Selecting another published article transfers the flag; a flagged draft does not displace the published main event.
- The Hero takes its heading, short description, and background image from the selected article.
- The heading links to `/news/:slug` and receives a subtle hover accent. There is no additional «Головна подія дня» label.
- The audience block «42 728 підписників» sits at the bottom-right on desktop and remains responsive on mobile.
- With no selected article, the current static heading, description, and background remain. A selected article without a cover uses the current background.

## Implementation constraints

- Add a `main_event` boolean through the admin/public API and PostgreSQL; do not repurpose `featured`.
- Keep the selected article on the first public page so the home loader can build the Hero without another request.
- Do not change unrelated public/admin behavior or add dependencies.
- Do not commit or push unless explicitly commanded.
