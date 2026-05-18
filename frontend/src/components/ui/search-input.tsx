"use client";

import { Search } from "lucide-react";
import { cn } from "@/lib/cn";

interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

export function SearchInput({ value, onChange, placeholder = "Search...", className }: SearchInputProps) {
  return (
    <div className={cn("flex h-8 items-center rounded-lg border border-[var(--border)] bg-[var(--background)] px-2.5 gap-2", className)}>
      <Search size={13} className="shrink-0 text-[var(--muted-foreground)]" />
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full bg-transparent text-[12px] outline-none placeholder:text-[var(--muted-foreground)] text-[var(--foreground)]"
      />
    </div>
  );
}
