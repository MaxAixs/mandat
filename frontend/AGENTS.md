# AGENTS.md — Frontend

The public part of the news website: news feed, digests, analytics. An SPA built with React.

## Stack

- **React 19** + **TypeScript** (strict typing, `tsc -b` before building)
- **Vite 7** — dev server and build
- **Tailwind CSS 3.4** — styling, theming via CSS variables
- **shadcn/ui** (`new-york` style, baseColor `slate`) — UI components in `src/components/ui/`
- **react-router v7** — routing
- **lucide-react** — icons
- ESLint (flat config: `typescript-eslint` + `react-hooks` + `react-refresh`)

## Commands

```bash
npm run dev       # dev server with HMR
npm run build     # tsc -b && vite build (type checking + production build)
npm run lint      # ESLint across the whole project
npm run preview   # preview of the production build
```

Before finishing any task: `npm run lint` and `npm run build` must pass without errors.

## Structure

```
src/
  pages/       Pages (routes). Currently: Home.tsx
  sections/    Landing/page sections: Header, Hero, News, Digest, Telegram, Analytics, Insight, Ticker, Footer
  components/    Shared components (NewsCard.tsx)
  components/ui/  shadcn/ui components (40+ of them) — do NOT edit manually
  hooks/       Custom hooks (use-mobile.ts)
  lib/         Utilities: utils.ts (cn() for merging classes)
  types/       Shared TypeScript types (domain model: Article, StatItem, NavLink)
  constants/   Site constants: NAV_LINKS, FOOTER_LINKS, SITE_NAME
  api/         Backend client; mock.ts holds mock content until the Go API is ready
  assets/      Static assets imported from code (images, fonts)
  App.tsx      Root component (router)
  main.tsx     Entry point
  index.css    Global styles + CSS variables of the Tailwind/shadcn theme
```

`main.tsx`, `index.css`, `App.tsx` stay in the root of `src/` — this is required by Vite and shadcn (`components.json` references `src/index.css`).

`_archive/` in the root of `frontend/` is scaffolding junk (old archives, mockup, setup scripts). It is not committed to git (listed in `.gitignore`) and is not used in the code.

## Code Conventions

- **TypeScript only**: new files are `.tsx`/`.ts`, avoid `any` unless absolutely necessary.
- **Function components** with hooks; do not use class components.
- **Imports via the `@/` alias**: `@/components/ui/button`, `@/lib/utils`, `@/hooks/use-mobile`.
- **Components**: `PascalCase.tsx`, one component per file. Hooks: `use-*.ts` in `src/hooks/`.
- **Styling — Tailwind classes only**. Custom CSS is a last resort (in `App.css` or `index.css`).
- Merge classes using `cn()` from `@/lib/utils`.
- **Theme colors — via the `mandat-*` Tailwind classes** (`bg-mandat-surface`, `text-mandat-muted`, `border-mandat-line`, `bg-mandat-red`, `text-mandat-red-bright`, ...). They map to CSS variables in `src/index.css` (see `tailwind.config.js`). Never hardcode hex colors in components.
- **Content does not live in components.** Mock data comes from `@/api/mock`, navigation/links from `@/constants`, types from `@/types`. When the backend lands, only the API layer changes.

## Working with shadcn/ui

- Ready-made components are already in `src/components/ui/` — look there first.
- If the one you need is missing, add it via the CLI: `npx shadcn@latest add <component>`.
- Do not edit files in `components/ui/` manually — customize via props and wrappers in your own components.

## Common Tasks

- **New page**: create `src/pages/MyPage.tsx`, register it in the router (`App.tsx`).
- **New section**: `src/sections/MySection.tsx`, import it on the relevant page.
- **Icons**: `import { IconName } from 'lucide-react'`.

## What Not to Do

- Do not add new dependencies without approval (first check whether something suitable already exists in shadcn/radix/react-hook-form).
- Do not edit `components.json`, `tailwind.config.js`, `vite.config.ts` without necessity.
- Do not commit `dist/`, `node_modules/`, `_archive/` (scaffolding junk — already in `.gitignore`).
