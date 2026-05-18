# UI Standards — Morfoschools

## Design System

**Morfosis Studio** — Source of truth for all UI decisions.

**Reference:** `/home/bayw/Documents/Morfosis/morfosis-studio`

## Key Design Language (from morfosis-studio)

### Shell Architecture
- **Dark shell** (`--shell: #111111`) wraps the entire app
- **Sidebar** = 66px dark icon strip (NOT dual-panel, NOT 330px)
- **Topbar** = inside dark shell area (breadcrumb + theme toggle + user button + AI chat toggle)
- **Content** = floating white card (`rounded-2xl` mobile, `rounded-xl` desktop, `shadow-[0_20px_60px_rgba(0,0,0,0.12)]`)
- **Page header** = sticky inside content card via PageShell component
- **Content max-width** = `max-w-5xl` centered
- **AI Chat** = fixed right panel (360px), pushes content left when open

### Sidebar
- Width: `66px` (`--sidebar-width: 66px`)
- Container: `fixed inset-y-0 left-0 z-40 hidden md:flex w-[var(--sidebar-width)] flex-col items-center bg-[var(--shell)] py-4`
- NO horizontal padding — icons center via `items-center` + `w-full`
- Brand: wrapped in `flex w-full justify-center mb-6`
- Nav items: `h-[42px] w-[42px] rounded-[11px]`
- Active: `bg-[var(--shell-active)] shadow-[inset_0_0_0_1px_rgba(255,255,255,0.08)]`
- Inactive: `text-[var(--shell-muted)]`, hover `bg-[var(--shell-hover)]`
- Icons: `h-[18px] w-[18px]`, active `stroke-[2.25]`, inactive `stroke-[2]`
- Tooltip on hover

### Topbar
- Padding: `px-4`
- Height: `var(--header-height)` = 60px
- Background: transparent (inherits dark shell)
- Left: Breadcrumb (Home icon → page segments)
- Right: AI chat toggle + theme toggle (desktop only, no border/box) + user button
- User button: avatar (h-7 w-7) + name + role sub-info (desktop), avatar only (mobile)
- Theme toggle in user dropdown on mobile
- User dropdown: info + dark/light toggle (mobile) + logout

### PageShell (sticky header inside content card)
- Sticky `top-0 z-20` with backdrop blur
- Height: `h-14`
- Desktop: title + subtitle left, search (h-8 inline) + action button right
- Mobile: title left + `+` icon button right, search in separate row below
- Action button: `h-8 rounded-lg bg-[var(--primary)]` (desktop shows label, mobile shows + icon only)

### Mobile
- Bottom nav: `h-16`, horizontal scroll, in dark shell below content card
- Content card: `rounded-2xl`, `px-2 pb-[4.5rem]` spacing
- RightPullSheet: full width on mobile
- Row actions: 3-dot dropdown (portal, fixed positioning)

## Components

### InputField (floating label)
- Height: `h-11`
- Border: single `border rounded-lg`
- Focus: `border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]`
- Prefix icon: `h-7 w-7 rounded-md border bg-[var(--muted)]`
- Label: floats from center to `top-1 text-[10px]` on focus/value
- NO native validation (no `required`, no `type="email"`, no `type="number"`)
- Keep `type="password"` for masking only

### SelectField (floating label dropdown)
- Same h-11, rounded-lg, floating label pattern as InputField
- Dropdown: `rounded-lg border bg-[var(--card)] p-1 shadow-lg`
- Supports `disabled` prop
- Label floats based on `hasSelection` (not value truthiness)

### DatePicker (custom calendar)
- Same h-11 trigger as InputField
- Calendar dropdown: `w-72 rounded-xl border bg-[var(--card)] p-3 shadow-lg`
- Day grid: `h-8 w-8 rounded-lg`
- Selected: `bg-[var(--primary)] text-[var(--primary-foreground)]`
- Today: `bg-[var(--brand-soft)] text-[var(--brand)]`
- "Today" quick button at bottom

### DateRangePicker
- Same trigger as DatePicker
- Start/End toggle buttons
- Range highlight: `bg-[var(--brand-soft)]`

### Button
- Primary = BLACK (`--primary: #111827`)
- Sizes: sm(`h-8`), md(`h-9`), lg(`h-11`)
- Border radius: sm/md `rounded-lg`, lg `rounded-xl`
- `active:scale-[0.97]`, `font-semibold`
- Variants: primary, secondary, outline, ghost, danger
- Loading: spinner `h-3.5 w-3.5 animate-spin`

### RightPullSheet
- NO backdrop overlay — user can interact with rest of app
- `absolute right-0 top-0 z-40 h-full w-full sm:max-w-md`
- `rounded-r-[inherit]` to match parent card corners
- Close only via X button or Cancel
- Header: title + X button, `border-b`

### ConfirmDialog
- `absolute inset-0 z-50` centered overlay inside content card
- `max-w-sm rounded-xl border bg-[var(--card)] p-5 shadow-xl`
- Destructive variant: danger icon + danger button

### RowActions (3-dot dropdown)
- Uses `createPortal` to render on `document.body`
- `position: fixed` calculated from button rect
- Closes on outside click (via `click` event) and scroll
- Action fires via `requestAnimationFrame` after close

### Toast
- Left colored `border-l-4` by tone
- `rounded-xl border bg-[var(--card)] p-4 shadow-sm`
- Fixed `bottom-4 right-4 z-50`

### SearchInput
- Compact `h-8 rounded-lg border bg-[var(--background)]`
- Plain input (NOT floating label)
- Search icon `h-3.5 w-3.5` left

### Breadcrumb
- Component-based: BreadcrumbList, BreadcrumbItem, BreadcrumbLink, BreadcrumbPage, BreadcrumbSeparator
- Home icon as first item
- ChevronRight separator
- Current page: `font-semibold text-[var(--shell-foreground)]`

### Skeleton
- `animate-pulse rounded-lg bg-[var(--muted)]`

### AI Chat Panel
- Fixed right `w-[360px]`, pushes content via `md:pr-[360px]` on AppShell
- Dark themed (same as shell)
- Model selector dropdown
- Attach menu (plus button)
- Auto-resize textarea (max 120px)
- Suggestions shown when no conversation
- Messages: brand color for user, subtle bg for assistant

## Golden Rules (Non-Negotiable)

1. **NO native form validation** — no `required`, no `type="email"`, no `type="number"`. All validation server-side via structured field errors.
2. **Loading state on EVERY action** — create, edit, archive, assign, unassign, switch. Button disabled + spinner.
3. **No placeholder-as-label** — floating label IS the placeholder
4. **System font stack** — `ui-sans-serif, system-ui, sans-serif` + emoji fonts. No Google Fonts.
5. **All colors via CSS vars** — never hardcode hex in components
6. **Dark mode** via `[data-theme="dark"]` on root element
7. **Confirmation for destructive actions** — ConfirmDialog before archive/delete
8. **Toast feedback** on every mutation (success/error)
9. **Empty state** for lists with no data
10. **Skeleton loading** while data fetches

## Forbidden

- ❌ Native browser validation (required, type=email popups)
- ❌ Buttons without loading state on async actions
- ❌ Dual-panel sidebar
- ❌ Inter or any Google Font
- ❌ `px-*` on sidebar container
- ❌ `padding-left` on content card wrapper
- ❌ `rounded-2xl` on content card desktop (use `rounded-xl`)
- ❌ Placeholder text in inputs
- ❌ Blue primary button (primary = black, brand = blue)
- ❌ Hardcoded colors
- ❌ Missing loading/empty states
- ❌ OKLCH color values
- ❌ Backdrop overlay on RightPullSheet
- ❌ `overflow-hidden` on content card (breaks portaled dropdowns)
- ❌ Border/box on theme toggle icon

## File Structure

```
components/
├── ui/
│   ├── button.tsx
│   ├── input-field.tsx        # Floating label, h-11
│   ├── select-field.tsx       # Floating label dropdown
│   ├── date-picker.tsx        # Custom calendar
│   ├── date-range-picker.tsx  # Start/end range
│   ├── search-input.tsx       # Compact h-8 plain input
│   ├── breadcrumb.tsx         # Component-based breadcrumb
│   ├── right-pull-sheet.tsx   # No overlay, rounded-r-[inherit]
│   ├── confirm-dialog.tsx     # Centered, destructive variant
│   ├── row-actions.tsx        # Portal dropdown, 3-dot
│   ├── toast.tsx              # border-l-4, fixed bottom-right
│   └── skeleton.tsx
├── layout/
│   ├── app-shell.tsx          # Dark shell + floating card + AI chat
│   ├── sidebar.tsx            # 66px icon strip
│   ├── topbar.tsx             # Breadcrumb + user + AI toggle
│   ├── mobile-nav.tsx         # Bottom h-16, horizontal scroll
│   ├── page-shell.tsx         # Sticky header inside card
│   └── ai-chat-panel.tsx      # 360px right panel, model selector
└── [feature pages use PageShell + components above]
```
