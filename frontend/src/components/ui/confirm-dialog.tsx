"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Button } from "@/components/ui/button";
import { AlertTriangle, X } from "lucide-react";
import { cn } from "@/lib/cn";

interface DialogProps {
  open: boolean;
  title: string;
  description?: string;
  children: React.ReactNode;
  width?: "sm" | "md" | "lg";
  onClose: () => void;
  footer?: React.ReactNode;
}

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function Dialog({ open, title, description, children, width = "md", onClose, footer }: DialogProps) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => { setMounted(true); }, []);
  if (!open || !mounted) return null;

  return createPortal(
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/30 backdrop-blur-[1px]">
      <div className={cn("w-full rounded-xl border border-[var(--border)] bg-[var(--card)] p-5 shadow-xl max-h-[90vh] flex flex-col", width === "sm" && "max-w-sm", width === "md" && "max-w-md", width === "lg" && "max-w-2xl")}>
        <div className="flex shrink-0 items-start justify-between gap-3 pb-4">
          <div>
            <h3 className="text-[16px] font-bold text-[var(--foreground)]">{title}</h3>
            {description && <p className="mt-1 text-[13px] text-[var(--muted-foreground)]">{description}</p>}
          </div>
          <button type="button" onClick={onClose} className="rounded-md p-1 text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)]"><X size={16} /></button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto pr-2">
          {children}
        </div>
        {footer && <div className="mt-5 shrink-0 pt-4 border-t border-[var(--border)]">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  destructive = false,
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  // Portal to <body> so the dialog escapes any ancestor containing block
  // (e.g. backdrop-filter on PageShell's sticky header would otherwise clip
  // a position:fixed dialog to the header strip — see ADR-0009 follow-up).
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  if (!open || !mounted) return null;

  return createPortal(
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/30 backdrop-blur-[1px]">
      <div className="w-full max-w-sm rounded-xl border border-[var(--border)] bg-[var(--card)] p-5 shadow-xl">
        <div className="flex items-start gap-3">
          {destructive && (
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--danger-soft)] text-[var(--danger)]">
              <AlertTriangle size={16} />
            </div>
          )}
          <div>
            <h3 className="text-[14px] font-semibold text-[var(--foreground)]">{title}</h3>
            <p className="mt-1.5 text-[12px] text-[var(--muted-foreground)]">{description}</p>
          </div>
        </div>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onCancel} disabled={loading}>
            {cancelLabel}
          </Button>
          <Button
            variant={destructive ? "danger" : "primary"}
            size="sm"
            onClick={onConfirm}
            loading={loading}
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
