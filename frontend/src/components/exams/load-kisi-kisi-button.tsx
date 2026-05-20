"use client";

/**
 * LoadKisiKisiButton (Phase 9.8 inline rewrite) — small icon-only
 * button that sits next to the KisiKisiToggle in the exam header.
 * Tooltip explains the action; visible only when usesKisiKisi=true.
 * Click opens the LoadKisiKisiSheet modal.
 */

import { ClipboardPaste } from "lucide-react";
import { cn } from "@/lib/cn";

export interface LoadKisiKisiButtonProps {
  visible: boolean;
  hasBlueprint: boolean;
  onClick: () => void;
  disabled?: boolean;
}

export function LoadKisiKisiButton({
  visible,
  hasBlueprint,
  onClick,
  disabled,
}: LoadKisiKisiButtonProps) {
  if (!visible) return null;
  const tooltip = hasBlueprint ? "Ganti template kisi-kisi" : "Load template kisi-kisi";
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={tooltip}
      aria-label={tooltip}
      className={cn(
        "inline-flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] transition-all",
        "hover:bg-[var(--muted)] hover:text-[var(--foreground)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand)]/40 active:scale-[0.96]",
        disabled && "opacity-60 cursor-not-allowed",
      )}
    >
      <ClipboardPaste size={14} />
    </button>
  );
}
