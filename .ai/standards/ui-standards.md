# UI Standards — Morfoschools

## Design System

Morfosis Design System. OKLCH color tokens. Premium school SaaS aesthetic.

## Non-Negotiables

1. **Floating labels** — NEVER placeholder-as-label
2. **Sidebar** — dark icon strip (66px), never white panel with text
3. **OKLCH tokens** — not Tailwind defaults
4. **Every button** — idle, hover, focus-visible, loading, disabled states
5. **Mutating actions** — loading feedback, prevent double-submit, toast on success/error
6. **Destructive actions** — custom ConfirmDialog, never native confirm/alert
7. **Form validation** — inline field errors, never browser native validation
8. **Dark/light mode** — no layout/color flicker on toggle
9. **Tenant palette** — CSS variables, resolved before paint

## Component Patterns

| Component | Pattern |
|-----------|---------|
| Page layout | `page.tsx` (wiring) + `*-page.tsx` (UI) |
| Forms | FormDrawer (pulled-right) |
| Destructive confirm | ConfirmDialog (centered) |
| Data pages | DirectoryToolbar + Table (desktop) + Cards (mobile) |
| Loading | Skeleton that mirrors real content structure |
| Empty state | Illustration + message + primary action |
| Error state | Alert + retry action |

## Data UX Rules

- No dummy/mock initial rows
- Backend-wired pages must show skeleton → real data
- Empty state when no records
- Error state with retry on API failure
- Consistent table/list controls
- Equal-height adjacent controls

## Interaction Contract

- All buttons: idle → hover → focus → loading → disabled
- Mutating: loading feedback + success/error toast
- Destructive: ConfirmDialog before action
- Forms: react-hook-form + Zod, inline field errors
- Adjacent controls: equal height, consistent spacing
