"use client";

import * as React from "react";
import { Loader2 } from "lucide-react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/cn";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 rounded-xl text-sm font-semibold transition-all duration-200 outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] disabled:pointer-events-none disabled:opacity-55 cursor-pointer",
  {
    variants: {
      variant: {
        primary:
          "bg-[color:var(--brand)] px-4 py-2.5 text-white shadow-sm hover:bg-[color:var(--brand-strong)] active:scale-[0.97]",
        secondary:
          "border border-[color:var(--border-strong)] bg-[color:var(--surface)] px-4 py-2.5 text-[color:var(--foreground)] hover:bg-[color:var(--surface-subtle)] active:scale-[0.97]",
        danger:
          "bg-[color:var(--danger)] px-4 py-2.5 text-white shadow-sm hover:opacity-90 active:scale-[0.97]",
        ghost:
          "px-3 py-2 text-[color:var(--foreground-muted)] hover:bg-[color:var(--surface-subtle)] hover:text-[color:var(--foreground)]",
      },
      size: {
        default: "h-10",
        sm: "h-8 rounded-lg px-3 text-xs",
        lg: "h-12 rounded-2xl px-5 text-sm",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  loading?: boolean;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ children, className, disabled, loading = false, variant, size, ...props }, ref) => {
    return (
      <button
        aria-busy={loading || undefined}
        className={cn(buttonVariants({ variant, size }), className)}
        disabled={disabled || loading}
        ref={ref}
        {...props}
      >
        {loading && <Loader2 className="h-4 w-4 animate-spin" />}
        {children}
      </button>
    );
  }
);

Button.displayName = "Button";
export { buttonVariants };
