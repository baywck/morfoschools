"use client";

/**
 * ExportBlueprintButton — small icon-only button next to the
 * LoadKisiKisiButton. Visible only when the exam currently has a
 * blueprint to export. Click opens ExportBlueprintSheet.
 */

import { ClipboardCopy } from "lucide-react";
import { cn } from "@/lib/cn";

export interface ExportBlueprintButtonProps {
  visible: boolean;
  onClick: () => void;
  disabled?: boolean;
}

export function ExportBlueprintButton({
  visible,
  onClick,
  disabled,
}: ExportBlueprintButtonProps) {
  if (!visible) return null;
  const tooltip = "Export kisi-kisi sebagai template";
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
      <ClipboardCopy size={14} />
    </button>
  );
}
