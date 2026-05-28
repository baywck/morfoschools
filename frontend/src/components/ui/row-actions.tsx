"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { MoreVertical } from "lucide-react";
import { cn } from "@/lib/cn";

interface RowAction {
  label: string;
  icon?: React.ReactNode;
  onClick: () => void;
  variant?: "default" | "danger";
  disabled?: boolean;
}

interface RowActionsProps {
  actions: RowAction[];
}

export function RowActions({ actions }: RowActionsProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const close = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (btnRef.current?.contains(e.target as Node)) return;
      if (menuRef.current?.contains(e.target as Node)) return;
      close();
    }
    function handleScroll() { close(); }
    // Use click (not mousedown) so menu item onClick fires first
    document.addEventListener("click", handleClick);
    document.addEventListener("scroll", handleScroll, true);
    return () => {
      document.removeEventListener("click", handleClick);
      document.removeEventListener("scroll", handleScroll, true);
    };
  }, [open, close]);

  function toggle(e: React.MouseEvent) {
    e.stopPropagation();
    if (!open && btnRef.current) {
      const rect = btnRef.current.getBoundingClientRect();
      // Position dropdown: align right edge with button, below button
      const left = Math.max(8, rect.right - 160);
      setPos({ top: rect.bottom + 4, left });
    }
    setOpen((v) => !v);
  }

  function handleAction(action: RowAction) {
    if (action.disabled) return;
    setOpen(false);
    // Delay action to allow dropdown to close and portal to unmount
    requestAnimationFrame(() => {
      action.onClick();
    });
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
          ref={menuRef}
          style={{ position: "fixed", top: pos.top, left: pos.left, zIndex: 9999 }}
          className="w-40 rounded-xl border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg"
        >
          {actions.map((action, i) => (
            <button
              key={i}
              onClick={() => handleAction(action)}
              disabled={action.disabled}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[11px] font-medium transition-colors",
                action.variant === "danger"
                  ? "text-[var(--danger)] hover:bg-[var(--danger-soft)]"
                  : "text-[var(--foreground)] hover:bg-[var(--muted)]",
                action.disabled && "cursor-not-allowed opacity-50 hover:bg-transparent"
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
