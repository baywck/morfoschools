"use client";

import * as React from "react";
import { useState, useRef, useEffect } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/cn";

interface SelectOption {
  value: string;
  label: string;
}

interface SelectFieldProps {
  label: string;
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  error?: string;
  helperText?: string;
  prefix?: React.ReactNode;
  placeholder?: string;
  disabled?: boolean;
}

export function SelectField({ label, value, options, onChange, error, helperText, prefix, placeholder, disabled }: SelectFieldProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const selectedOption = options.find((o) => o.value === value);
  const displayText = selectedOption?.label || placeholder || "";
  const hasSelection = !!selectedOption;
  const isFloating = open || hasSelection;

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  return (
    <div className="w-full relative" ref={ref}>
      <div
        onClick={() => { if (!disabled) setOpen((v) => !v); }}
        className={cn(
          "relative flex h-11 items-center rounded-lg border bg-[var(--card)] transition-all",
          disabled ? "opacity-50 cursor-not-allowed" : "cursor-pointer",
          open
            ? "border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]"
            : error
              ? "border-[var(--danger)]"
              : disabled
                ? "border-[var(--border)]"
                : "border-[var(--border)] hover:border-[var(--border-strong)]"
        )}
      >
        {prefix && (
          <div className={cn(
            "ml-2 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors",
            open ? "border-[var(--field-focus)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)]"
          )}>
            {prefix}
          </div>
        )}

        <div className={cn("flex-1 relative h-full", prefix ? "pl-2 pr-3" : "px-3")}>
          {/* Floating label */}
          <span className={cn(
            "pointer-events-none absolute transition-all duration-150",
            prefix ? "left-2" : "left-3",
            isFloating
              ? "top-1 text-[10px] font-medium"
              : "top-1/2 -translate-y-1/2 text-[13px]",
            error ? "text-[var(--danger)]" : open ? "text-[var(--brand)]" : "text-[var(--muted-foreground)]"
          )}>
            {label}
          </span>
          {/* Display value */}
          <span className={cn(
            "absolute text-[13px] font-medium",
            prefix ? "left-2" : "left-3",
            isFloating ? "bottom-1.5" : "hidden",
            hasSelection ? "text-[var(--foreground)]" : "text-[var(--muted-foreground)]"
          )}>
            {displayText}
          </span>
        </div>

        <ChevronDown size={14} className={cn("mr-3 shrink-0 text-[var(--muted-foreground)] transition-transform", open && "rotate-180")} />
      </div>

      {/* Dropdown */}
      {open && (
        <div className="absolute z-30 mt-1 max-h-56 w-full overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg">
          {options.length === 0 ? (
            <p className="px-3 py-2 text-[12px] text-[var(--muted-foreground)]">No options</p>
          ) : (
            options.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => { onChange(opt.value); setOpen(false); }}
                className={cn(
                  "flex w-full items-center rounded-md px-2.5 py-2 text-[12px] transition-colors",
                  opt.value === value
                    ? "bg-[var(--muted)] text-[var(--foreground)] font-medium"
                    : "text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]"
                )}
              >
                {opt.label}
              </button>
            ))
          )}
        </div>
      )}

      {error && <p className="mt-1 text-[11px] font-medium text-[var(--danger)]" role="alert">{error}</p>}
      {helperText && !error && <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{helperText}</p>}
    </div>
  );
}
