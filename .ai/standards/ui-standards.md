# UI Standards — Morfoschools

## Design System

**Morfosis Studio** — Source of truth for all UI decisions.

**Reference:** `/home/bayw/Documents/Morfosis/morfosis-studio`

## Key Design Language (from morfosis-studio)

### Shell Architecture
- **Dark shell** (`--shell: #111111`) wraps the entire app
- **Sidebar** = 66px dark icon strip (NOT dual-panel, NOT 330px)
- **Topbar** = inside dark shell area (breadcrumb + search + theme toggle)
- **Content** = floating white card (`rounded-2xl`, `shadow-[0_20px_60px_rgba(0,0,0,0.12)]`)
- **Page header** = sticky inside content card (not in topbar)
- **Content max-width** = `max-w-5xl` centered

### Sidebar
- Width: `66px` (`--sidebar-width: 66px`)
- Items: `42x42px`, `rounded-[11px]`
- Active: `bg-white/10`, `shadow-[inset_0_0_0_1px_rgba(255,255,255,0.08)]`
- Inactive: `text-[var(--shell-muted)]`, hover `bg-white/[0.06]`
- Tooltip on hover (appears to the right)

### TextField
- Height: `62px` (`min-h-[62px]`)
- Border: `border-2 rounded-2xl`
- Focus: `border-[var(--field-focus)] shadow-[0_0_0_3px_var(--field-ring)]`
- Prefix/suffix: in `rounded-xl` pill containers (`w-10`, `border`, `bg-muted`)
- Label: floating (animates from center to top-2.5 on focus/fill)
- Font: `text-sm font-semibold` for value, `text-[11px] font-semibold` for floated label

### Button
- Primary = **BLACK** (`--primary: #111827`)
- Sizes: sm(`h-8`), md(`h-9`), lg(`h-11`)
- Border radius: sm/md `rounded-lg`, lg `rounded-xl`
- Active: `active:scale-[0.97]`
- Font: `font-semibold`

### Colors
- Primary = black (not blue)
- Brand = blue (`#3b82f6`) — used for field focus, links, accents
- All via CSS custom properties
- Dark mode via `[data-theme="dark"]`

### Typography
- Font: Inter via `next/font/google`
- Base: 14px
- Labels: `text-[11px] font-semibold uppercase tracking-wider`
- Body: `text-[12px]` or `text-[13px]`

## Forbidden

- ❌ Dual-panel sidebar (330px with menu panel)
- ❌ OKLCH colors
- ❌ Placeholder text in inputs (label IS placeholder)
- ❌ Blue primary button (primary = black, brand = blue)
- ❌ Content without floating card wrapper
- ❌ Sidebar wider than 66px
- ❌ System font without Inter
- ❌ Hardcoded colors (use CSS vars)
