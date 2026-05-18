"use client";

import * as React from "react";
import { cn } from "@/lib/cn";
import { Loader2 } from "lucide-react";

type ButtonVariant = "primary" | "secondary" | "outline" | "ghost" | "danger";
type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
}

const variantStyles: Record<ButtonVariant, string> = {
  primary: "bg-[var(--primary)] text-[var(--primary-foreground)] shadow-sm hover:opacity-90",
  secondary: "bg-[var(--muted)] text-[var(--foreground)] border border-[var(--border)] hover:border-[var(--border-strong)]",
  outline: "border-2 border-[var(--border-strong)] text-[var(--foreground)] hover:bg-[var(--muted)]",
  ghost: "text-[var(--muted-foreground)] hover:text-[var(--foreground)] hover:bg-[var(--muted)]",
  danger: "bg-[var(--danger)] text-white shadow-sm hover:opacity-90",
};

const sizeStyles: Record<ButtonSize, string> = {
  sm: "h-8 px-3 text-[11px] rounded-lg gap-1.5",
  md: "h-9 px-4 text-[12px] rounded-lg gap-2",
  lg: "h-11 px-5 text-[13px] rounded-xl gap-2",
};

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ children, className, disabled, loading = false, variant = "primary", size = "md", ...props }, ref) => {
    return (
      <button
        ref={ref}
        aria-busy={loading || undefined}
        disabled={disabled || loading}
        className={cn(
          "inline-flex items-center justify-center font-semibold transition-all duration-150 active:scale-[0.97] disabled:opacity-50 disabled:pointer-events-none",
          variantStyles[variant],
          sizeStyles[size],
          className
        )}
        {...props}
      >
        {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
        {children}
      </button>
    );
  }
);

Button.displayName = "Button";
