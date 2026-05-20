"use client";

/**
 * ToggleSwitch — iOS-style switch primitive used wherever a boolean
 * setting needs a tactile on/off control. Replaces the older
 * pill-button pattern (e.g. KisiKisiToggle pre-Phase 9.9).
 *
 * Accessibility: role=switch, aria-checked, keyboard space/enter to
 * flip. The label is associated via the parent label element when
 * supplied; otherwise consumers should add their own aria-label or
 * external <label htmlFor=...>.
 */

import { useId } from "react";
import { cn } from "@/lib/cn";

export interface ToggleSwitchProps {
  checked: boolean;
  onChange: (next: boolean) => void;
  label?: React.ReactNode;
  disabled?: boolean;
  size?: "sm" | "md";
  /** Inline tooltip via title attribute. */
  title?: string;
  /** Optional accessibility label when no visual label is provided. */
  ariaLabel?: string;
  /** Optional id passed through to the underlying button. */
  id?: string;
  /** Trailing element rendered after the label (e.g. lock icon). */
  trailing?: React.ReactNode;
}

export function ToggleSwitch({
  checked,
  onChange,
  label,
  disabled = false,
  size = "md",
  title,
  ariaLabel,
  id,
  trailing,
}: ToggleSwitchProps) {
  const autoId = useId();
  const buttonId = id ?? autoId;

  const dims =
    size === "sm"
      ? { track: "h-4 w-7", thumb: "h-3 w-3", translate: "translate-x-3" }
      : { track: "h-5 w-9", thumb: "h-4 w-4", translate: "translate-x-4" };

  function handleToggle() {
    if (disabled) return;
    onChange(!checked);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLButtonElement>) {
    if (disabled) return;
    if (e.key === " " || e.key === "Enter") {
      e.preventDefault();
      onChange(!checked);
    }
  }

  const switchEl = (
    <button
      id={buttonId}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      title={title}
      disabled={disabled}
      onClick={handleToggle}
      onKeyDown={handleKeyDown}
      className={cn(
        "relative inline-flex shrink-0 items-center rounded-full transition-colors duration-200 ease-out",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand)]/40 focus-visible:ring-offset-1 focus-visible:ring-offset-[var(--background)]",
        dims.track,
        checked
          ? "bg-[var(--primary)]"
          : "bg-[var(--muted)] border border-[var(--border)]",
        disabled && "cursor-not-allowed opacity-50",
      )}
    >
      <span
        aria-hidden
        className={cn(
          "pointer-events-none inline-block transform rounded-full bg-white shadow-sm transition-transform duration-200 ease-out",
          dims.thumb,
          checked ? dims.translate : "translate-x-0.5",
        )}
      />
    </button>
  );

  if (!label && !trailing) {
    return switchEl;
  }

  return (
    <label
      htmlFor={buttonId}
      className={cn(
        "inline-flex items-center gap-2 select-none",
        disabled ? "cursor-not-allowed" : "cursor-pointer",
      )}
    >
      {label && (
        <span
          className={cn(
            "text-[12px] font-medium",
            disabled
              ? "text-[var(--muted-foreground)]"
              : "text-[var(--foreground)]",
          )}
        >
          {label}
        </span>
      )}
      {switchEl}
      {trailing}
    </label>
  );
}
