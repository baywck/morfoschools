"use client";

import { use, useEffect, useId, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-provider";
import { useToast } from "@/components/ui/toast";
import {
  getExam,
  publishExam,
  updateExam,
  updateExamKisiKisi,
  getExamBlueprint,
  getExamCurriculumContext,
  getSlotsWithQuestions,
  listSubjects,
  updateExamBlueprintSlot,
  deleteExamBlueprintSlot,
  type Exam,
  type ExamBlueprint,
  type ExamCurriculumContext,
  type SlotPayload,
  type SlotWithQuestion,
  type SlotsWithQuestionsResponse,
  type Subject,
} from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { InputField } from "@/components/ui/input-field";
import { TextareaField } from "@/components/ui/textarea-field";
import { SelectField } from "@/components/ui/select-field";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import { Skeleton } from "@/components/ui/skeleton";
import { ShareDialog } from "@/components/share-dialog";
import { ExamCanvas } from "@/components/exams/exam-canvas";
import { InlineMagicPopover } from "@/components/ai/inline-magic-popover";
import { LoadKisiKisiSheet } from "@/components/exams/load-kisi-kisi-sheet";
import { ExportBlueprintSheet } from "@/components/exams/export-blueprint-sheet";
import { RenderedContent } from "@/components/ui/rendered-content";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { RowActions } from "@/components/ui/row-actions";
import {
  ClipboardCopy,
  ClipboardList,
  ClipboardPaste,
  FileQuestion,
  Info,
  Pencil,
  Printer,
  Save,
  Send,
  Settings2,
  Share2,
  ShieldCheck,
  Sparkles,
  Trash2,
} from "lucide-react";
import { cn } from "@/lib/cn";
import { questionTypeLabel } from "@/lib/question-labels";
import { tenantEnabledPhases } from "@/lib/tenant-education";
import { gradeOptionsForPhases, phaseForGrade } from "@/lib/grade-options";
import { MerdekaKisiKisiFields } from "@/components/blueprint/merdeka-kisi-kisi-fields";

interface PageProps {
  params: Promise<{ id: string }>;
}

type WorkspaceTab = "setup" | "questions" | "kisi-kisi";

const examTypeOptions = [
  { value: "quiz", label: "Quiz" },
  { value: "midterm", label: "Midterm" },
  { value: "final", label: "Final" },
  { value: "tryout", label: "Tryout" },
  { value: "daily", label: "Daily" },
];

function normalizeTab(raw: string | undefined, _usesKisiKisi: boolean): WorkspaceTab {
  if (raw === "setup" || raw === "questions" || raw === "kisi-kisi") return raw;
  return "setup";
}

function tabFromPath(pathname: string): WorkspaceTab | undefined {
  const segment = pathname.split("/").filter(Boolean).at(-1);
  if (segment === "setup" || segment === "questions" || segment === "kisi-kisi") return segment;
  return undefined;
}

function statusTone(status: string) {
  if (status === "published") return "bg-[var(--success-soft)] text-[var(--success)]";
  if (status === "archived") return "bg-[var(--muted)] text-[var(--muted-foreground)]";
  return "bg-[var(--warning-soft)] text-[var(--warning)]";
}

export default function ExamDetailPage({ params }: PageProps) {
  const { id: examId } = use(params);
  const router = useRouter();
  const pathname = usePathname();
  const { session } = useAuth();
  const { toast } = useToast();

  const gradeOptions = useMemo(
    () => gradeOptionsForPhases(tenantEnabledPhases(session?.effectiveTenant)),
    [session?.effectiveTenant],
  );

  const [exam, setExam] = useState<Exam | null>(null);
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [blueprint, setBlueprint] = useState<ExamBlueprint | null>(null);
  const [slotsData, setSlotsData] = useState<SlotsWithQuestionsResponse | null>(null);
  const [curriculumContext, setCurriculumContext] = useState<ExamCurriculumContext | null>(null);
  const [loading, setLoading] = useState(true);

  const [showShare, setShowShare] = useState(false);
  const [showLoadKK, setShowLoadKK] = useState(false);
  const [showExportKK, setShowExportKK] = useState(false);
  const [pendingToggleNotice, setPendingToggleNotice] = useState<string | null>(null);

  const [savingBasic, setSavingBasic] = useState(false);
  const [savingBehavior, setSavingBehavior] = useState(false);
  const [togglingKisi, setTogglingKisi] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const [basicForm, setBasicForm] = useState({
    title: "",
    description: "",
    subjectId: "",
    gradeLevel: "",
    examType: "quiz",
    durationMinutes: "",
    maxScore: "100",
    passingScore: "70",
  });
  const [behaviorForm, setBehaviorForm] = useState({
    shuffleQuestions: false,
    shuffleOptions: false,
    showResultImmediately: false,
  });

  const activeTab = normalizeTab(tabFromPath(pathname), exam?.usesKisiKisi ?? false);
  const canWrite = exam?.canWrite !== false;
  const hasSubject = !!exam?.subjectId;
  const visibleTab = !hasSubject && activeTab === "kisi-kisi" ? "setup" : activeTab;

  function setTab(tab: WorkspaceTab) {
    router.replace(`/app/exams/${examId}/${tab}`, { scroll: false });
  }

  function syncForms(next: Exam) {
    setBasicForm({
      title: next.title,
      description: next.description ?? "",
      subjectId: next.subjectId ?? "",
      gradeLevel: next.gradeLevel ?? "",
      examType: next.examType,
      durationMinutes: next.durationMinutes != null ? String(next.durationMinutes) : "",
      maxScore: String(next.maxScore),
      passingScore: String(next.passingScore),
    });
    setBehaviorForm({
      shuffleQuestions: next.shuffleQuestions,
      shuffleOptions: next.shuffleOptions,
      showResultImmediately: next.showResultImmediately,
    });
  }

  async function reload() {
    setLoading(true);
    const [examRes, bpRes, subjectsRes] = await Promise.all([
      getExam(examId),
      getExamBlueprint(examId),
      listSubjects({ status: "active" }),
    ]);
    if (examRes.data) {
      setExam(examRes.data);
      syncForms(examRes.data);
      if (examRes.data.usesKisiKisi) {
        const [slotsRes, curriculumRes] = await Promise.all([
          getSlotsWithQuestions(examId),
          getExamCurriculumContext(examId),
        ]);
        setSlotsData(slotsRes.data ?? null);
        setCurriculumContext(curriculumRes.data ?? null);
      } else {
        setSlotsData(null);
        setCurriculumContext(null);
      }
    }
    if (bpRes.data) setBlueprint(bpRes.data.blueprint);
    if (subjectsRes.data) setSubjects(subjectsRes.data.data);
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
    return () => window.removeEventListener("morfoschools:data-changed", h);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [examId]);

  async function handleSaveBasic(e: React.FormEvent) {
    e.preventDefault();
    if (!exam) return;
    setSavingBasic(true);
    setFieldErrors({});
    const res = await updateExam(exam.id, {
      title: basicForm.title,
      description: basicForm.description,
      subjectId: basicForm.subjectId || undefined,
      gradeLevel: basicForm.gradeLevel || undefined,
      examType: basicForm.examType,
      durationMinutes: basicForm.durationMinutes ? Number(basicForm.durationMinutes) : undefined,
      maxScore: basicForm.maxScore ? Number(basicForm.maxScore) : undefined,
      passingScore: basicForm.passingScore ? Number(basicForm.passingScore) : undefined,
    });
    setSavingBasic(false);
    if (res.error) {
      setFieldErrors(res.error.fields ?? {});
      toast({ tone: "error", title: "Setup gagal disimpan", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Setup ujian disimpan" });
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handleSaveBehavior(e: React.FormEvent) {
    e.preventDefault();
    if (!exam) return;
    setSavingBehavior(true);
    const res = await updateExam(exam.id, behaviorForm);
    setSavingBehavior(false);
    if (res.error) {
      toast({ tone: "error", title: "Behavior gagal disimpan", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Behavior ujian disimpan" });
    reload();
  }

  async function handleKisiKisiToggle(enabled: boolean) {
    if (!exam) return;
    setTogglingKisi(true);
    const res = await updateExamKisiKisi(exam.id, enabled);
    setTogglingKisi(false);
    if (res.error) {
      toast({ tone: "error", title: "Toggle change failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: `Kisi-kisi: ${enabled ? "On" : "Off"}`, description: res.data?.warning || res.data?.hint });
    setPendingToggleNotice(res.data?.warning || res.data?.hint || null);
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handlePublish() {
    if (!exam) return;
    if (exam.questionCount === 0) {
      toast({ tone: "error", title: "Cannot publish", description: "Add at least one question first." });
      return;
    }
    const res = await publishExam(exam.id);
    if (res.error) {
      toast({ tone: "error", title: "Publish failed", description: res.error.fields?.coverage || res.error.message });
      return;
    }
    toast({ tone: "success", title: "Exam published" });
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
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
    return <div className="p-6"><p className="text-[13px] text-[var(--muted-foreground)]">Exam not found.</p></div>;
  }

  return (
    <>
      <PageShell
        title={exam.title}
        subtitle={`${exam.examType} · ${exam.questionCount} question${exam.questionCount !== 1 ? "s" : ""} · ${exam.totalPoints} pts`}
        back={{ href: "/app/exams", label: "Back to exams" }}
        actions={
          <>
            <InlineMagicPopover entityKind="exam" entityId={exam.id} examId={exam.id} />
            {canWrite && exam.status === "draft" && (
              <button onClick={handlePublish} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm transition-all hover:opacity-90 active:scale-[0.97]">
                <Send size={14} /> Publish
              </button>
            )}
            {exam.canDelete === true && (
              <button type="button" onClick={() => setShowShare(true)} className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-[12px] font-medium text-[var(--foreground)] transition-colors hover:bg-[var(--muted)]">
                <Share2 size={14} /> Collaborator
              </button>
            )}
          </>
        }
      >
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", statusTone(exam.status))}>{exam.status}</span>
          {exam.usesKisiKisi && <span className="inline-flex items-center gap-1 rounded-md bg-[var(--brand-soft)] px-2 py-0.5 text-[10px] font-semibold text-[var(--brand)]"><Sparkles size={10} /> Kisi-kisi aktif</span>}
          <span className="text-[11px] text-[var(--muted-foreground)]">Max {exam.maxScore} · Pass {exam.passingScore}{exam.durationMinutes ? ` · ${exam.durationMinutes} min` : ""}</span>
        </div>

        {pendingToggleNotice && (
          <div className="mb-3 flex items-start justify-between gap-2 rounded-lg border border-[var(--warning)] bg-[var(--warning-soft)] px-3 py-2 text-[11px] text-[var(--warning)]">
            <span>{pendingToggleNotice}</span>
            <button type="button" onClick={() => setPendingToggleNotice(null)} className="text-[var(--warning)] hover:opacity-70" aria-label="Dismiss">×</button>
          </div>
        )}

        <div className="rounded-2xl border border-[var(--border)] bg-[var(--card)] shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border)] px-4 py-3">
            <div className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-[var(--brand-soft)] text-[var(--brand)]"><Settings2 size={17} /></div>
              <div>
                <h2 className="text-[14px] font-bold text-[var(--foreground)]">Authoring Workspace</h2>
                <p className="text-[11px] text-[var(--muted-foreground)]">Atur setup{hasSubject ? ", kisi-kisi," : " dan"} questions dari satu workspace.</p>
              </div>
            </div>
            <div className="flex items-center gap-2 text-[10px] font-medium text-[var(--muted-foreground)]">
              <span className="rounded-md bg-[var(--muted)] px-2 py-1">{exam.questionCount} soal</span>
              <span className="rounded-md bg-[var(--muted)] px-2 py-1">{exam.totalPoints} pts</span>
            </div>
          </div>

          <div className="border-b border-[var(--border)] px-3 py-3">
            <div className="flex items-center gap-1 rounded-xl bg-[var(--muted)] p-1">
              <WorkspaceTabButton active={visibleTab === "setup"} icon={<Settings2 size={14} />} label="Setup" onClick={() => setTab("setup")} />
              {hasSubject && <WorkspaceTabButton active={visibleTab === "kisi-kisi"} icon={<ClipboardList size={14} />} label="Kisi-Kisi" onClick={() => setTab("kisi-kisi")} />}
              <WorkspaceTabButton active={visibleTab === "questions"} icon={<FileQuestion size={14} />} label="Questions" onClick={() => setTab("questions")} />
            </div>
          </div>

          <div className="p-3 md:p-4">
            {visibleTab === "setup" && (
              <SetupPanel
                exam={exam}
                subjects={subjects}
                gradeOptions={gradeOptions}
                basicForm={basicForm}
                setBasicForm={setBasicForm}
                behaviorForm={behaviorForm}
                setBehaviorForm={setBehaviorForm}
                fieldErrors={fieldErrors}
                canWrite={canWrite}
                savingBasic={savingBasic}
                savingBehavior={savingBehavior}
                onSaveBasic={handleSaveBasic}
                onSaveBehavior={handleSaveBehavior}
              />
            )}
            {visibleTab === "questions" && (
              <ExamCanvas
                exam={exam}
                onExamChange={reload}
                onBlueprintTypeChange={() => {}}
                onRequestLoadKisiKisi={() => setShowLoadKK(true)}
                onGenerateFromQuestions={() => toast({ tone: "info", title: "Buka AI chat", description: "Panggil convert_questions_to_kisi_kisi via chat untuk konfirmasi reverse-flow." })}
              />
            )}
            {visibleTab === "kisi-kisi" && (
              <KisiKisiManagerPanel
                exam={exam}
                blueprint={blueprint}
                slotsData={slotsData}
                curriculumContext={curriculumContext}
                canWrite={canWrite}
                togglingKisi={togglingKisi}
                onToggleKisiKisi={handleKisiKisiToggle}
                onLoadTemplate={() => setShowLoadKK(true)}
                onExportTemplate={() => setShowExportKK(true)}
                onPrint={() => window.print()}
                onGenerateFromQuestions={() => toast({ tone: "info", title: "Buka AI chat", description: "Reverse-flow kisi-kisi tetap lewat proposal/confirm AI chat." })}
                onChanged={reload}
              />
            )}
          </div>
        </div>
      </PageShell>

      <LoadKisiKisiSheet open={showLoadKK} examId={examId} hasBlueprint={!!blueprint} hasQuestions={exam.questionCount > 0} onClose={() => setShowLoadKK(false)} onApplied={reload} onGenerateFromQuestions={() => toast({ tone: "info", title: "Buka AI chat", description: "Panggil convert_questions_to_kisi_kisi via chat untuk konfirmasi reverse-flow." })} />
      <ShareDialog open={showShare} onClose={() => setShowShare(false)} resource="exams" resourceId={exam.id} resourceName={exam.title} currentUserCanManage={exam.canDelete === true} />
      <ExportBlueprintSheet open={showExportKK} examId={examId} defaultTitle={exam.title} onClose={() => setShowExportKK(false)} />
    </>
  );
}

function WorkspaceTabButton({ active, disabled, icon, label, onClick }: { active: boolean; disabled?: boolean; icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "inline-flex h-9 flex-1 items-center justify-center gap-2 rounded-lg text-[12px] font-semibold transition-all",
        active ? "bg-[var(--card)] text-[var(--foreground)] shadow-sm" : "text-[var(--muted-foreground)] hover:bg-[var(--card)]/70 hover:text-[var(--foreground)]",
        disabled && "cursor-not-allowed opacity-45 hover:bg-transparent hover:text-[var(--muted-foreground)]",
      )}
    >
      {icon}{label}
    </button>
  );
}

function SetupPanel(props: {
  exam: Exam;
  subjects: Subject[];
  gradeOptions: string[];
  basicForm: any;
  setBasicForm: (v: any) => void;
  behaviorForm: any;
  setBehaviorForm: (v: any) => void;
  fieldErrors: Record<string, string>;
  canWrite: boolean;
  savingBasic: boolean;
  savingBehavior: boolean;
  onSaveBasic: (e: React.FormEvent) => void;
  onSaveBehavior: (e: React.FormEvent) => void;
}) {
  const p = props;
  return (
    <div className="space-y-4">
      <form onSubmit={p.onSaveBasic} className="rounded-xl border border-[var(--border)] bg-[var(--accent)] p-3">
        <PanelHeader icon={<Settings2 size={15} />} title="Basic setup" subtitle="Nama, detail, mapel, kelas, durasi, dan scoring utama." />
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          <InputField label="Nama ujian" value={p.basicForm.title} disabled={!p.canWrite} error={p.fieldErrors.title} onChange={(e) => p.setBasicForm({ ...p.basicForm, title: e.target.value })} />
          <SelectField label="Exam type" value={p.basicForm.examType} disabled={!p.canWrite} options={examTypeOptions} onChange={(v) => p.setBasicForm({ ...p.basicForm, examType: v })} />
          <SelectField label="Subject" value={p.basicForm.subjectId} disabled error={p.fieldErrors.subjectId} helperText="Subject dikunci setelah exam dibuat." options={[{ value: "", label: "None" }, ...p.subjects.map((s) => ({ value: s.id, label: s.name }))]} onChange={() => {}} />
          <SelectField label="Grade" value={p.basicForm.gradeLevel} disabled error={p.fieldErrors.gradeLevel} helperText="Grade dikunci setelah exam dibuat." options={[{ value: "", label: "None" }, ...p.gradeOptions.map((grade) => ({ value: grade, label: `Kelas ${grade} · Fase ${(phaseForGrade(grade) || "?").toUpperCase()}` }))]} onChange={() => {}} />
          <InputField label="Duration minutes" inputMode="numeric" value={p.basicForm.durationMinutes} disabled={!p.canWrite} error={p.fieldErrors.durationMinutes} onChange={(e) => p.setBasicForm({ ...p.basicForm, durationMinutes: e.target.value.replace(/\D/g, "").slice(0, 4) })} />
          <div className="grid grid-cols-2 gap-3">
            <InputField label="Max score" inputMode="numeric" value={p.basicForm.maxScore} disabled={!p.canWrite} error={p.fieldErrors.maxScore} onChange={(e) => p.setBasicForm({ ...p.basicForm, maxScore: e.target.value })} />
            <InputField label="Passing" inputMode="numeric" value={p.basicForm.passingScore} disabled={!p.canWrite} error={p.fieldErrors.passingScore} onChange={(e) => p.setBasicForm({ ...p.basicForm, passingScore: e.target.value })} />
          </div>
          <div className="md:col-span-2">
            <TextareaField label="Deskripsi" rows={3} value={p.basicForm.description} disabled={!p.canWrite} onChange={(e) => p.setBasicForm({ ...p.basicForm, description: e.target.value })} />
          </div>
        </div>
        <SaveRow disabled={!p.canWrite || p.savingBasic} saving={p.savingBasic} label="Save basic setup" />
      </form>

      <form onSubmit={p.onSaveBehavior} className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
        <PanelHeader icon={<ShieldCheck size={15} />} title="Exam behavior" subtitle="Atur randomisasi dan apa yang dilihat siswa setelah submit." />
        <div className="mt-3 grid gap-2 md:grid-cols-3">
          <SettingToggle title="Random questions" desc="Acak urutan soal saat ujian." checked={p.behaviorForm.shuffleQuestions} disabled={!p.canWrite} onChange={(v) => p.setBehaviorForm({ ...p.behaviorForm, shuffleQuestions: v })} />
          <SettingToggle title="Random answers" desc="Acak opsi jawaban pilihan ganda." checked={p.behaviorForm.shuffleOptions} disabled={!p.canWrite} onChange={(v) => p.setBehaviorForm({ ...p.behaviorForm, shuffleOptions: v })} />
          <SettingToggle title="Show result" desc="Tampilkan hasil segera setelah submit." checked={p.behaviorForm.showResultImmediately} disabled={!p.canWrite} onChange={(v) => p.setBehaviorForm({ ...p.behaviorForm, showResultImmediately: v })} />
        </div>
        <SaveRow disabled={!p.canWrite || p.savingBehavior} saving={p.savingBehavior} label="Save behavior" />
      </form>

    </div>
  );
}

function PanelHeader({ icon, title, subtitle }: { icon: React.ReactNode; title: string; subtitle: string }) {
  return <div className="flex items-start gap-2.5"><div className="mt-0.5 flex h-7 w-7 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">{icon}</div><div><h3 className="text-[13px] font-bold text-[var(--foreground)]">{title}</h3><p className="text-[11px] text-[var(--muted-foreground)]">{subtitle}</p></div></div>;
}

function SaveRow({ disabled, saving, label }: { disabled: boolean; saving: boolean; label: string }) {
  return <div className="mt-3 flex justify-end"><button type="submit" disabled={disabled} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm transition-all hover:opacity-90 active:scale-[0.97] disabled:opacity-50">{saving ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : <Save size={14} />}{label}</button></div>;
}

function SettingToggle({ title, desc, checked, disabled, onChange }: { title: string; desc: string; checked: boolean; disabled: boolean; onChange: (v: boolean) => void }) {
  return <div className="flex items-center justify-between gap-3 rounded-lg border border-[var(--border)] bg-[var(--accent)] px-3 py-2"><div><p className="text-[12px] font-semibold text-[var(--foreground)]">{title}</p><p className="text-[10px] text-[var(--muted-foreground)]">{desc}</p></div><ToggleSwitch checked={checked} disabled={disabled} onChange={onChange} ariaLabel={title} /></div>;
}

function KisiKisiManagerPanel({
  exam,
  blueprint,
  slotsData,
  curriculumContext,
  canWrite,
  togglingKisi,
  onToggleKisiKisi,
  onLoadTemplate,
  onExportTemplate,
  onPrint,
  onGenerateFromQuestions,
  onChanged,
}: {
  exam: Exam;
  blueprint: ExamBlueprint | null;
  slotsData: SlotsWithQuestionsResponse | null;
  curriculumContext: ExamCurriculumContext | null;
  canWrite: boolean;
  togglingKisi: boolean;
  onToggleKisiKisi: (enabled: boolean) => void;
  onLoadTemplate: () => void;
  onExportTemplate: () => void;
  onPrint: () => void;
  onGenerateFromQuestions: () => void;
  onChanged: () => Promise<void>;
}) {
  const { toast } = useToast();
  const [editingSlot, setEditingSlot] = useState<SlotWithQuestion | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SlotWithQuestion | null>(null);
  const [savingSlot, setSavingSlot] = useState(false);
  const [deletingSlot, setDeletingSlot] = useState(false);
  const [slotErrors, setSlotErrors] = useState<Record<string, string>>({});
  const [slotForm, setSlotForm] = useState<SlotPayload>({});
  const filled = slotsData?.coverage.filled ?? blueprint?.filledSlots ?? 0;
  const total = slotsData?.coverage.total ?? blueprint?.totalSlots ?? 0;
  const unlinked = slotsData?.unlinked ?? [];
  const slots = slotsData?.slots ?? [];

  function openEditSlot(slot: SlotWithQuestion) {
    setEditingSlot(slot);
    setSlotErrors({});
    setSlotForm({
      capaianPembelajaran: slot.capaianPembelajaran ?? "",
      elemenCp: slot.elemenCp ?? "",
      tujuanPembelajaran: slot.tujuanPembelajaran ?? "",
      materiPokok: slot.materiPokok ?? "",
      kelas: slot.kelas ?? exam.gradeLevel ?? "",
      semester: slot.semester ?? "",
      cognitiveLevel: slot.cognitiveLevel ?? "",
      difficulty: slot.difficulty ?? "",
      indikatorSoal: slot.indikatorSoal ?? "",
      questionType: slot.questionType ?? "multiple_choice",
      points: slot.points ?? 1,
    });
  }

  async function saveSlotEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editingSlot) return;
    setSavingSlot(true);
    setSlotErrors({});
    try {
      const res = await updateExamBlueprintSlot(editingSlot.id, slotForm);
      if (res.error) {
        setSlotErrors(res.error.fields ?? {});
        toast({ tone: "error", title: "Gagal menyimpan kisi-kisi", description: res.error.message });
        return;
      }
      toast({ tone: "success", title: "Kisi-kisi diperbarui" });
      setEditingSlot(null);
      await onChanged();
    } finally {
      setSavingSlot(false);
    }
  }

  async function confirmDeleteSlot() {
    if (!deleteTarget) return;
    setDeletingSlot(true);
    try {
      const res = await deleteExamBlueprintSlot(deleteTarget.id);
      if (res.error) {
        toast({ tone: "error", title: "Gagal menghapus kisi-kisi", description: res.error.message });
        return;
      }
      toast({ tone: "success", title: "Slot kisi-kisi dihapus", description: deleteTarget.question ? "Soal terkait dilepas dari slot ini, bukan dihapus." : undefined });
      setDeleteTarget(null);
      await onChanged();
    } finally {
      setDeletingSlot(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
        <PanelHeader icon={<Sparkles size={15} />} title="Kisi-kisi" subtitle="Aktifkan blueprint agar soal bisa dipetakan ke indikator dan coverage." />
        {exam.usesKisiKisi && <CurriculumContextNotice context={curriculumContext} />}
        <div className="mt-3 flex items-center justify-between rounded-lg border border-[var(--border)] bg-[var(--accent)] px-3 py-2">
          <div><p className="text-[12px] font-semibold text-[var(--foreground)]">Use Kisi-Kisi / Blueprint</p><p className="text-[11px] text-[var(--muted-foreground)]">Jika dimatikan, data lama tidak dihapus; hanya disembunyikan dari authoring.</p></div>
          <div className="flex items-center gap-2">
            {togglingKisi && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-[var(--muted-foreground)] border-r-transparent" />}
            <ToggleSwitch checked={exam.usesKisiKisi} disabled={!canWrite || togglingKisi} onChange={onToggleKisiKisi} ariaLabel="Toggle kisi-kisi" />
          </div>
        </div>
      </div>

      {!exam.usesKisiKisi ? (
        <div className="rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-8 text-center">
          <ClipboardList size={24} className="mx-auto mb-2 text-[var(--muted-foreground)]" />
          <p className="text-[13px] font-bold text-[var(--foreground)]">Kisi-kisi belum aktif</p>
          <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">Aktifkan toggle di atas untuk mulai mengelola blueprint exam.</p>
        </div>
      ) : !blueprint ? (
        <div className="rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-8 text-center"><Sparkles size={24} className="mx-auto mb-2 text-[var(--muted-foreground)]" /><p className="text-[13px] font-bold text-[var(--foreground)]">Kisi-kisi aktif, tapi belum ada blueprint</p><p className="mt-1 text-[11px] text-[var(--muted-foreground)]">Load template atau gunakan AI reverse-flow dari soal yang sudah ada.</p>{canWrite && <div className="mt-4 flex justify-center gap-2"><button type="button" onClick={onLoadTemplate} className="h-8 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)]">Load Template</button><button type="button" onClick={onGenerateFromQuestions} className="h-8 rounded-lg border border-[var(--border)] px-3 text-[12px] font-semibold text-[var(--foreground)]">Generate dari soal</button></div>}</div>
      ) : (
        <>
          <div className="grid gap-3 md:grid-cols-4">
            <SummaryCard label="Slot kisi-kisi" value={String(total)} />
            <SummaryCard label="Soal terhubung" value={`${filled}/${exam.questionCount}`} />
            <SummaryCard label="Belum terhubung" value={String(unlinked.length)} tone={unlinked.length > 0 ? "warning" : "success"} />
            <SummaryCard label="Coverage slot" value={total > 0 ? `${Math.round((filled / total) * 100)}%` : "0%"} />
          </div>
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <PanelHeader icon={<ClipboardList size={15} />} title={blueprint.title || "Kisi-Kisi Manager"} subtitle={`${blueprint.curriculumCode} · ${blueprint.blueprintType} · ${total} slot`} />
              <div className="hidden flex-wrap items-center gap-2 md:flex">
                {canWrite && <KisiActionButton icon={<ClipboardPaste size={13} />} label={blueprint ? "Ganti template kisi-kisi" : "Import template kisi-kisi"} onClick={onLoadTemplate} />}
                {blueprint && <KisiActionButton icon={<ClipboardCopy size={13} />} label="Simpan sebagai template" onClick={onExportTemplate} />}
                {blueprint && <KisiActionButton icon={<Printer size={13} />} label="Print kisi-kisi" onClick={onPrint} />}
              </div>
              <div className="md:hidden">
                <RowActions actions={[
                  ...(canWrite ? [{ label: blueprint ? "Ganti template" : "Import template", icon: <ClipboardPaste size={13} />, onClick: onLoadTemplate }] : []),
                  ...(blueprint ? [
                    { label: "Simpan template", icon: <ClipboardCopy size={13} />, onClick: onExportTemplate },
                    { label: "Print kisi-kisi", icon: <Printer size={13} />, onClick: onPrint },
                  ] : []),
                ]} />
              </div>
            </div>
          </div>
          <div className="hidden rounded-xl border border-[var(--border)] bg-[var(--card)] print:block print:border-black print:shadow-none md:block">
            <div className="overflow-x-auto md:overflow-x-visible">
              <table className="w-full table-fixed border-collapse text-left text-[11px]">
                <thead className="bg-[var(--muted)] text-[10px] uppercase tracking-wide text-[var(--muted-foreground)]">
                  <tr>
                    <th className="w-[6%] border-b border-[var(--border)] px-2 py-2 text-center">Soal</th>
                    <th className="w-[20%] border-b border-[var(--border)] px-2 py-2">CP / Elemen</th>
                    <th className="w-[20%] border-b border-[var(--border)] px-2 py-2">Tujuan Pembelajaran</th>
                    <th className="w-[12%] border-b border-[var(--border)] px-2 py-2">Materi</th>
                    <th className="w-[25%] border-b border-[var(--border)] px-2 py-2">Indikator Soal</th>
                    <th className="w-[11%] border-b border-[var(--border)] px-2 py-2">Bentuk / Level</th>
                    {canWrite && <th className="w-[6%] border-b border-[var(--border)] px-2 py-2 text-right">Aksi</th>}
                  </tr>
                </thead>
                <tbody>
                  {slots.length === 0 ? (
                    <tr><td colSpan={canWrite ? 7 : 6} className="px-3 py-8 text-center text-[12px] text-[var(--muted-foreground)]">Belum ada slot kisi-kisi.</td></tr>
                  ) : slots.map((slot) => (
                    <tr key={slot.id} className="align-top odd:bg-[var(--background)] even:bg-[var(--accent)]/40">
                      <td className="border-t border-[var(--border)] px-2 py-2 text-center">
                        {slot.question ? <QuestionNumberBadge questionNumber={Math.max(1, (slot.question.sortOrder ?? 0) + 1)} content={slot.question.content} /> : <span className="rounded-md bg-[var(--warning-soft)] px-2 py-0.5 text-[10px] font-semibold text-[var(--warning)]">-</span>}
                      </td>
                      <td className="break-words border-t border-[var(--border)] px-2 py-2"><p className="font-semibold text-[var(--foreground)]">{slot.elemenCp || "-"}</p><p className="mt-1 line-clamp-3 text-[var(--muted-foreground)]">{slot.capaianPembelajaran || "-"}</p></td>
                      <td className="break-words border-t border-[var(--border)] px-2 py-2 text-[var(--foreground)]">{slot.tujuanPembelajaran || "-"}</td>
                      <td className="break-words border-t border-[var(--border)] px-2 py-2 text-[var(--foreground)]">{slot.materiPokok || "-"}</td>
                      <td className="break-words border-t border-[var(--border)] px-2 py-2 text-[var(--foreground)]">{slot.indikatorSoal || "-"}</td>
                      <td className="border-t border-[var(--border)] px-2 py-2 text-[var(--muted-foreground)]">
                        <p className="font-medium text-[var(--foreground)]">{questionTypeLabel(slot.questionType)}</p>
                        <p className="mt-1 text-[10px] font-semibold text-[var(--muted-foreground)]">{slot.cognitiveLevel || "-"}</p>
                      </td>
                      {canWrite && (
                        <td className="border-t border-[var(--border)] px-2 py-2 text-right">
                          <div className="flex justify-end">
                            <RowActions actions={[
                              { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEditSlot(slot) },
                              { label: "Hapus", icon: <Trash2 size={13} />, variant: "danger", onClick: () => setDeleteTarget(slot) },
                            ]} />
                          </div>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          <div className="space-y-3 md:hidden">
            {slots.length === 0 ? (
              <div className="rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-6 text-center text-[12px] text-[var(--muted-foreground)]">Belum ada slot kisi-kisi.</div>
            ) : slots.map((slot) => (
              <div key={slot.id} className="rounded-2xl border border-[var(--border)] bg-[var(--card)] p-3 shadow-sm">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      {slot.question ? <QuestionNumberBadge questionNumber={Math.max(1, (slot.question.sortOrder ?? 0) + 1)} content={slot.question.content} /> : <span className="rounded-md bg-[var(--warning-soft)] px-2 py-0.5 text-[10px] font-semibold text-[var(--warning)]">Belum ada soal</span>}
                      <span className="rounded-md bg-[var(--muted)] px-2 py-0.5 text-[10px] font-semibold text-[var(--muted-foreground)]">{slot.cognitiveLevel || "-"}</span>
                      <span className="rounded-md bg-[var(--brand-soft)] px-2 py-0.5 text-[10px] font-semibold text-[var(--brand)]">{questionTypeLabel(slot.questionType)}</span>
                    </div>
                    <p className="mt-2 text-[13px] font-bold text-[var(--foreground)]">{slot.elemenCp || "Elemen belum diisi"}</p>
                    <p className="mt-1 line-clamp-2 text-[11px] leading-relaxed text-[var(--muted-foreground)]">{slot.materiPokok || "Materi belum diisi"}</p>
                  </div>
                  {canWrite && <RowActions actions={[{ label: "Edit", icon: <Pencil size={13} />, onClick: () => openEditSlot(slot) }, { label: "Hapus", icon: <Trash2 size={13} />, variant: "danger", onClick: () => setDeleteTarget(slot) }]} />}
                </div>
                <div className="mt-3 space-y-2 border-t border-[var(--border)] pt-3">
                  <MobileKisiField label="TP" value={slot.tujuanPembelajaran} />
                  <MobileKisiField label="Indikator" value={slot.indikatorSoal} />
                  <MobileKisiField label="CP" value={slot.capaianPembelajaran} clamp />
                </div>
              </div>
            ))}
          </div>
          <div className="space-y-2"><h3 className="text-[12px] font-bold uppercase tracking-wide text-[var(--muted-foreground)]">Soal belum terhubung</h3>{unlinked.length === 0 ? <p className="rounded-xl border border-[var(--border)] bg-[var(--success-soft)] p-4 text-[12px] font-medium text-[var(--success)]">Semua soal sudah terhubung ke kisi-kisi.</p> : unlinked.map((q) => <div key={q.id} className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3"><div className="flex items-center justify-between gap-2"><p className="text-[11px] font-semibold text-[var(--muted-foreground)]">{questionTypeLabel(q.questionType)} · {q.points} pts</p><span className="rounded-md bg-[var(--warning-soft)] px-2 py-0.5 text-[10px] font-semibold text-[var(--warning)]">unlinked</span></div><div className="mt-2"><RenderedContent html={q.content} className="text-[12px]" /></div></div>)}</div>
          <RightPullSheet
            open={!!editingSlot}
            title="Edit slot kisi-kisi"
            width="lg"
            onClose={() => setEditingSlot(null)}
            footer={
              <div className="flex justify-end gap-2">
                <button type="button" disabled={savingSlot} onClick={() => setEditingSlot(null)} className="h-8 rounded-lg border border-[var(--border)] px-3 text-[12px] font-semibold text-[var(--foreground)] disabled:opacity-50">Batal</button>
                <button type="submit" form="edit-kisi-kisi-slot-form" disabled={savingSlot} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] disabled:opacity-50">{savingSlot ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : <Save size={14} />}Simpan slot</button>
              </div>
            }
          >
            {editingSlot && (
              <form id="edit-kisi-kisi-slot-form" onSubmit={saveSlotEdit} className="space-y-3">
                <PanelHeader icon={<Pencil size={15} />} title="Kisi-kisi manual" subtitle="Perubahan pada slot akan memengaruhi pemetaan/validasi soal yang terhubung." />
                {editingSlot.question && <p className="rounded-lg border border-[var(--warning)]/25 bg-[var(--warning-soft)] px-3 py-2 text-[11px] font-medium text-[var(--warning)]">Slot ini terhubung ke soal. Mengubah indikator, bentuk, atau level dapat membuat soal perlu ditinjau ulang.</p>}
                <MerdekaKisiKisiFields
                  capaianPembelajaran={slotForm.capaianPembelajaran ?? ""}
                  elemenCp={slotForm.elemenCp ?? ""}
                  tujuanPembelajaran={slotForm.tujuanPembelajaran ?? ""}
                  materiPokok={slotForm.materiPokok ?? ""}
                  kelas={slotForm.kelas ?? ""}
                  semester={slotForm.semester ?? ""}
                  cognitiveLevel={slotForm.cognitiveLevel ?? ""}
                  difficulty={slotForm.difficulty ?? ""}
                  indikatorSoal={slotForm.indikatorSoal ?? ""}
                  onCapaianPembelajaran={(v) => setSlotForm({ ...slotForm, capaianPembelajaran: v })}
                  onElemenCp={(v) => setSlotForm({ ...slotForm, elemenCp: v })}
                  onTujuanPembelajaran={(v) => setSlotForm({ ...slotForm, tujuanPembelajaran: v })}
                  onMateriPokok={(v) => setSlotForm({ ...slotForm, materiPokok: v })}
                  onKelas={(v) => setSlotForm({ ...slotForm, kelas: v })}
                  onSemester={(v) => setSlotForm({ ...slotForm, semester: v })}
                  onCognitiveLevel={(v) => setSlotForm({ ...slotForm, cognitiveLevel: v })}
                  onDifficulty={(v) => setSlotForm({ ...slotForm, difficulty: v })}
                  onIndikatorSoal={(v) => setSlotForm({ ...slotForm, indikatorSoal: v })}
                  errors={slotErrors}
                />
              </form>
            )}
          </RightPullSheet>
          <ConfirmDialog
            open={!!deleteTarget}
            title="Hapus slot kisi-kisi?"
            description={deleteTarget?.question ? "Slot akan dihapus dan soal terkait akan dilepas dari kisi-kisi, tetapi soal tidak ikut dihapus." : "Slot akan dihapus dari kisi-kisi exam ini. Tindakan ini tidak menghapus soal."}
            confirmLabel="Hapus slot"
            destructive
            loading={deletingSlot}
            onConfirm={confirmDeleteSlot}
            onCancel={() => setDeleteTarget(null)}
          />
        </>
      )}
    </div>
  );
}


function MobileKisiField({ label, value, clamp }: { label: string; value?: string | null; clamp?: boolean }) {
  return (
    <div>
      <p className="text-[10px] font-bold uppercase tracking-wide text-[var(--muted-foreground)]">{label}</p>
      <p className={cn("mt-0.5 break-words text-[12px] leading-relaxed text-[var(--foreground)]", clamp && "line-clamp-3")}>{value || "-"}</p>
    </div>
  );
}

function CurriculumContextNotice({ context }: { context: ExamCurriculumContext | null }) {
  if (!context) {
    return (
      <div className="mt-3 flex items-start gap-2 rounded-lg border border-[var(--border)] bg-[var(--accent)] px-3 py-2 text-[11px] text-[var(--muted-foreground)]">
        <Info size={13} className="mt-0.5 shrink-0" />
        Memuat CP Kurikulum Merdeka...
      </div>
    );
  }
  if (context.status === "ready") {
    return (
      <div className="mt-3 flex items-start gap-2 rounded-lg border border-[var(--success)]/20 bg-[var(--success-soft)] px-3 py-2 text-[11px] text-[var(--success)]">
        <Info size={13} className="mt-0.5 shrink-0" />
        <div>
          <p className="font-semibold">CP resmi tersedia untuk {context.subjectName || context.subjectCode} {context.gradeLevel ? `kelas ${context.gradeLevel}` : ""}{context.phase ? ` / Fase ${context.phase.toUpperCase()}` : ""}.</p>
          <p className="mt-0.5 opacity-80">Source: {context.source === "remote_fetch" ? "Kemendikdasmendasmen · baru disinkronkan" : "local DB"}</p>
        </div>
      </div>
    );
  }
  if (context.status === "not_applicable") return null;
  return (
    <div className="mt-3 flex items-start gap-2 rounded-lg border border-[var(--warning)]/25 bg-[var(--warning-soft)] px-3 py-2 text-[11px] text-[var(--warning)]">
      <Info size={13} className="mt-0.5 shrink-0" />
      <div>
        <p className="font-semibold">CP resmi belum tersedia untuk exam ini.</p>
        <p className="mt-0.5">{context.warnings[0] ?? "AI tetap bisa membantu draft, tetapi CP/TP perlu diverifikasi manual."}</p>
      </div>
    </div>
  );
}

function KisiActionButton({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-[11px] font-semibold text-[var(--foreground)] transition-all hover:bg-[var(--muted)] active:scale-[0.97]"
    >
      {icon}
      {label}
    </button>
  );
}

function QuestionNumberBadge({ questionNumber, content }: { questionNumber: number; content: string }) {
  const tooltipId = useId();

  return (
    <span className="group relative inline-flex overflow-visible">
      <button
        type="button"
        aria-describedby={tooltipId}
        className="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-[var(--brand-soft)] px-2 text-[11px] font-bold text-[var(--brand)] transition-transform hover:scale-105 focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand)]/40"
      >
        {questionNumber}
      </button>
      <span
        id={tooltipId}
        role="tooltip"
        className="pointer-events-none absolute left-full top-1/2 z-[100] ml-2 hidden w-[min(420px,calc(100vw-48px))] -translate-y-1/2 rounded-xl border border-slate-700 bg-slate-950 p-3 text-left shadow-2xl group-hover:block group-focus-within:block"
      >
        <span className="absolute -left-1.5 top-1/2 h-3 w-3 -translate-y-1/2 rotate-45 border-b border-l border-slate-700 bg-slate-950" />
        <RenderedContent html={content} className="relative max-h-64 overflow-auto whitespace-normal break-words text-[12px] leading-relaxed text-slate-50 [&_*]:text-slate-50" />
      </span>
    </span>
  );
}

function SummaryCard({ label, value, tone }: { label: string; value: string; tone?: "success" | "warning" }) {
  return <div className={cn("rounded-xl border border-[var(--border)] bg-[var(--card)] p-3", tone === "success" && "bg-[var(--success-soft)]", tone === "warning" && "bg-[var(--warning-soft)]")}><p className="text-[10px] font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">{label}</p><p className="mt-1 text-[20px] font-bold text-[var(--foreground)]">{value}</p></div>;
}
