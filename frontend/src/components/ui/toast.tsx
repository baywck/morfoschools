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

const toneStyles: Record<ToastTone, string> = {
  success: "bg-[var(--success-soft)] border-[var(--success)]/20 text-[var(--success)]",
  error: "bg-[var(--danger-soft)] border-[var(--danger)]/20 text-[var(--danger)]",
  info: "bg-[var(--info-soft)] border-[var(--info)]/20 text-[var(--info)]",
  warning: "bg-[var(--warning-soft)] border-[var(--warning)]/20 text-[var(--warning)]",
};

const toneIcon: Record<ToastTone, string> = {
  success: "bg-[var(--success)]/10",
  error: "bg-[var(--danger)]/10",
  info: "bg-[var(--info)]/10",
  warning: "bg-[var(--warning)]/10",
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
              "flex items-start gap-3 rounded-xl border p-3.5 shadow-sm backdrop-blur-sm",
              "min-w-[300px] max-w-[400px] animate-[slideIn_0.2s_ease-out]",
              toneStyles[t.tone]
            )}
          >
            <div className={cn("mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full", toneIcon[t.tone])}>
              <span className="block h-2 w-2 rounded-full bg-current" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-[12px] font-semibold">{t.title}</p>
              {t.description && (
                <p className="mt-0.5 text-[11px] opacity-80">{t.description}</p>
              )}
            </div>
            <button
              onClick={() => dismiss(t.id)}
              className="shrink-0 opacity-60 hover:opacity-100 transition-opacity"
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
