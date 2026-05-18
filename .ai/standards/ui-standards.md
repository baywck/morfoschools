# UI Standards — Morfoschools

## Design System

**Morfosis Design System** — Based on Metronic Layout 16.

**Mandatory reference before ANY UI work:**
- Skill: `/home/bayw/.pi/agent/skills/morfosis-ui/SKILL.md`
- Design Spec: `/home/bayw/.pi/agent/skills/morfosis-ui/DESIGN_SPEC.md`
- Component Reference: `/home/bayw/.pi/agent/skills/morfosis-ui/COMPONENT_REFERENCE.md`

## Non-Negotiables

1. **Primary button = BLACK** (#111827), not blue, not brand color
2. **ALL buttons must have icons** — icon before text, h-3.5 w-3.5
3. **Sidebar = dual-panel** — black icon strip (70px, rounded-2xl) + optional menu panel
4. **Header = 60px** — search + action buttons + divider + user dropdown
5. **Font = Inter** via `next/font/google`, variable `--font-inter`
6. **CSS vars = hex-based** — not OKLCH, not Tailwind defaults
7. **Floating labels** — NO placeholder prop, label IS the placeholder
8. **Input icon** — rendered in `h-7 w-7 rounded-md bg-muted border` box
9. **Adjacent elements = same height** (h-8 for buttons/search, h-11 for inputs)
10. **Mobile bottom nav** — first 5 items, h-14, replaces sidebar on mobile
11. **Dark mode** via `[data-theme="dark"]` on root element
12. **Content area** — `p-5` padding, `space-y-5` between sections
13. **Cards** — `rounded-lg` (never rounded-2xl/3xl), max `p-5`, `shadow-sm` max
14. **DataTable** — must have loading (skeleton) + empty (EmptyState) states
15. **User dropdown** — avatar + name + subinfo + chevron, with profile/settings/logout

## Forbidden

- ❌ Blue/colored primary button
- ❌ Buttons without icons
- ❌ Placeholder text in inputs
- ❌ Single-panel sidebar (must be dual: icon strip + menu panel)
- ❌ Mismatched heights on adjacent elements without divider
- ❌ System font without Inter
- ❌ Hardcoded colors (use CSS vars)
- ❌ Missing loading/empty states
- ❌ rounded-2xl/3xl on cards
- ❌ Heavy shadows
- ❌ PageHeader as a card
- ❌ Modals without header/body/footer structure

## Component Patterns

| Component | Key Rule |
|-----------|----------|
| Button | BLACK primary, always has icon, h-8 (sm/md), h-9 (lg) |
| InputField | FieldShell wrapper, floating label, icon in box, h-11 |
| Toast | Left colored border-l-4, rounded-lg |
| Skeleton | `animate-pulse rounded-lg bg-muted` |
| Badge | `rounded-md px-2 py-0.5 text-xs`, soft bg tones |
| DataTable | rounded-lg border bg-card, skeleton rows for loading |
| PageHeader | NOT a card, just flex row, no border/bg |
| FormSection | rounded-lg border bg-card p-5 |
| Modal | header (border-b) / body / footer (border-t) |
| EmptyState | border-dashed, bg-accent, centered |

## Layout

| Element | Height/Size |
|---------|-------------|
| Header | 60px (var(--header-height)) |
| Sidebar icon strip | 70px wide |
| Sidebar full | 330px (var(--sidebar-width)) |
| Buttons sm/md/icon | h-8 |
| Buttons lg | h-9 |
| Input fields | h-11 |
| Nav items (panel) | h-[34px] |
| Icon strip nav | h-9 w-9 |
| Mobile bottom nav | h-14 |

## File Structure

```
components/
├── ui/                  # Atomic components
│   ├── button.tsx
│   ├── field-shell.tsx
│   ├── input-field.tsx
│   ├── select-field.tsx
│   ├── textarea-field.tsx
│   ├── badge.tsx
│   ├── data-table.tsx
│   ├── metric-card.tsx
│   ├── page-header.tsx
│   ├── form-section.tsx
│   ├── modal.tsx
│   ├── confirm-dialog.tsx
│   ├── right-pull-sheet.tsx
│   ├── tabs.tsx
│   ├── toast.tsx
│   ├── skeleton.tsx
│   └── empty-state.tsx
├── app-shell/           # Layout components
│   ├── app-shell.tsx
│   ├── sidebar.tsx
│   ├── header.tsx
│   └── mobile-nav.tsx
└── auth-guard.tsx
```
