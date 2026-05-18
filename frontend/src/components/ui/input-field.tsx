"use client";

import * as React from "react";
import { useState } from "react";
import { cn } from "@/lib/cn";
import { FieldShell } from "./field-shell";

interface InputFieldProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "placeholder"> {
  label: string;
  error?: string;
  hint?: string;
  icon?: React.ReactNode;
}

export const InputField = React.forwardRef<HTMLInputElement, InputFieldProps>(
  ({ label, error, hint, icon, id, className, onFocus, onBlur, value, defaultValue, ...props }, ref) => {
    const inputId = id || label.toLowerCase().replace(/\s+/g, "-");
    const [focused, setFocused] = useState(false);
    const [internalValue, setInternalValue] = useState(defaultValue || "");

    const currentValue = value !== undefined ? value : internalValue;

    return (
      <FieldShell
        id={inputId}
        label={label}
        value={currentValue as string}
        focused={focused}
        error={error}
        hint={hint}
        icon={icon}
      >
        <input
          id={inputId}
          ref={ref}
          value={value}
          defaultValue={defaultValue}
          onFocus={(e) => { setFocused(true); onFocus?.(e); }}
          onBlur={(e) => { setFocused(false); onBlur?.(e); }}
          onChange={(e) => { if (value === undefined) setInternalValue(e.target.value); props.onChange?.(e); }}
          className={cn(
            "h-11 w-full bg-transparent pb-1 pt-5 text-sm outline-none",
            icon ? "px-0 pr-3" : "px-3",
            error && "text-[var(--danger)]",
            className
          )}
          aria-invalid={!!error}
          aria-describedby={error ? `${inputId}-error` : undefined}
          {...props}
        />
      </FieldShell>
    );
  }
);

InputField.displayName = "InputField";
