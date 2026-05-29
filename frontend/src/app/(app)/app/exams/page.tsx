"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-provider";
import { useCRUD } from "@/lib/use-crud";
import {
  listExams,
  createExam,
  updateExam,
  archiveExam,
  restoreExam,
  hardDeleteExam,
  publishExam,
  listSubjects,
  type Exam,
  type Subject,
} from "@/lib/modules-api";
import { useEffect } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import {
  ClipboardCheck,
  Pencil,
  Trash2,
  Plus,
  Save,
  Send,
  RotateCcw,
  ListChecks,
  ClipboardList,
  Settings2,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { cn } from "@/lib/cn";
import { tenantEnabledPhases } from "@/lib/tenant-education";
import { gradeOptionsForPhases, phaseForGrade } from "@/lib/grade-options";
import { TextareaField } from "@/components/ui/textarea-field";
import { ToggleSwitch } from "@/components/ui/toggle-switch";

const examTypeOptions = [
  { value: "quiz", label: "Quiz" },
  { value: "midterm", label: "Midterm" },
  { value: "final", label: "Final" },
  { value: "tryout", label: "Tryout" },
  { value: "daily", label: "Daily" },
];

const statusTone = (status: string) => {
  switch (status) {
    case "published":
      return "bg-[var(--success-soft)] text-[var(--success)]";
    case "draft":
      return "bg-[var(--warning-soft)] text-[var(--warning)]";
    case "archived":
      return "bg-[var(--muted)] text-[var(--muted-foreground)]";
    default:
      return "bg-[var(--muted)] text-[var(--muted-foreground)]";
  }
};

export default function ExamsPage() {
  const router = useRouter();
  const { toast } = useToast();
  const { session } = useAuth();
  const gradeOptions = useMemo(() => gradeOptionsForPhases(tenantEnabledPhases(session?.effectiveTenant)), [session?.effectiveTenant]);

  const crud = useCRUD<Exam>({
    name: "Exam",
    list: listExams,
    create: createExam,
    update: updateExam,
    archive: archiveExam,
    restore: restoreExam,
  });

  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [deleteTarget, setDeleteTarget] = useState<Exam | null>(null);
  const [deleting, setDeleting] = useState(false);
  const emptyExamForm = {
    title: "",
    description: "",
    subjectId: "",
    gradeLevel: "",
    examType: "quiz",
    durationMinutes: "",
    maxScore: "100",
    passingScore: "70",
    shuffleQuestions: false,
    shuffleOptions: false,
    showResultImmediately: false,
    usesKisiKisi: false,
  };
  const [createForm, setCreateForm] = useState(emptyExamForm);
  const [editForm, setEditForm] = useState(emptyExamForm);

  const actionsForExam = (e: Exam) => {
    const base = [
      {
        label: "Manage soal",
        icon: <ListChecks size={14} />,
        onClick: () => router.push(`/app/exams/${e.id}/questions`),
      },
      {
        label: "Manage blueprint",
        icon: <ClipboardList size={14} />,
        onClick: () => router.push(`/app/exams/${e.id}/kisi-kisi`),
      },
    ];
    const canWrite = e.canWrite !== false;
    const canDelete = e.canDelete === true;
    if (e.status === "archived") {
      return [
        ...(canDelete ? [{ label: "Restore", icon: <RotateCcw size={14} />, onClick: () => crud.handleRestore(e.id) }] : []),
        ...(canDelete ? [{ label: "Hard delete", icon: <Trash2 size={14} />, onClick: () => setDeleteTarget(e), variant: "danger" as const }] : []),
      ];
    }
    return [
      ...base,
      ...(canWrite ? [{ label: "Edit", icon: <Pencil size={14} />, onClick: () => openEdit(e) }] : []),
      ...(canWrite && e.status === "draft" ? [{ label: "Publish", icon: <Send size={14} />, onClick: () => handlePublish(e) }] : []),
      ...(canDelete ? [{ label: "Archive", icon: <Trash2 size={14} />, onClick: () => crud.setArchiveTarget(e), variant: "danger" as const }] : []),
      ...(canDelete ? [{ label: "Hard delete", icon: <Trash2 size={14} />, onClick: () => setDeleteTarget(e), variant: "danger" as const }] : []),
    ];
  };

  // Load subjects once on mount for the subject dropdowns.
  useEffect(() => {
    listSubjects({ status: "active" }).then((res) => {
      if (res.data) setSubjects(res.data.data);
    });
  }, []);

  function openEdit(exam: Exam) {
    crud.setEditTarget(exam);
    crud.setFieldErrors({});
    setEditForm({
      title: exam.title,
      description: exam.description ?? "",
      subjectId: exam.subjectId ?? "",
      gradeLevel: exam.gradeLevel ?? "",
      examType: exam.examType,
      durationMinutes:
        exam.durationMinutes != null ? String(exam.durationMinutes) : "",
      maxScore: String(exam.maxScore),
      passingScore: String(exam.passingScore),
      shuffleQuestions: exam.shuffleQuestions,
      shuffleOptions: exam.shuffleOptions,
      showResultImmediately: exam.showResultImmediately,
      usesKisiKisi: exam.usesKisiKisi,
    });
  }

  function buildPayload(form: typeof createForm) {
    return {
      title: form.title,
      description: form.description,
      subjectId: form.subjectId || undefined,
      gradeLevel: form.gradeLevel || undefined,
      examType: form.examType,
      durationMinutes: form.durationMinutes
        ? Number(form.durationMinutes)
        : undefined,
      maxScore: form.maxScore ? Number(form.maxScore) : undefined,
      passingScore: form.passingScore ? Number(form.passingScore) : undefined,
      shuffleQuestions: form.shuffleQuestions,
      shuffleOptions: form.shuffleOptions,
      showResultImmediately: form.showResultImmediately,
      usesKisiKisi: form.usesKisiKisi,
    };
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const ok = await crud.handleCreate(buildPayload(createForm));
    if (ok) {
      setCreateForm(emptyExamForm);
    }
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    const { subjectId: _subjectId, gradeLevel: _gradeLevel, ...payload } = buildPayload(editForm);
    void _subjectId;
    void _gradeLevel;
    await crud.handleEdit(crud.editTarget.id, payload);
  }

  async function handleHardDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    const res = await hardDeleteExam(deleteTarget.id);
    setDeleting(false);
    if (res.error) {
      toast({ tone: "error", title: "Delete failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Exam permanently deleted" });
    setDeleteTarget(null);
    crud.reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handlePublish(exam: Exam) {
    if (exam.questionCount === 0) {
      toast({
        tone: "error",
        title: "Cannot publish",
        description: "Add at least one question before publishing.",
      });
      return;
    }
    const res = await publishExam(exam.id);
    if (res.error) {
      toast({ tone: "error", title: "Publish failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Exam published" });
    crud.reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  return (
    <>
      <PageShell
        title="Exams"
        subtitle={`${crud.total} exam${crud.total !== 1 ? "s" : ""}`}
        search={{
          value: crud.search,
          onChange: crud.setSearch,
          placeholder: "Search exams...",
        }}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Exam"
      >
        {crud.loading ? (
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        ) : crud.items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <ClipboardCheck size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">
              No exams yet
            </p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">
              Create your first exam to start adding questions and scheduling.
            </p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((e) => (
                <div
                  key={e.id}
                  className="group flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors cursor-pointer"
                  onClick={() => router.push(`/app/exams/${e.id}/setup`)}
                >
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]">
                    <ClipboardCheck size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">
                      {e.title}
                    </p>
                    <p className="text-[11px] text-[var(--muted-foreground)] truncate">
                      {e.subjectName || "No subject"} · {e.gradeLevel ? `Kelas ${e.gradeLevel} · ` : ""}{e.examType} ·{" "}
                      {e.questionCount} question{e.questionCount !== 1 ? "s" : ""} ·{" "}
                      {e.totalPoints} pts
                    </p>
                  </div>
                  <span
                    className={cn(
                      "rounded-md px-2 py-0.5 text-[10px] font-medium",
                      statusTone(e.status)
                    )}
                  >
                    {e.status}
                  </span>
                  <div onClick={(ev) => ev.stopPropagation()}>
                    <RowActions actions={actionsForExam(e)} />
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet
        open={crud.showCreate}
        title="Create Exam"
        width="lg"
        onClose={() => crud.setShowCreate(false)}
      >
        <ExamFormSheet
          mode="create"
          form={createForm}
          setForm={setCreateForm}
          subjects={subjects}
          gradeOptions={gradeOptions}
          fieldErrors={crud.fieldErrors}
          saving={crud.creating}
          onSubmit={handleCreate}
          onCancel={() => crud.setShowCreate(false)}
        />
      </RightPullSheet>

      {/* Edit Sheet */}
      <RightPullSheet
        open={!!crud.editTarget}
        title="Edit Exam"
        width="lg"
        onClose={() => crud.setEditTarget(null)}
      >
        <ExamFormSheet
          mode="edit"
          form={editForm}
          setForm={setEditForm}
          subjects={subjects}
          gradeOptions={gradeOptions}
          fieldErrors={crud.fieldErrors}
          saving={crud.editing}
          onSubmit={handleEdit}
          onCancel={() => crud.setEditTarget(null)}
        />
      </RightPullSheet>

      {/* Archive Confirm */}
      <ConfirmDialog
        open={!!crud.archiveTarget}
        title="Archive Exam"
        description={`Archive "${crud.archiveTarget?.title}"? You can restore it later.`}
        confirmLabel="Archive"
        destructive
        loading={crud.archiving}
        onConfirm={() =>
          crud.archiveTarget && crud.handleArchive(crud.archiveTarget.id)
        }
        onCancel={() => crud.setArchiveTarget(null)}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        title="Permanently delete exam?"
        description={`Hard delete "${deleteTarget?.title}" and all related questions, options, sections, groups, blueprint snapshot, collaborators, attempts, and AI context. This cannot be undone.`}
        confirmLabel="Hard delete"
        destructive
        loading={deleting}
        onConfirm={handleHardDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </>
  );
}

function PanelHeader({ icon, title, subtitle }: { icon: React.ReactNode; title: string; subtitle: string }) {
  return <div className="flex items-start gap-2.5"><div className="mt-0.5 flex h-7 w-7 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">{icon}</div><div><h3 className="text-[13px] font-bold text-[var(--foreground)]">{title}</h3><p className="text-[11px] text-[var(--muted-foreground)]">{subtitle}</p></div></div>;
}

function SettingToggle({ title, desc, checked, onChange }: { title: string; desc: string; checked: boolean; onChange: (v: boolean) => void }) {
  return <div className="flex items-center justify-between gap-3 rounded-lg border border-[var(--border)] bg-[var(--accent)] px-3 py-2"><div><p className="text-[12px] font-semibold text-[var(--foreground)]">{title}</p><p className="text-[10px] text-[var(--muted-foreground)]">{desc}</p></div><ToggleSwitch checked={checked} onChange={onChange} ariaLabel={title} /></div>;
}

type ExamFormSheetForm = {
  title: string;
  description: string;
  subjectId: string;
  gradeLevel: string;
  examType: string;
  durationMinutes: string;
  maxScore: string;
  passingScore: string;
  shuffleQuestions: boolean;
  shuffleOptions: boolean;
  showResultImmediately: boolean;
  usesKisiKisi: boolean;
};

function ExamFormSheet({
  mode,
  form,
  setForm,
  subjects,
  gradeOptions,
  fieldErrors,
  saving,
  onSubmit,
  onCancel,
}: {
  mode: "create" | "edit";
  form: ExamFormSheetForm;
  setForm: (form: ExamFormSheetForm) => void;
  subjects: Subject[];
  gradeOptions: string[];
  fieldErrors: Record<string, string>;
  saving: boolean;
  onSubmit: (e: React.FormEvent) => void;
  onCancel: () => void;
}) {
  return (
    <form onSubmit={onSubmit} className="space-y-3">
      <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
        <PanelHeader icon={<Settings2 size={15} />} title="Basic setup" subtitle={mode === "create" ? "Pilih mapel dan kelas dengan benar. Subject dan Grade dikunci setelah exam dibuat." : "Identitas utama exam. Subject dan Grade tidak bisa diedit setelah dibuat."} />
        <div className="mt-3 grid gap-2 md:grid-cols-2">
          <InputField label="Nama ujian" value={form.title} error={fieldErrors.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
          <SelectField label="Exam type" value={form.examType} options={examTypeOptions} onChange={(v) => setForm({ ...form, examType: v })} />
          <SelectField label="Subject" value={form.subjectId} disabled={mode === "edit"} helperText={mode === "create" ? "Tidak bisa diedit setelah exam dibuat." : "Subject dikunci setelah exam dibuat."} error={fieldErrors.subjectId} options={[{ value: "", label: "None" }, ...subjects.map((s) => ({ value: s.id, label: s.name }))]} onChange={(v) => setForm({ ...form, subjectId: v })} />
          <SelectField label="Grade" value={form.gradeLevel} disabled={mode === "edit"} helperText={mode === "create" ? "Tidak bisa diedit setelah exam dibuat." : "Grade dikunci setelah exam dibuat."} error={fieldErrors.gradeLevel} options={[{ value: "", label: "None" }, ...gradeOptions.map((grade) => ({ value: grade, label: `Kelas ${grade} · Fase ${(phaseForGrade(grade) || "?").toUpperCase()}` }))]} onChange={(v) => setForm({ ...form, gradeLevel: v })} />
          <InputField label="Duration minutes" inputMode="numeric" value={form.durationMinutes} error={fieldErrors.durationMinutes} onChange={(e) => setForm({ ...form, durationMinutes: e.target.value.replace(/\D/g, "").slice(0, 4) })} />
          <div className="grid grid-cols-2 gap-2">
            <InputField label="Max score" inputMode="numeric" value={form.maxScore} error={fieldErrors.maxScore} onChange={(e) => setForm({ ...form, maxScore: e.target.value })} />
            <InputField label="Passing" inputMode="numeric" value={form.passingScore} error={fieldErrors.passingScore} onChange={(e) => setForm({ ...form, passingScore: e.target.value })} />
          </div>
          <div className="md:col-span-2">
            <TextareaField label="Deskripsi" rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
          </div>
        </div>
      </div>

      <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
        <PanelHeader icon={<ShieldCheck size={15} />} title="Exam behavior" subtitle="Default perilaku saat exam dikerjakan lewat delivery context." />
        <div className="mt-3 grid gap-2 md:grid-cols-3">
          <SettingToggle title="Random questions" desc="Acak urutan soal." checked={form.shuffleQuestions} onChange={(v) => setForm({ ...form, shuffleQuestions: v })} />
          <SettingToggle title="Random answers" desc="Acak opsi pilihan ganda." checked={form.shuffleOptions} onChange={(v) => setForm({ ...form, shuffleOptions: v })} />
          <SettingToggle title="Show result" desc="Tampilkan hasil setelah submit." checked={form.showResultImmediately} onChange={(v) => setForm({ ...form, showResultImmediately: v })} />
        </div>
      </div>

      <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-3">
        <PanelHeader icon={<Sparkles size={15} />} title="Kisi-kisi" subtitle="Aktifkan blueprint agar soal bisa dipetakan ke indikator dan coverage." />
        <div className="mt-3 flex items-center justify-between rounded-lg border border-[var(--border)] bg-[var(--accent)] px-3 py-2">
          <div><p className="text-[12px] font-semibold text-[var(--foreground)]">Use Kisi-Kisi / Blueprint</p><p className="text-[11px] text-[var(--muted-foreground)]">Jika aktif saat create, container blueprint kosong akan dibuat otomatis.</p></div>
          <ToggleSwitch checked={form.usesKisiKisi} onChange={(v) => setForm({ ...form, usesKisiKisi: v })} ariaLabel="Toggle kisi-kisi" />
        </div>
      </div>

      <div className="flex gap-2 justify-end pt-1">
        <button type="button" onClick={onCancel} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
        <button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
          {saving ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : mode === "create" ? <Plus size={14} /> : <Save size={14} />}
          {mode === "create" ? "Create Exam" : "Save Changes"}
        </button>
      </div>
    </form>
  );
}
