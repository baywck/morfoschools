"use client";

import { useState, useRef, useEffect } from "react";
import { createPortal } from "react-dom";
import { MoreVertical } from "lucide-react";
import { cn } from "@/lib/cn";

interface RowAction {
  label: string;
  icon?: React.ReactNode;
  onClick: () => void;
  variant?: "default" | "danger";
}

interface RowActionsProps {
  actions: RowAction[];
}

export function RowActions({ actions }: RowActionsProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (btnRef.current && btnRef.current.contains(e.target as Node)) return;
      setOpen(false);
    }
    function handleScroll() { setOpen(false); }
    document.addEventListener("mousedown", handleClick);
    document.addEventListener("scroll", handleScroll, true);
    return () => {
      document.removeEventListener("mousedown", handleClick);
      document.removeEventListener("scroll", handleScroll, true);
    };
  }, [open]);

  function toggle() {
    if (!open && btnRef.current) {
      const rect = btnRef.current.getBoundingClientRect();
      setPos({ top: rect.bottom + 4, left: rect.right - 160 });
    }
    setOpen((v) => !v);
  }

  return (
    <>
      <button
        ref={btnRef}
        onClick={toggle}
        className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)] transition-colors"
      >
        <MoreVertical size={15} />
      </button>

      {open && pos && typeof document !== "undefined" && createPortal(
        <div
          style={{ position: "fixed", top: pos.top, left: pos.left, zIndex: 9999 }}
          className="w-40 rounded-xl border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg"
        >
          {actions.map((action, i) => (
            <button
              key={i}
              onClick={() => { action.onClick(); setOpen(false); }}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[11px] font-medium transition-colors",
                action.variant === "danger"
                  ? "text-[var(--danger)] hover:bg-[var(--danger-soft)]"
                  : "text-[var(--foreground)] hover:bg-[var(--muted)]"
              )}
            >
              {action.icon}
              {action.label}
            </button>
          ))}
        </div>,
        document.body
      )}
    </>
  );
}
