"use client";

import * as React from "react";
import { useRef, useState } from "react";
import { cn } from "@/lib/cn";

interface TextareaFieldProps extends Omit<React.TextareaHTMLAttributes<HTMLTextAreaElement>, "placeholder"> {
  label: string;
  error?: string;
  helperText?: string;
}

export const TextareaField = React.forwardRef<HTMLTextAreaElement, TextareaFieldProps>(
  ({ label, error, helperText, id, className, onFocus, onBlur, onChange, value, defaultValue, disabled, rows = 4, ...props }, ref) => {
    const inputId = id || label.toLowerCase().replace(/\s+/g, "-");
    const [focused, setFocused] = useState(false);
    const [internalValue, setInternalValue] = useState(defaultValue || "");
    const dismissedErrorRef = useRef<string | undefined>(undefined);
    const visibleError = error && error !== dismissedErrorRef.current ? error : undefined;
    const currentValue = value !== undefined ? value : internalValue;
    const hasValue = String(currentValue).length > 0;
    const isFloating = focused || hasValue;

    function handleChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
      if (error) dismissedErrorRef.current = error;
      if (value === undefined) setInternalValue(e.target.value);
      onChange?.(e);
    }

    return (
      <div className="w-full">
        <div
          className={cn(
            "relative rounded-lg border bg-[var(--card)] transition-all",
            focused
              ? "border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]"
              : visibleError
                ? "border-[var(--danger)]"
                : "border-[var(--border)] hover:border-[var(--border-strong)]",
            disabled && "cursor-not-allowed opacity-60",
            className,
          )}
        >
          <textarea
            id={inputId}
            ref={ref}
            disabled={disabled}
            value={value}
            defaultValue={defaultValue}
            rows={rows}
            onFocus={(e) => {
              setFocused(true);
              onFocus?.(e);
            }}
            onBlur={(e) => {
              setFocused(false);
              onBlur?.(e);
            }}
            onChange={handleChange}
            placeholder=" "
            className={cn(
              "min-h-24 w-full resize-y bg-transparent px-3 pb-2 pt-5 text-[13px] font-medium leading-relaxed text-[var(--foreground)] outline-none",
              disabled && "cursor-not-allowed",
            )}
            aria-invalid={!!visibleError}
            aria-describedby={visibleError ? `${inputId}-error` : undefined}
            {...props}
          />
          <label
            htmlFor={inputId}
            className={cn(
              "pointer-events-none absolute left-3 transition-all duration-150",
              isFloating ? "top-1 text-[10px] font-medium" : "top-3.5 text-[13px]",
              visibleError ? "text-[var(--danger)]" : focused ? "text-[var(--brand)]" : "text-[var(--muted-foreground)]",
            )}
          >
            {label}
          </label>
        </div>
        {visibleError && (
          <p id={`${inputId}-error`} className="mt-1 text-[11px] font-medium text-[var(--danger)]" role="alert">
            {visibleError}
          </p>
        )}
        {helperText && !visibleError && (
          <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{helperText}</p>
        )}
      </div>
    );
  },
);

TextareaField.displayName = "TextareaField";
