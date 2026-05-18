"use client";

import { createContext, useCallback, useContext, useState } from "react";
import { cn } from "@/lib/cn";
import { X } from "lucide-react";

type ToastTone = "success" | "error" | "info" | "warning";

interface Toast {
  id: string;
  tone: ToastTone;
  title: string;
  description?: string;
}

interface ToastContextValue {
  toast: (t: Omit<Toast, "id">) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

const toneBorder: Record<ToastTone, string> = {
  success: "border-l-[var(--success)]",
  error: "border-l-[var(--danger)]",
  info: "border-l-[var(--info)]",
  warning: "border-l-[var(--warning)]",
};

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((t: Omit<Toast, "id">) => {
    const id = Math.random().toString(36).slice(2);
    setToasts((prev) => [...prev, { ...t, id }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((toast) => toast.id !== id));
    }, 4000);
  }, []);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2" role="status" aria-live="polite">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={cn(
              "flex items-start gap-3 rounded-xl border border-l-4 bg-[var(--card)] p-4 shadow-sm",
              "min-w-[300px] max-w-[400px]",
              toneBorder[t.tone]
            )}
          >
            <div className="flex-1 min-w-0">
              <p className="text-[12px] font-semibold text-[var(--foreground)]">{t.title}</p>
              {t.description && (
                <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{t.description}</p>
              )}
            </div>
            <button
              onClick={() => dismiss(t.id)}
              className="shrink-0 text-[var(--muted-foreground)] hover:text-[var(--foreground)] transition-colors"
              aria-label="Dismiss"
            >
              <X size={13} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
