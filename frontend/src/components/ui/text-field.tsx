"use client";

import * as React from "react";
import { useState } from "react";
import { cn } from "@/lib/cn";

interface TextFieldProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "placeholder" | "prefix"> {
  label: string;
  error?: string;
  helperText?: string;
  prefix?: React.ReactNode;
  suffix?: React.ReactNode;
}

export const TextField = React.forwardRef<HTMLInputElement, TextFieldProps>(
  ({ label, error, helperText, prefix, suffix, id, className, onFocus, onBlur, value, defaultValue, disabled, ...props }, ref) => {
    const inputId = id || label.toLowerCase().replace(/\s+/g, "-");
    const [focused, setFocused] = useState(false);
    const [internalValue, setInternalValue] = useState(defaultValue || "");

    const currentValue = value !== undefined ? value : internalValue;
    const hasValue = String(currentValue).length > 0;
    const floatLabel = focused || hasValue;

    return (
      <div className="w-full">
        <div
          className={cn(
            "relative min-h-[62px] rounded-2xl border-2 bg-[var(--field-bg)] transition-all duration-200",
            focused
              ? "border-[var(--field-focus)] shadow-[0_0_0_3px_var(--field-ring)]"
              : error
                ? "border-[var(--danger)] bg-[var(--danger-soft)]"
                : "border-[var(--field-border)] hover:border-[var(--border-strong)]",
            disabled && "opacity-60 cursor-not-allowed",
            className
          )}
        >
          {/* Prefix icon */}
          {prefix && (
            <div
              className={cn(
                "pointer-events-none absolute left-2.5 inset-y-2.5 flex w-10 items-center justify-center rounded-xl border bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors",
                focused ? "border-[var(--field-focus)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)]",
                error && "border-[var(--danger)] bg-[var(--danger-soft)] text-[var(--danger)]"
              )}
            >
              {prefix}
            </div>
          )}

          {/* Input */}
          <input
            id={inputId}
            ref={ref}
            type="text"
            disabled={disabled}
            value={value}
            defaultValue={defaultValue}
            onFocus={(e) => { setFocused(true); onFocus?.(e); }}
            onBlur={(e) => { setFocused(false); onBlur?.(e); }}
            onChange={(e) => { if (value === undefined) setInternalValue(e.target.value); props.onChange?.(e); }}
            placeholder=" "
            className={cn(
              "h-[58px] w-full bg-transparent px-4 pb-2.5 pt-7 text-sm font-semibold text-[var(--foreground)] outline-none placeholder:text-transparent",
              prefix && "pl-[4rem]",
              suffix && "pr-16",
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
              "pointer-events-none absolute select-none transition-all duration-200",
              prefix ? "left-[4rem]" : "left-4",
              floatLabel
                ? "top-2.5 text-[11px] font-semibold"
                : "top-1/2 -translate-y-1/2 text-sm font-medium",
              error ? "text-[var(--danger)]" : "text-[var(--muted-foreground)]"
            )}
          >
            {label}
          </label>

          {/* Suffix */}
          {suffix && (
            <div className="pointer-events-none absolute right-2.5 inset-y-2.5 flex w-10 items-center justify-center rounded-xl text-[var(--muted-foreground)]">
              {suffix}
            </div>
          )}
        </div>

        {/* Error / Helper */}
        {error && (
          <p id={`${inputId}-error`} className="mt-1.5 flex items-center gap-1.5 pl-1 text-[11px] font-medium text-[var(--danger)]" role="alert">
            {error}
          </p>
        )}
        {helperText && !error && (
          <p className="mt-1.5 pl-1 text-[11px] text-[var(--muted-foreground)]">{helperText}</p>
        )}
      </div>
    );
  }
);

TextField.displayName = "TextField";
