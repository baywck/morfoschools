"use client";

/**
 * MobileActionsMenu — three-dot dropdown for header action clusters
 * that overflow on narrow viewports. Desktop renders the children
 * inline as before; mobile collapses everything into a single icon
 * button that opens a panel.
 *
 * Usage:
 *   <MobileActionsMenu>
 *     <button>Action A</button>
 *     <button>Action B</button>
 *   </MobileActionsMenu>
 *
 * The component does NOT alter desktop behaviour — children render
 * straight-through above the `md` breakpoint.
 */

import { useEffect, useRef, useState } from "react";
import { MoreVertical } from "lucide-react";
import { cn } from "@/lib/cn";

export interface MobileActionsMenuProps {
  /** Buttons to render inline on desktop, stacked in the dropdown on mobile. */
  children: React.ReactNode;
  /** Tailwind className for the desktop wrapper (md+). */
  desktopClassName?: string;
}

export function MobileActionsMenu({
  children,
  desktopClassName = "hidden md:flex items-center gap-2",
}: MobileActionsMenuProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("mousedown", handleClick);
      document.removeEventListener("keydown", handleKey);
    };
  }, [open]);

  return (
    <>
      {/* Desktop: render inline */}
      <div className={desktopClassName}>{children}</div>

      {/* Mobile: 3-dot trigger + dropdown */}
      <div className="relative md:hidden" ref={ref}>
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-haspopup="menu"
          aria-expanded={open}
          aria-label="More actions"
          className={cn(
            "inline-flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] transition-all",
            "hover:bg-[var(--muted)] hover:text-[var(--foreground)] active:scale-[0.96]",
            open && "bg-[var(--muted)] text-[var(--foreground)]",
          )}
        >
          <MoreVertical size={14} />
        </button>
        {open && (
          <div
            role="menu"
            className="absolute right-0 top-full z-30 mt-1 flex min-w-[180px] flex-col gap-1 rounded-lg border border-[var(--border)] bg-[var(--card)] p-1.5 shadow-lg"
            onClick={() => setOpen(false)}
          >
            {children}
          </div>
        )}
      </div>
    </>
  );
}
