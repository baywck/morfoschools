"use client";

import { X } from "lucide-react";
import { cn } from "@/lib/cn";

interface RightPullSheetProps {
  open: boolean;
  title: string;
  onClose: () => void;
  children: React.ReactNode;
  /**
   * Tailwind width classes for the sheet panel. Defaults to a single-column
   * form size. Use `wide` or `xwide` for richer authoring panels (e.g.
   * exam question editor with options + scoring grid).
   *
   * Pass `"sm"` (default), `"md"`, `"lg"`, `"xl"`, or any custom Tailwind
   * width string for full control.
   */
  width?: "sm" | "md" | "lg" | "xl" | string;
  /**
   * Optional footer rendered at the bottom of the sheet outside the
   * scrolling content area. Use for sticky save/cancel buttons.
   */
  footer?: React.ReactNode;
}

const widthMap: Record<string, string> = {
  sm: "w-full sm:max-w-md",
  md: "w-full sm:max-w-lg",
  lg: "w-full sm:max-w-2xl",
  xl: "w-full sm:max-w-3xl",
};

export function RightPullSheet({
  open,
  title,
  onClose,
  children,
  width = "sm",
  footer,
}: RightPullSheetProps) {
  if (!open) return null;

  // Resolve width: known key → mapped classes; otherwise treat as a custom
  // Tailwind class string the caller provided.
  const widthClasses = widthMap[width] ?? width;

  return (
    <div
      className={cn(
        "absolute right-0 top-0 z-40 h-full border-l border-[var(--border)] bg-[var(--card)] shadow-xl flex flex-col rounded-r-[inherit]",
        widthClasses
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
        <h3 className="text-[14px] font-semibold text-[var(--foreground)]">{title}</h3>
        <button
          onClick={onClose}
          aria-label="Close"
          className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)] transition-colors"
        >
          <X size={16} />
        </button>
      </div>
      {/* Content */}
      <div className="flex-1 overflow-y-auto p-5">{children}</div>
      {/* Optional sticky footer */}
      {footer && (
        <div className="border-t border-[var(--border)] px-5 py-3">{footer}</div>
      )}
    </div>
  );
}
