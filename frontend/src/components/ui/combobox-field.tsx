"use client";

import * as React from "react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Plus } from "lucide-react";
import { cn } from "@/lib/cn";

export interface ComboboxOption {
  value: string;
  label: string;
}

interface ComboboxFieldProps {
  label: string;
  value: string;
  inputValue: string;
  options: ComboboxOption[];
  onInputChange: (value: string) => void;
  onSelect: (option: ComboboxOption) => void;
  onCreate?: (typedValue: string) => void;
  createLabel?: (typedValue: string) => string;
  error?: string;
  helperText?: string;
  prefix?: React.ReactNode;
  disabled?: boolean;
}

export function ComboboxField({
  label,
  value,
  inputValue,
  options,
  onInputChange,
  onSelect,
  onCreate,
  createLabel,
  error,
  helperText,
  prefix,
  disabled,
}: ComboboxFieldProps) {
  const [open, setOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const dismissedErrorRef = useRef<string | undefined>(undefined);
  const visibleError = error && error !== dismissedErrorRef.current ? error : undefined;
  const normalizedQuery = inputValue.trim().toLowerCase();
  const filtered = useMemo(() => {
    if (!normalizedQuery) return options.slice(0, 12);
    return options
      .filter((option) => option.label.toLowerCase().includes(normalizedQuery))
      .slice(0, 12);
  }, [normalizedQuery, options]);
  const exactMatch = options.some((option) => option.label.trim().toLowerCase() === normalizedQuery);
  const canCreate = !!onCreate && normalizedQuery.length > 0 && !exactMatch;
  const hasValue = inputValue.trim().length > 0;
  const isFloating = focused || open || hasValue;

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
        setFocused(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  function select(option: ComboboxOption) {
    if (error) dismissedErrorRef.current = error;
    onSelect(option);
    setOpen(false);
  }

  function create() {
    const typedValue = inputValue.trim();
    if (!typedValue || !onCreate) return;
    if (error) dismissedErrorRef.current = error;
    onCreate(typedValue);
    setOpen(false);
  }

  return (
    <div className="relative w-full" ref={ref}>
      <div
        className={cn(
          "relative flex h-11 items-center rounded-lg border bg-[var(--card)] transition-all",
          focused || open
            ? "border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]"
            : visibleError
              ? "border-[var(--danger)]"
              : "border-[var(--border)] hover:border-[var(--border-strong)]",
          disabled && "cursor-not-allowed opacity-60",
        )}
      >
        {prefix && (
          <div className={cn(
            "ml-2 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors",
            focused || open ? "border-[var(--field-focus)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)]",
            visibleError && "border-[var(--danger)] bg-[var(--danger-soft)] text-[var(--danger)]",
          )}>
            {prefix}
          </div>
        )}
        <input
          disabled={disabled}
          value={inputValue}
          onFocus={() => { setFocused(true); setOpen(true); }}
          onChange={(e) => {
            if (error) dismissedErrorRef.current = error;
            onInputChange(e.target.value);
            setOpen(true);
          }}
          placeholder=" "
          className={cn(
            "peer h-full w-full bg-transparent pb-1 pt-3.5 text-[13px] font-medium text-[var(--foreground)] outline-none disabled:cursor-not-allowed",
            prefix ? "pl-2 pr-3" : "px-3",
          )}
          aria-invalid={!!visibleError}
        />
        <label
          className={cn(
            "pointer-events-none absolute transition-all duration-150",
            prefix ? "left-11" : "left-3",
            isFloating ? "top-1 text-[10px] font-medium" : "top-1/2 -translate-y-1/2 text-[13px]",
            visibleError ? "text-[var(--danger)]" : focused || open ? "text-[var(--brand)]" : "text-[var(--muted-foreground)]",
          )}
        >
          {label}
        </label>
      </div>

      {open && !disabled && (
        <div className="absolute z-30 mt-1 max-h-64 w-full overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg">
          {filtered.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => select(option)}
              className={cn(
                "flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-2 text-left text-[12px] transition-colors",
                option.value === value
                  ? "bg-[var(--muted)] font-medium text-[var(--foreground)]"
                  : "text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]",
              )}
            >
              <span>{option.label}</span>
              {option.value === value && <Check size={13} className="text-[var(--brand)]" />}
            </button>
          ))}
          {filtered.length === 0 && (
            <p className="px-3 py-2 text-[12px] text-[var(--muted-foreground)]">Tidak ada subject yang cocok.</p>
          )}
          {canCreate && (
            <button
              type="button"
              onClick={create}
              className="mt-1 flex w-full items-center gap-2 rounded-md border border-dashed border-[var(--border-strong)] bg-[var(--accent)] px-2.5 py-2 text-left text-[12px] font-semibold text-[var(--foreground)] transition-colors hover:bg-[var(--muted)]"
            >
              <Plus size={13} />
              {createLabel ? createLabel(inputValue.trim()) : `Tambah “${inputValue.trim()}”`}
            </button>
          )}
        </div>
      )}

      {visibleError && <p className="mt-1 text-[11px] font-medium text-[var(--danger)]" role="alert">{visibleError}</p>}
      {helperText && !visibleError && <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{helperText}</p>}
    </div>
  );
}
