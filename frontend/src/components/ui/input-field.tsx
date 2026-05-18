"use client";

import * as React from "react";
import { cn } from "@/lib/cn";

interface InputFieldProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "size"> {
  label: string;
  error?: string;
  helperText?: string;
}

export const InputField = React.forwardRef<HTMLInputElement, InputFieldProps>(
  ({ label, error, helperText, id, className, ...props }, ref) => {
    const inputId = id || label.toLowerCase().replace(/\s+/g, "-");

    return (
      <div className="w-full space-y-1.5">
        <label htmlFor={inputId} className="block text-[12px] font-medium text-[var(--foreground)]">
          {label}
        </label>
        <input
          id={inputId}
          ref={ref}
          className={cn(
            "flex h-9 w-full rounded-lg border bg-[var(--card)] px-3 text-[13px] text-[var(--foreground)] outline-none transition-all",
            "placeholder:text-[var(--muted-foreground)]",
            "focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)]",
            error
              ? "border-[var(--danger)] focus:border-[var(--danger)] focus:ring-[var(--danger)]/10"
              : "border-[var(--border)] hover:border-[var(--border-strong)]",
            props.disabled && "opacity-60 cursor-not-allowed",
            className
          )}
          aria-invalid={!!error}
          aria-describedby={error ? `${inputId}-error` : undefined}
          {...props}
        />
        {error && (
          <p id={`${inputId}-error`} className="text-[11px] font-medium text-[var(--danger)]" role="alert">
            {error}
          </p>
        )}
        {helperText && !error && (
          <p className="text-[11px] text-[var(--muted-foreground)]">{helperText}</p>
        )}
      </div>
    );
  }
);

InputField.displayName = "InputField";
