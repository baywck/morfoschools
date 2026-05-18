"use client";

import * as React from "react";
import { useState } from "react";
import { cn } from "@/lib/cn";

interface InputFieldProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "placeholder" | "prefix"> {
  label: string;
  error?: string;
  helperText?: string;
  prefix?: React.ReactNode;
}

export const InputField = React.forwardRef<HTMLInputElement, InputFieldProps>(
  ({ label, error, helperText, prefix, id, className, onFocus, onBlur, value, defaultValue, disabled, ...props }, ref) => {
    const inputId = id || label.toLowerCase().replace(/\s+/g, "-");
    const [focused, setFocused] = useState(false);
    const [internalValue, setInternalValue] = useState(defaultValue || "");

    const currentValue = value !== undefined ? value : internalValue;
    const hasValue = String(currentValue).length > 0;
    const isFloating = focused || hasValue;

    return (
      <div className="w-full">
        <div
          className={cn(
            "relative flex h-11 items-center rounded-lg border bg-[var(--card)] transition-all",
            focused
              ? "border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]"
              : error
                ? "border-[var(--danger)]"
                : "border-[var(--border)] hover:border-[var(--border-strong)]",
            disabled && "opacity-60 cursor-not-allowed",
            className
          )}
        >
          {/* Prefix */}
          {prefix && (
            <div className={cn(
              "ml-2 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors",
              focused ? "border-[var(--field-focus)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)]",
              error && "border-[var(--danger)] bg-[var(--danger-soft)] text-[var(--danger)]"
            )}>
              {prefix}
            </div>
          )}

          {/* Input */}
          <input
            id={inputId}
            ref={ref}
            disabled={disabled}
            value={value}
            defaultValue={defaultValue}
            onFocus={(e) => { setFocused(true); onFocus?.(e); }}
            onBlur={(e) => { setFocused(false); onBlur?.(e); }}
            onChange={(e) => { if (value === undefined) setInternalValue(e.target.value); props.onChange?.(e); }}
            placeholder=" "
            className={cn(
              "peer h-full w-full bg-transparent text-[13px] font-medium text-[var(--foreground)] outline-none",
              prefix ? "pl-2 pr-3" : "px-3",
              "pt-3.5 pb-1",
              disabled && "cursor-not-allowed"
            )}
            aria-invalid={!!error}
            aria-describedby={error ? `${inputId}-error` : undefined}
            {...props}
          />

          {/* Floating label */}
          <label
            htmlFor={inputId}
            className={cn(
              "pointer-events-none absolute transition-all duration-150",
              prefix ? "left-11" : "left-3",
              isFloating
                ? "top-1 text-[10px] font-medium"
                : "top-1/2 -translate-y-1/2 text-[13px]",
              error ? "text-[var(--danger)]" : focused ? "text-[var(--brand)]" : "text-[var(--muted-foreground)]"
            )}
          >
            {label}
          </label>
        </div>

        {error && (
          <p id={`${inputId}-error`} className="mt-1 text-[11px] font-medium text-[var(--danger)]" role="alert">
            {error}
          </p>
        )}
        {helperText && !error && (
          <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{helperText}</p>
        )}
      </div>
    );
  }
);

InputField.displayName = "InputField";
