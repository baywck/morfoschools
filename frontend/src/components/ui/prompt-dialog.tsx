"use client";

/**
 * PromptDialog — single-input modal that replaces native window.prompt.
 *
 * Why a custom dialog: the rich editor toolbar collects link URLs, image
 * URLs, and LaTeX expressions. Native window.prompt is jarring (browser-
 * styled, breaks the Morfosis aesthetic, can be blocked by the OS, no
 * markdown/keyboard niceties).
 *
 * API: imperative — call `await promptInput({ title, ... })` from any
 * handler. Returns the entered string, or null on cancel.
 *
 * Implementation: a hidden portal-mounted React tree owned by the
 * provider; the imperative `promptInput` resolves a Promise the
 * provider is holding. One provider per app at the layout level.
 */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { Button } from "@/components/ui/button";

interface PromptOptions {
  title: string;
  description?: string;
  label?: string;
  defaultValue?: string;
  placeholder?: string;
  /** Use a multi-line textarea instead of single-line input. */
  multiline?: boolean;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Inline validator. Return error string to block submit, "" or null to allow. */
  validate?: (value: string) => string | null;
}

type PromptResolver = (value: string | null) => void;

interface PromptContextShape {
  prompt: (options: PromptOptions) => Promise<string | null>;
}

const PromptContext = createContext<PromptContextShape | null>(null);

export function usePrompt(): PromptContextShape["prompt"] {
  const ctx = useContext(PromptContext);
  if (!ctx) {
    throw new Error("usePrompt must be used inside <PromptProvider>");
  }
  return ctx.prompt;
}

interface PromptState {
  options: PromptOptions;
  resolver: PromptResolver;
}

export function PromptProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<PromptState | null>(null);
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [mounted, setMounted] = useState(false);
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  const prompt = useCallback((options: PromptOptions) => {
    return new Promise<string | null>((resolve) => {
      setValue(options.defaultValue ?? "");
      setError(null);
      setState({ options, resolver: resolve });
    });
  }, []);

  const close = useCallback(
    (result: string | null) => {
      if (!state) return;
      state.resolver(result);
      setState(null);
      setValue("");
      setError(null);
    },
    [state],
  );

  // Autofocus + select on open
  useEffect(() => {
    if (!state) return;
    const id = window.setTimeout(() => {
      inputRef.current?.focus();
      if (inputRef.current && "select" in inputRef.current) {
        inputRef.current.select();
      }
    }, 30);
    return () => window.clearTimeout(id);
  }, [state]);

  // Esc / Enter handling at the document level so the input's keydown
  // doesn't have to forward each event.
  useEffect(() => {
    if (!state) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        close(null);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [state, close]);

  function submit() {
    if (!state) return;
    const validator = state.options.validate;
    if (validator) {
      const err = validator(value);
      if (err) {
        setError(err);
        return;
      }
    }
    close(value);
  }

  const ctxValue = useMemo(() => ({ prompt }), [prompt]);

  return (
    <PromptContext.Provider value={ctxValue}>
      {children}
      {state && mounted &&
        createPortal(
          <div
            className="fixed inset-0 z-[110] flex items-center justify-center bg-black/30 backdrop-blur-[1px]"
            role="dialog"
            aria-modal="true"
            aria-labelledby="prompt-title"
            onClick={(e) => {
              if (e.target === e.currentTarget) close(null);
            }}
          >
            <div className="w-full max-w-sm rounded-xl border border-[var(--border)] bg-[var(--card)] p-5 shadow-xl">
              <h3
                id="prompt-title"
                className="text-[14px] font-semibold text-[var(--foreground)]"
              >
                {state.options.title}
              </h3>
              {state.options.description && (
                <p className="mt-1.5 text-[12px] text-[var(--muted-foreground)]">
                  {state.options.description}
                </p>
              )}
              <div className="mt-4">
                {state.options.label && (
                  <label className="mb-1.5 block text-[11px] font-medium text-[var(--muted-foreground)]">
                    {state.options.label}
                  </label>
                )}
                {state.options.multiline ? (
                  <textarea
                    ref={(node) => {
                      inputRef.current = node;
                    }}
                    value={value}
                    onChange={(e) => {
                      setValue(e.target.value);
                      if (error) setError(null);
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                        e.preventDefault();
                        submit();
                      }
                    }}
                    placeholder={state.options.placeholder}
                    rows={5}
                    className="w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-[13px] text-[var(--foreground)] placeholder:text-[var(--muted-foreground)] outline-none focus:border-[var(--brand)] focus:ring-2 focus:ring-[var(--field-ring)] font-mono"
                  />
                ) : (
                  <input
                    ref={(node) => {
                      inputRef.current = node;
                    }}
                    type="text"
                    value={value}
                    onChange={(e) => {
                      setValue(e.target.value);
                      if (error) setError(null);
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        submit();
                      }
                    }}
                    placeholder={state.options.placeholder}
                    className="h-10 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-[13px] text-[var(--foreground)] placeholder:text-[var(--muted-foreground)] outline-none focus:border-[var(--brand)] focus:ring-2 focus:ring-[var(--field-ring)]"
                  />
                )}
                {error && (
                  <p className="mt-1.5 text-[11px] font-medium text-[var(--danger)]" role="alert">
                    {error}
                  </p>
                )}
              </div>
              <div className="mt-5 flex justify-end gap-2">
                <Button variant="ghost" size="sm" onClick={() => close(null)}>
                  {state.options.cancelLabel ?? "Cancel"}
                </Button>
                <Button variant="primary" size="sm" onClick={submit}>
                  {state.options.confirmLabel ?? "OK"}
                </Button>
              </div>
            </div>
          </div>,
          document.body,
        )}
    </PromptContext.Provider>
  );
}
