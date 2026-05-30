"use client";

import { useEffect, useState } from "react";
import { createAuditActionPlan, getAgentActionPlanSummary, runNextAgentActionPlanBatch, type AgentActionPlanSummaryResponse } from "@/lib/modules-api";
import { ClipboardList, Loader2, RefreshCcw, Sparkles, TriangleAlert } from "lucide-react";

function formatPercent(value: number) {
  return `${value}%`;
}

function domainIssueEntries(domainSummary: Record<string, unknown>) {
  const issues: string[] = [];
  const keys = ["missingTP", "missingMateri", "missingIndikator", "disconnected", "totalSlots"];
  for (const key of keys) {
    const value = domainSummary[key];
    if (typeof value === "number" && key !== "totalSlots") {
      issues.push(`${key}: ${value}`);
    }
    if (typeof value === "string" && value.trim()) {
      issues.push(value);
    }
  }
  const nested = domainSummary.data;
  if (nested && typeof nested === "object") {
    for (const [key, value] of Object.entries(nested as Record<string, unknown>)) {
      if (key === "examId") continue;
      if (typeof value === "number" && key !== "totalSlots") {
        issues.push(`${key}: ${value}`);
      }
      if (Array.isArray(value)) {
        for (const item of value) {
          if (typeof item === "string" && item.trim()) {
            issues.push(item);
          }
        }
      }
    }
  }
  return issues;
}

export function ExamKisiActionPlanBanner({ examId, onPlanFinished }: { examId: string; onPlanFinished?: () => void }) {
  const [loading, setLoading] = useState(true);
  const [retrying, setRetrying] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<AgentActionPlanSummaryResponse | null>(null);

  const fetchSummary = async () => {
    setLoading(true);
    setError(null);
    const res = await getAgentActionPlanSummary(examId);
    if (res.error) {
      setData({ planId: "", status: "", summary: null, domainSummary: {}, batches: [] } as AgentActionPlanSummaryResponse);
    } else {
      setData(res.data ?? null);
    }
    setLoading(false);
  };

  useEffect(() => {
    if (!examId) return;
    fetchSummary();
  }, [examId]);

  const planStatus = data?.status ?? "";
  const hasPlan = Boolean(data?.planId);
  const isCompleted = planStatus === "completed";
  const isFailed = planStatus === "failed";
  const isActive = planStatus === "active";
  const showBanner = loading || error !== null || hasPlan || isCompleted;
  if (!showBanner) {
    return null;
  }

  const summary = data?.summary;
  const domainSummary = (data?.domainSummary ?? {}) as Record<string, unknown>;
  const issues = domainIssueEntries(domainSummary);
  const nextBatchIndex = summary?.nextBatchIndex ?? 0;

  const handleRunNext = async () => {
    if (!data?.planId) return;
    setRetrying(true);
    const res = await runNextAgentActionPlanBatch(data.planId);
    setRetrying(false);
    if (res.error) {
      setError(res.error.message);
      return;
    }
    await fetchSummary();
    if (planStatus === "completed") {
      onPlanFinished?.();
    }
  };

  const handleCreateAudit = async () => {
    setCreating(true);
    setError(null);
    const res = await createAuditActionPlan(examId);
    setCreating(false);
    if (res.error) {
      setError(res.error.message);
      return;
    }
    await fetchSummary();
  };

  return (
    <div className="rounded-2xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
      {loading ? (
        <div className="flex items-center gap-2 text-[12px] text-[var(--muted-foreground)]">
          <Loader2 size={14} className="animate-spin" />
          Memeriksa rencana eksekusi kisi-kisi...
        </div>
      ) : error ? (
        <div className="flex items-start gap-2 text-[12px] text-[var(--warning)]">
          <TriangleAlert size={14} className="mt-0.5" />
          <div>
            <p className="font-semibold">Gagal membaca action plan</p>
            <p className="mt-1 text-[var(--muted-foreground)]">{error}</p>
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className={`rounded-full px-2.5 py-0.5 text-[10px] font-bold ${isFailed ? "bg-[var(--warning-soft)] text-[var(--warning)]" : isActive ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"}`}>
                {planStatus.toUpperCase()}
              </span>
              <p className="truncate text-[13px] font-semibold text-[var(--foreground)]">
                {summary?.goal || "Rencana eksekusi kisi-kisi"}
              </p>
            </div>
            <div className="mt-2 text-[12px] text-[var(--muted-foreground)]">
              <p>
                Progress {formatPercent(summary?.progressPercent ?? 0)} · batch {summary?.currentBatchIndex ?? 0}/{summary?.totalBatches ?? 0}
                {nextBatchIndex > 0 ? ` · next batch ${nextBatchIndex}` : ""}
              </p>
              {issues.length > 0 && (
                <ul className="mt-2 list-disc space-y-1 pl-4">
                  {issues.slice(0, 6).map((item, idx) => (
                    <li key={idx} className="text-[11px] text-[var(--foreground)]">{item}</li>
                  ))}
                  {issues.length > 6 && <li className="text-[10px] text-[var(--muted-foreground)]">+{issues.length - 6} temuan lain</li>}
                </ul>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={fetchSummary}
              className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--border)] px-3 text-[12px] font-semibold text-[var(--foreground)]"
            >
              <RefreshCcw size={13} />
              Refresh
            </button>
            {isCompleted && (
              <button
                type="button"
                onClick={handleCreateAudit}
                disabled={creating}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] disabled:opacity-50"
              >
                {creating ? <Loader2 size={13} className="animate-spin" /> : <ClipboardList size={13} />}
                Audit ulang semua kisi-kisi
              </button>
            )}
            {(isFailed || isActive) && (
              <button
                type="button"
                onClick={handleRunNext}
                disabled={retrying}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] disabled:opacity-50"
              >
                {retrying ? <Loader2 size={13} className="animate-spin" /> : <Sparkles size={13} />}
                {isFailed ? "Retry batch berikutnya" : "Jalankan batch berikutnya"}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
