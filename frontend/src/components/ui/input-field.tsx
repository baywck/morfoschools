"use client";

import * as React from "react";
import { cn } from "@/lib/cn";

interface InputFieldProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
  icon?: React.ReactNode;
}

export const InputField = React.forwardRef<HTMLInputElement, InputFieldProps>(
  ({ label, error, icon, className, id, ...props }, ref) => {
    const inputId = id || label.toLowerCase().replace(/\s+/g, "-");

    return (
      <div className="relative">
        <div className="relative">
          {icon && (
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-[color:var(--foreground-muted)]">
              {icon}
            </span>
          )}
          <input
            id={inputId}
            ref={ref}
            placeholder=" "
            className={cn(
              "peer w-full rounded-xl border bg-[color:var(--surface)] px-4 pb-2 pt-5 text-sm text-[color:var(--foreground)] outline-none transition-all",
              "border-[color:var(--border)] focus:border-[color:var(--brand)] focus:ring-2 focus:ring-[color:var(--ring)]",
              error && "border-[color:var(--danger)] focus:border-[color:var(--danger)] focus:ring-[color:var(--danger)]/20",
              icon && "pl-10",
              className
            )}
            aria-invalid={!!error}
            aria-describedby={error ? `${inputId}-error` : undefined}
            {...props}
          />
          <label
            htmlFor={inputId}
            className={cn(
              "absolute left-4 top-1/2 -translate-y-1/2 text-sm text-[color:var(--foreground-muted)] transition-all pointer-events-none",
              "peer-placeholder-shown:top-1/2 peer-placeholder-shown:text-sm",
              "peer-focus:top-2.5 peer-focus:text-xs peer-focus:text-[color:var(--brand)]",
              "peer-[:not(:placeholder-shown)]:top-2.5 peer-[:not(:placeholder-shown)]:text-xs",
              icon && "left-10"
            )}
          >
            {label}
          </label>
        </div>
        {error && (
          <p id={`${inputId}-error`} className="mt-1 text-xs text-[color:var(--danger)]" role="alert">
            {error}
          </p>
        )}
      </div>
    );
  }
);

InputField.displayName = "InputField";
