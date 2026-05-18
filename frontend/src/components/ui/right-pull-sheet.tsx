"use client";

import { X } from "lucide-react";

interface RightPullSheetProps {
  open: boolean;
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}

export function RightPullSheet({ open, title, onClose, children }: RightPullSheetProps) {
  if (!open) return null;

  return (
    <div className="absolute right-0 top-0 z-40 h-full w-full sm:max-w-md border-l border-[var(--border)] bg-[var(--card)] shadow-xl flex flex-col rounded-r-[inherit]">
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
      <div className="flex-1 overflow-y-auto p-5">
        {children}
      </div>
    </div>
  );
}
