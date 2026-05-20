"use client";

import { useState, useRef, useEffect } from "react";
import { ChevronLeft, ChevronRight, Calendar, Clock } from "lucide-react";
import { cn } from "@/lib/cn";

interface DateTimePickerProps {
  label: string;
  /** ISO 8601 string ("2026-05-20T08:30:00Z") or empty */
  value: string;
  /** Receives ISO 8601 string in UTC */
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

function pad2(n: number) {
  return String(n).padStart(2, "0");
}

function formatDisplay(value: string) {
  if (!value) return "";
  const d = new Date(value);
  if (isNaN(d.getTime())) return "";
  // 20 May 2026, 14:30
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}, ${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}

export function DateTimePicker({
  label,
  value,
  onChange,
  error,
  helperText,
}: DateTimePickerProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const dismissedErrorRef = useRef<string | undefined>(undefined);

  const visibleError =
    error && error !== dismissedErrorRef.current ? error : undefined;

  // Initialize view from value or now
  const today = new Date();
  const selected = value ? new Date(value) : null;
  const [viewYear, setViewYear] = useState(selected?.getFullYear() ?? today.getFullYear());
  const [viewMonth, setViewMonth] = useState(selected?.getMonth() ?? today.getMonth());
  const [hours, setHours] = useState<number>(selected?.getHours() ?? 8);
  const [minutes, setMinutes] = useState<number>(selected?.getMinutes() ?? 0);

  useEffect(() => {
    if (!selected) return;
    setHours(selected.getHours());
    setMinutes(selected.getMinutes());
    // We intentionally don't sync viewYear/viewMonth on every value change to
    // avoid yanking the calendar away from the user mid-pick. They sync once
    // on first mount when `selected` is non-null.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

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
    if (viewMonth === 0) {
      setViewMonth(11);
      setViewYear(viewYear - 1);
    } else {
      setViewMonth(viewMonth - 1);
    }
  }

  function nextMonth() {
    if (viewMonth === 11) {
      setViewMonth(0);
      setViewYear(viewYear + 1);
    } else {
      setViewMonth(viewMonth + 1);
    }
  }

  function emit(year: number, month: number, day: number, h: number, m: number) {
    if (error) dismissedErrorRef.current = error;
    // Build local Date then export as ISO (UTC). The backend uses ISO 8601
    // and time.Parse(time.RFC3339) which accepts the Z-suffixed form.
    const d = new Date(year, month, day, h, m, 0, 0);
    onChange(d.toISOString());
  }

  function selectDay(day: number) {
    emit(viewYear, viewMonth, day, hours, minutes);
  }

  function changeTime(newH: number, newM: number) {
    setHours(newH);
    setMinutes(newM);
    if (!selected) return;
    emit(
      selected.getFullYear(),
      selected.getMonth(),
      selected.getDate(),
      newH,
      newM
    );
  }

  function isSelected(day: number) {
    if (!selected) return false;
    return (
      selected.getFullYear() === viewYear &&
      selected.getMonth() === viewMonth &&
      selected.getDate() === day
    );
  }

  function isToday(day: number) {
    return (
      today.getFullYear() === viewYear &&
      today.getMonth() === viewMonth &&
      today.getDate() === day
    );
  }

  return (
    <div className="w-full relative" ref={ref}>
      {/* Trigger — same shape as InputField/DatePicker so floating label aligns */}
      <div
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "relative flex h-11 items-center rounded-lg border bg-[var(--card)] transition-all cursor-pointer",
          open
            ? "border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]"
            : visibleError
            ? "border-[var(--danger)]"
            : "border-[var(--border)] hover:border-[var(--border-strong)]"
        )}
      >
        <div className="ml-2 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--muted)] text-[var(--muted-foreground)]">
          <Calendar size={14} />
        </div>
        <div className="flex-1 relative h-full pl-2 pr-3">
          <span
            className={cn(
              "pointer-events-none absolute transition-all duration-150 left-2",
              isFloating
                ? "top-1 text-[10px] font-medium"
                : "top-1/2 -translate-y-1/2 text-[13px]",
              visibleError
                ? "text-[var(--danger)]"
                : open
                ? "text-[var(--brand)]"
                : "text-[var(--muted-foreground)]"
            )}
          >
            {label}
          </span>
          {value && (
            <span className="absolute bottom-1.5 left-2 text-[13px] font-medium text-[var(--foreground)]">
              {formatDisplay(value)}
            </span>
          )}
        </div>
      </div>

      {/* Calendar + time dropdown */}
      {open && (
        <div className="absolute z-50 mt-1 w-72 rounded-xl border border-[var(--border)] bg-[var(--card)] p-3 shadow-lg">
          {/* Header */}
          <div className="flex items-center justify-between mb-2">
            <button
              type="button"
              onClick={prevMonth}
              className="flex h-7 w-7 items-center justify-center rounded-md hover:bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors"
            >
              <ChevronLeft size={14} />
            </button>
            <span className="text-[12px] font-semibold text-[var(--foreground)]">
              {MONTHS[viewMonth]} {viewYear}
            </span>
            <button
              type="button"
              onClick={nextMonth}
              className="flex h-7 w-7 items-center justify-center rounded-md hover:bg-[var(--muted)] text-[var(--muted-foreground)] transition-colors"
            >
              <ChevronRight size={14} />
            </button>
          </div>

          {/* Day headers */}
          <div className="grid grid-cols-7 mb-1">
            {DAYS.map((d) => (
              <div
                key={d}
                className="flex h-7 items-center justify-center text-[10px] font-medium text-[var(--muted-foreground)]"
              >
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

          {/* Time row */}
          <div className="mt-3 border-t border-[var(--border)] pt-3">
            <div className="flex items-center gap-2">
              <Clock size={14} className="text-[var(--muted-foreground)]" />
              <div className="flex items-center gap-1 flex-1">
                <select
                  value={hours}
                  onChange={(e) => changeTime(Number(e.target.value), minutes)}
                  className="h-8 rounded-md border border-[var(--border)] bg-[var(--card)] px-2 text-[12px] font-medium text-[var(--foreground)] focus:border-[var(--brand)] focus:outline-none"
                >
                  {Array.from({ length: 24 }).map((_, h) => (
                    <option key={h} value={h}>
                      {pad2(h)}
                    </option>
                  ))}
                </select>
                <span className="text-[12px] font-semibold text-[var(--muted-foreground)]">
                  :
                </span>
                <select
                  value={minutes}
                  onChange={(e) => changeTime(hours, Number(e.target.value))}
                  className="h-8 rounded-md border border-[var(--border)] bg-[var(--card)] px-2 text-[12px] font-medium text-[var(--foreground)] focus:border-[var(--brand)] focus:outline-none"
                >
                  {/* 5-min granularity for compact list, sufficient for exam scheduling */}
                  {Array.from({ length: 12 }).map((_, i) => {
                    const m = i * 5;
                    return (
                      <option key={m} value={m}>
                        {pad2(m)}
                      </option>
                    );
                  })}
                </select>
              </div>
              <button
                type="button"
                onClick={() => {
                  const now = new Date();
                  setViewYear(now.getFullYear());
                  setViewMonth(now.getMonth());
                  emit(
                    now.getFullYear(),
                    now.getMonth(),
                    now.getDate(),
                    now.getHours(),
                    Math.floor(now.getMinutes() / 5) * 5
                  );
                }}
                className="text-[10px] font-medium text-[var(--brand)] hover:underline"
              >
                Now
              </button>
            </div>
          </div>
        </div>
      )}

      {visibleError && (
        <p
          className="mt-1 text-[11px] font-medium text-[var(--danger)]"
          role="alert"
        >
          {visibleError}
        </p>
      )}
      {helperText && !visibleError && (
        <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">
          {helperText}
        </p>
      )}
    </div>
  );
}
