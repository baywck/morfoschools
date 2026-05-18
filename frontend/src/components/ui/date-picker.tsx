"use client";

import { useState, useRef, useEffect } from "react";
import { ChevronLeft, ChevronRight, Calendar } from "lucide-react";
import { cn } from "@/lib/cn";

interface DatePickerProps {
  label: string;
  value: string; // YYYY-MM-DD
  onChange: (value: string) => void;
  error?: string;
  helperText?: string;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DAYS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];

function getDaysInMonth(year: number, month: number) {
  return new Date(year, month + 1, 0).getDate();
}

function getFirstDayOfMonth(year: number, month: number) {
  return new Date(year, month, 1).getDay();
}

function formatDisplay(value: string) {
  if (!value) return "";
  const d = new Date(value + "T00:00:00");
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
}

export function DatePicker({ label, value, onChange, error, helperText }: DatePickerProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const today = new Date();
  const selected = value ? new Date(value + "T00:00:00") : null;
  const [viewYear, setViewYear] = useState(selected?.getFullYear() || today.getFullYear());
  const [viewMonth, setViewMonth] = useState(selected?.getMonth() ?? today.getMonth());

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const isFloating = open || !!value;
  const daysInMonth = getDaysInMonth(viewYear, viewMonth);
  const firstDay = getFirstDayOfMonth(viewYear, viewMonth);

  function prevMonth() {
    if (viewMonth === 0) { setViewMonth(11); setViewYear(viewYear - 1); }
    else setViewMonth(viewMonth - 1);
  }

  function nextMonth() {
    if (viewMonth === 11) { setViewMonth(0); setViewYear(viewYear + 1); }
    else setViewMonth(viewMonth + 1);
  }

  function selectDay(day: number) {
    const m = String(viewMonth + 1).padStart(2, "0");
    const d = String(day).padStart(2, "0");
    onChange(`${viewYear}-${m}-${d}`);
    setOpen(false);
  }

  function isSelected(day: number) {
    if (!selected) return false;
    return selected.getFullYear() === viewYear && selected.getMonth() === viewMonth && selected.getDate() === day;
  }

  function isToday(day: number) {
    return today.getFullYear() === viewYear && today.getMonth() === viewMonth && today.getDate() === day;
  }

  return (
    <div className="w-full relative" ref={ref}>
      {/* Trigger */}
      <div
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "relative flex h-11 items-center rounded-lg border bg-[var(--card)] transition-all cursor-pointer",
          open
            ? "border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]"
            : error
              ? "border-[var(--danger)]"
              : "border-[var(--border)] hover:border-[var(--border-strong)]"
        )}
      >
        <div className="ml-2 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--muted)] text-[var(--muted-foreground)]">
          <Calendar size={14} />
        </div>
        <div className="flex-1 relative h-full pl-2 pr-3">
          <span className={cn(
            "pointer-events-none absolute transition-all duration-150 left-2",
            isFloating
              ? "top-1 text-[10px] font-medium"
              : "top-1/2 -translate-y-1/2 text-[13px]",
            error ? "text-[var(--danger)]" : open ? "text-[var(--brand)]" : "text-[var(--muted-foreground)]"
          )}>
            {label}
          </span>
          {value && (
            <span className="absolute bottom-1.5 left-2 text-[13px] font-medium text-[var(--foreground)]">
              {formatDisplay(value)}
            </span>
          )}
        </div>
      </div>

      {/* Calendar dropdown */}
      {open && (
        <div className="absolute z-50 mt-1 w-72 rounded-xl border border-[var(--border)] bg-[var(--card)] p-3 shadow-lg">
          {/* Header */}
          <div className="flex items-center justify-between mb-2">
            <button type="button" onClick={prevMonth} className="flex h-7 w-7 items-center justify-center rounded-md hover:bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors">
              <ChevronLeft size={14} />
            </button>
            <span className="text-[12px] font-semibold text-[var(--foreground)]">
              {MONTHS[viewMonth]} {viewYear}
            </span>
            <button type="button" onClick={nextMonth} className="flex h-7 w-7 items-center justify-center rounded-md hover:bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors">
              <ChevronRight size={14} />
            </button>
          </div>

          {/* Day headers */}
          <div className="grid grid-cols-7 mb-1">
            {DAYS.map((d) => (
              <div key={d} className="flex h-7 items-center justify-center text-[10px] font-medium text-[var(--muted-foreground)]">
                {d}
              </div>
            ))}
          </div>

          {/* Days grid */}
          <div className="grid grid-cols-7">
            {Array.from({ length: firstDay }).map((_, i) => (
              <div key={`empty-${i}`} className="h-8" />
            ))}
            {Array.from({ length: daysInMonth }).map((_, i) => {
              const day = i + 1;
              return (
                <button
                  key={day}
                  type="button"
                  onClick={() => selectDay(day)}
                  className={cn(
                    "flex h-8 w-8 items-center justify-center rounded-lg text-[12px] font-medium transition-all mx-auto",
                    isSelected(day)
                      ? "bg-[var(--primary)] text-[var(--primary-foreground)]"
                      : isToday(day)
                        ? "bg-[var(--brand-soft)] text-[var(--brand)]"
                        : "text-[var(--foreground)] hover:bg-[var(--muted)]"
                  )}
                >
                  {day}
                </button>
              );
            })}
          </div>

          {/* Today button */}
          <div className="mt-2 border-t border-[var(--border)] pt-2">
            <button
              type="button"
              onClick={() => {
                const m = String(today.getMonth() + 1).padStart(2, "0");
                const d = String(today.getDate()).padStart(2, "0");
                onChange(`${today.getFullYear()}-${m}-${d}`);
                setOpen(false);
              }}
              className="w-full h-7 rounded-md text-[11px] font-medium text-[var(--brand)] hover:bg-[var(--brand-soft)] transition-colors"
            >
              Today
            </button>
          </div>
        </div>
      )}

      {error && <p className="mt-1 text-[11px] font-medium text-[var(--danger)]" role="alert">{error}</p>}
      {helperText && !error && <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{helperText}</p>}
    </div>
  );
}
