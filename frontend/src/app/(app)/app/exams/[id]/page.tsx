"use client";

import { use, useEffect, useState } from "react";
import { useToast } from "@/components/ui/toast";
import {
  getExam,
  listExamGates,
  createExamGate,
  deleteExamGate,
  publishExam,
  updateExamKisiKisi,
  getExamBlueprint,
  type Exam,
  type ExamGate,
} from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { InputField } from "@/components/ui/input-field";
import { Skeleton } from "@/components/ui/skeleton";
import { DateTimePicker } from "@/components/ui/datetime-picker";
import { ShareDialog } from "@/components/share-dialog";
import { ExamCanvas } from "@/components/exams/exam-canvas";
import { InlineMagicPopover } from "@/components/ai/inline-magic-popover";
import { KisiKisiToggle } from "@/components/exams/kisi-kisi-toggle";
import { LoadKisiKisiButton } from "@/components/exams/load-kisi-kisi-button";
import { LoadKisiKisiSheet } from "@/components/exams/load-kisi-kisi-sheet";
import { ExportBlueprintButton } from "@/components/exams/export-blueprint-button";
import { ExportBlueprintSheet } from "@/components/exams/export-blueprint-sheet";
import {
  CalendarClock,
  FileQuestion,
  Plus,
  Send,
  Share2,
  Sparkles,
  Trash2,
} from "lucide-react";
import { cn } from "@/lib/cn";

interface PageProps {
  // Next.js 15: params is a Promise.
  params: Promise<{ id: string }>;
}

export default function ExamDetailPage({ params }: PageProps) {
  const { id: examId } = use(params);
  const { toast } = useToast();

  const [exam, setExam] = useState<Exam | null>(null);
  const [gates, setGates] = useState<ExamGate[]>([]);
  const [loading, setLoading] = useState(true);

  const [hasBlueprint, setHasBlueprint] = useState(false);

  const [showShare, setShowShare] = useState(false);
  const [pendingToggleNotice, setPendingToggleNotice] = useState<string | null>(
    null,
  );
  const [showLoadKK, setShowLoadKK] = useState(false);
  const [showExportKK, setShowExportKK] = useState(false);

  const [showGateForm, setShowGateForm] = useState(false);
  const [gateForm, setGateForm] = useState({
    opensAt: "",
    closesAt: "",
    accessCode: "",
  });

  async function reload() {
    setLoading(true);
    const [examRes, gatesRes, bpRes] = await Promise.all([
      getExam(examId),
      listExamGates(examId),
      getExamBlueprint(examId),
    ]);
    if (examRes.data) setExam(examRes.data);
    if (gatesRes.data) setGates(gatesRes.data.data);
    setHasBlueprint(!!bpRes.data?.blueprint);
    setLoading(false);
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [examId]);

  useEffect(() => {
    function h() {
      reload();
    }
    window.addEventListener("morfoschools:data-changed", h);
    return () =>
      window.removeEventListener("morfoschools:data-changed", h);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [examId]);

  async function handleKisiKisiToggle(enabled: boolean) {
    if (!exam) return;
    const res = await updateExamKisiKisi(exam.id, enabled);
    if (res.error) {
      toast({
        tone: "error",
        title: "Toggle change failed",
        description: res.error.message,
      });
      return;
    }
    toast({
      tone: "success",
      title: `Kisi-kisi: ${enabled ? "On" : "Off"}`,
      description: res.data?.warning || res.data?.hint,
    });
    if (res.data?.warning) {
      setPendingToggleNotice(res.data.warning);
    } else if (res.data?.hint) {
      setPendingToggleNotice(res.data.hint);
    }
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handlePublish() {
    if (!exam) return;
    if (exam.questionCount === 0) {
      toast({
        tone: "error",
        title: "Cannot publish",
        description: "Add at least one question first.",
      });
      return;
    }
    const res = await publishExam(exam.id);
    if (res.error) {
      toast({
        tone: "error",
        title: "Publish failed",
        description: res.error.fields?.coverage || res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Exam published" });
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handleAddGate(e: React.FormEvent) {
    e.preventDefault();
    if (!gateForm.opensAt || !gateForm.closesAt) {
      toast({
        tone: "error",
        title: "Both opens and closes are required",
      });
      return;
    }
    const res = await createExamGate(examId, {
      opensAt: gateForm.opensAt,
      closesAt: gateForm.closesAt,
      accessCode: gateForm.accessCode || undefined,
    });
    if (res.error) {
      toast({
        tone: "error",
        title: "Failed",
        description: res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Gate window added" });
    setGateForm({ opensAt: "", closesAt: "", accessCode: "" });
    setShowGateForm(false);
    reload();
  }

  async function handleDeleteGate(g: ExamGate) {
    const res = await deleteExamGate(g.id);
    if (res.error) {
      toast({
        tone: "error",
        title: "Failed",
        description: res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Gate deleted" });
    reload();
  }

  if (loading) {
    return (
      <div className="space-y-3 p-6">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (!exam) {
    return (
      <div className="p-6">
        <p className="text-[13px] text-[var(--muted-foreground)]">
          Exam not found.
        </p>
      </div>
    );
  }

  return (
    <>
      <PageShell
        title={exam.title}
        subtitle={`${exam.examType} · ${exam.questionCount} question${
          exam.questionCount !== 1 ? "s" : ""
        } · ${exam.totalPoints} pts`}
        back={{ href: "/app/exams", label: "Back to exams" }}
        actions={
          <>
            <InlineMagicPopover
              entityKind="exam"
              entityId={exam.id}
              examId={exam.id}
            />
            {exam.canWrite !== false && (
              <KisiKisiToggle exam={exam} onToggle={handleKisiKisiToggle} />
            )}
            <LoadKisiKisiButton
              visible={exam.usesKisiKisi && exam.canWrite !== false}
              hasBlueprint={hasBlueprint}
              onClick={() => setShowLoadKK(true)}
            />
            <ExportBlueprintButton
              visible={exam.usesKisiKisi && hasBlueprint}
              onClick={() => setShowExportKK(true)}
            />
            {exam.canWrite !== false && exam.status === "draft" && (
              <button
                onClick={handlePublish}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] transition-all"
              >
                <Send size={14} /> Publish
              </button>
            )}
            {exam.canDelete === true && (
              <button
                type="button"
                onClick={() => setShowShare(true)}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-[12px] font-medium text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors"
              >
                <Share2 size={14} /> Collaborator
              </button>
            )}
          </>
        }
      >
        {/* Status + meta strip */}
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "rounded-md px-2 py-0.5 text-[10px] font-medium",
              exam.status === "published"
                ? "bg-[var(--success-soft)] text-[var(--success)]"
                : exam.status === "archived"
                  ? "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  : "bg-[var(--warning-soft)] text-[var(--warning)]",
            )}
          >
            {exam.status}
          </span>
          {exam.usesKisiKisi && (
            <span className="inline-flex items-center gap-1 rounded-md bg-[var(--brand-soft)] px-2 py-0.5 text-[10px] font-semibold text-[var(--brand)]">
              <Sparkles size={10} /> Kurikulum Merdeka · CP/TP
            </span>
          )}
          <span className="text-[11px] text-[var(--muted-foreground)]">
            Max {exam.maxScore} · Pass {exam.passingScore}
            {exam.durationMinutes ? ` · ${exam.durationMinutes} min` : ""}
          </span>
        </div>

        {pendingToggleNotice && (
          <div className="mb-3 flex items-start justify-between gap-2 rounded-lg border border-[var(--warning)] bg-[var(--warning-soft)] px-3 py-2 text-[11px] text-[var(--warning)]">
            <span>{pendingToggleNotice}</span>
            <button
              type="button"
              onClick={() => setPendingToggleNotice(null)}
              className="text-[var(--warning)] hover:opacity-70"
              aria-label="Dismiss"
            >
              ×
            </button>
          </div>
        )}

        <div className="mb-4 rounded-2xl border border-[var(--border)] bg-[var(--card)] shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border)] px-4 py-3">
            <div className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-[var(--brand-soft)] text-[var(--brand)]">
                <FileQuestion size={17} />
              </div>
              <div>
                <h2 className="text-[14px] font-bold text-[var(--foreground)]">
                  Questions Manager
                </h2>
                <p className="text-[11px] text-[var(--muted-foreground)]">
                  Susun section, group stimulus, dan soal ujian dari satu kanvas.
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 text-[10px] font-medium text-[var(--muted-foreground)]">
              <span className="rounded-md bg-[var(--muted)] px-2 py-1">
                {exam.questionCount} soal
              </span>
              <span className="rounded-md bg-[var(--muted)] px-2 py-1">
                {exam.totalPoints} pts
              </span>
            </div>
          </div>
          <div className="p-3 md:p-4">
            <ExamCanvas
          exam={exam}
          onExamChange={reload}
          onBlueprintTypeChange={() => {}}
          onRequestLoadKisiKisi={() => setShowLoadKK(true)}
              onGenerateFromQuestions={() => {
                toast({
                  tone: "info",
                  title: "Buka AI chat",
                  description:
                    "Panggil convert_questions_to_kisi_kisi via chat untuk konfirmasi reverse-flow.",
                });
              }}
            />
          </div>
        </div>

        {/* Schedule (Gate Windows) — kept below canvas */}
        <div className="mt-8 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-[14px] font-semibold text-[var(--foreground)]">
              Schedule (Gate Windows)
            </h2>
            {exam.canWrite !== false && (
              <button
                onClick={() => setShowGateForm(true)}
                className="inline-flex h-7 items-center gap-1 rounded-md bg-[var(--muted)] px-2.5 text-[11px] font-medium text-[var(--foreground)] hover:bg-[var(--border)]"
              >
                <Plus size={12} /> Window
              </button>
            )}
          </div>

          {showGateForm && (
            <form
              onSubmit={handleAddGate}
              className="space-y-2 rounded-xl border border-[var(--border)] bg-[var(--accent)] p-3"
            >
              <DateTimePicker
                label="Opens at"
                value={gateForm.opensAt}
                onChange={(val) =>
                  setGateForm({ ...gateForm, opensAt: val })
                }
              />
              <DateTimePicker
                label="Closes at"
                value={gateForm.closesAt}
                onChange={(val) =>
                  setGateForm({ ...gateForm, closesAt: val })
                }
              />
              <InputField
                label="Access code (optional)"
                value={gateForm.accessCode}
                onChange={(e) =>
                  setGateForm({ ...gateForm, accessCode: e.target.value })
                }
              />
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowGateForm(false)}
                  className="px-2 text-[11px] font-medium text-[var(--muted-foreground)] hover:text-[var(--foreground)]"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="inline-flex h-7 items-center gap-1 rounded-md bg-[var(--primary)] px-2.5 text-[11px] font-semibold text-[var(--primary-foreground)]"
                >
                  Add Window
                </button>
              </div>
            </form>
          )}

          {gates.length === 0 ? (
            <p className="text-[12px] text-[var(--muted-foreground)]">
              No schedule yet. Without a gate window the exam is not takeable.
            </p>
          ) : (
            <div className="divide-y divide-[var(--border)] rounded-xl border border-[var(--border)] bg-[var(--card)]">
              {gates.map((g) => (
                <div
                  key={g.id}
                  className="flex items-center gap-3 px-3 py-2.5"
                >
                  <CalendarClock
                    size={14}
                    className="text-[var(--muted-foreground)]"
                  />
                  <div className="flex-1 min-w-0 text-[12px]">
                    <p className="text-[var(--foreground)]">
                      {new Date(g.opensAt).toLocaleString()} →{" "}
                      {new Date(g.closesAt).toLocaleString()}
                    </p>
                    <p className="text-[10px] text-[var(--muted-foreground)]">
                      {g.accessCode ? `Code: ${g.accessCode} · ` : ""}
                      {g.isOpen ? "Currently open" : "Currently closed"}
                    </p>
                  </div>
                  {exam.canWrite !== false && (
                    <button
                      onClick={() => handleDeleteGate(g)}
                      className="rounded-md p-1.5 text-[var(--muted-foreground)] hover:bg-[var(--destructive-soft)] hover:text-[var(--destructive)] transition-colors"
                    >
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </PageShell>

      <LoadKisiKisiSheet
        open={showLoadKK}
        examId={examId}
        hasBlueprint={hasBlueprint}
        hasQuestions={exam.questionCount > 0}
        onClose={() => setShowLoadKK(false)}
        onApplied={reload}
        onGenerateFromQuestions={() => {
          toast({
            tone: "info",
            title: "Buka AI chat",
            description:
              "Panggil convert_questions_to_kisi_kisi via chat untuk konfirmasi reverse-flow.",
          });
        }}
      />

      <ShareDialog
        open={showShare}
        onClose={() => setShowShare(false)}
        resource="exams"
        resourceId={exam.id}
        resourceName={exam.title}
        currentUserCanManage={exam.canDelete === true}
      />

      <ExportBlueprintSheet
        open={showExportKK}
        examId={examId}
        defaultTitle={exam.title}
        onClose={() => setShowExportKK(false)}
      />
    </>
  );
}
