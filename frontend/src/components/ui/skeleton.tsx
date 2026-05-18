import { cn } from "@/lib/cn";

interface SkeletonProps {
  className?: string;
}

export function Skeleton({ className }: SkeletonProps) {
  return (
    <div
      className={cn(
        "animate-pulse rounded-lg bg-[color:var(--surface-subtle)]",
        className
      )}
      aria-hidden="true"
    />
  );
}
