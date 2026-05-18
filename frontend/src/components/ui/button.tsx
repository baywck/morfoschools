"use client";

import * as React from "react";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/cn";

type ButtonVariant = "primary" | "secondary" | "outline" | "ghost" | "danger";
type ButtonSize = "sm" | "md" | "lg" | "icon";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
}

const variantClass: Record<ButtonVariant, string> = {
  primary: "bg-[var(--primary)] text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.98]",
  secondary: "bg-[var(--muted)] text-[var(--foreground)] hover:bg-[var(--border)] active:scale-[0.98]",
  outline: "border border-[var(--border)] bg-[var(--card)] text-[var(--foreground)] hover:bg-[var(--muted)] active:scale-[0.98]",
  ghost: "bg-transparent text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]",
  danger: "bg-[var(--danger)] text-white shadow-sm hover:opacity-90 active:scale-[0.98]",
};

const sizeClass: Record<ButtonSize, string> = {
  sm: "h-8 px-3 text-xs gap-1.5 rounded-md",
  md: "h-8 px-4 text-xs gap-1.5 rounded-lg",
  lg: "h-9 px-5 text-sm gap-2 rounded-lg",
  icon: "h-8 w-8 rounded-lg",
};

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ children, className, disabled, loading = false, variant = "primary", size = "md", ...props }, ref) => {
    return (
      <button
        ref={ref}
        aria-busy={loading || undefined}
        disabled={disabled || loading}
        className={cn(
          "inline-flex items-center justify-center whitespace-nowrap font-medium transition-all duration-150",
          "focus-visible:ring-2 focus-visible:ring-[var(--primary)]/20 focus-visible:ring-offset-1",
          "disabled:pointer-events-none disabled:opacity-50",
          variantClass[variant],
          sizeClass[size],
          className
        )}
        {...props}
      >
        {loading && (
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
        )}
        {children}
      </button>
    );
  }
);

Button.displayName = "Button";
