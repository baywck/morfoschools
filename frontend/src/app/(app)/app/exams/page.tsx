"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useCRUD } from "@/lib/use-crud";
import {
  listExams,
  createExam,
  updateExam,
  archiveExam,
  restoreExam,
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
  Send,
  RotateCcw,
  ListChecks,
  ClipboardList,
} from "lucide-react";
import { cn } from "@/lib/cn";

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

  const crud = useCRUD<Exam>({
    name: "Exam",
    list: listExams,
    create: createExam,
    update: updateExam,
    archive: archiveExam,
    restore: restoreExam,
  });

  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [createForm, setCreateForm] = useState({
    title: "",
    description: "",
    subjectId: "",
    examType: "quiz",
    durationMinutes: "",
    maxScore: "100",
    passingScore: "70",
    shuffleQuestions: false,
    shuffleOptions: false,
  });
  const [editForm, setEditForm] = useState({
    title: "",
    description: "",
    subjectId: "",
    examType: "quiz",
    durationMinutes: "",
    maxScore: "100",
    passingScore: "70",
    shuffleQuestions: false,
    shuffleOptions: false,
  });

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
      examType: exam.examType,
      durationMinutes:
        exam.durationMinutes != null ? String(exam.durationMinutes) : "",
      maxScore: String(exam.maxScore),
      passingScore: String(exam.passingScore),
      shuffleQuestions: exam.shuffleQuestions,
      shuffleOptions: exam.shuffleOptions,
    });
  }

  function buildPayload(form: typeof createForm) {
    return {
      title: form.title,
      description: form.description,
      subjectId: form.subjectId || undefined,
      examType: form.examType,
      durationMinutes: form.durationMinutes
        ? Number(form.durationMinutes)
        : undefined,
      maxScore: form.maxScore ? Number(form.maxScore) : undefined,
      passingScore: form.passingScore ? Number(form.passingScore) : undefined,
      shuffleQuestions: form.shuffleQuestions,
      shuffleOptions: form.shuffleOptions,
    };
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const ok = await crud.handleCreate(buildPayload(createForm));
    if (ok) {
      setCreateForm({
        title: "",
        description: "",
        subjectId: "",
        examType: "quiz",
        durationMinutes: "",
        maxScore: "100",
        passingScore: "70",
        shuffleQuestions: false,
        shuffleOptions: false,
      });
    }
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    await crud.handleEdit(crud.editTarget.id, buildPayload(editForm));
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
                  onClick={() => router.push(`/app/exams/${e.id}`)}
                >
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]">
                    <ClipboardCheck size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">
                      {e.title}
                    </p>
                    <p className="text-[11px] text-[var(--muted-foreground)] truncate">
                      {e.subjectName || "No subject"} · {e.examType} ·{" "}
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
                    <RowActions
                      actions={
                        e.status === "archived"
                          ? [
                              {
                                label: "Restore",
                                icon: <RotateCcw size={14} />,
                                onClick: () => crud.handleRestore(e.id),
                              },
                            ]
                          : [
                              {
                                label: "Manage soal",
                                icon: <ListChecks size={14} />,
                                onClick: () => router.push(`/app/exams/${e.id}`),
                              },
                              {
                                label: "Manage blueprint",
                                icon: <ClipboardList size={14} />,
                                onClick: () =>
                                  router.push(`/app/exams/${e.id}#blueprint`),
                              },
                              {
                                label: "Edit",
                                icon: <Pencil size={14} />,
                                onClick: () => openEdit(e),
                              },
                              ...(e.status === "draft"
                                ? [
                                    {
                                      label: "Publish",
                                      icon: <Send size={14} />,
                                      onClick: () => handlePublish(e),
                                    },
                                  ]
                                : []),
                              {
                                label: "Archive",
                                icon: <Trash2 size={14} />,
                                onClick: () => crud.setArchiveTarget(e),
                                variant: "danger" as const,
                              },
                            ]
                      }
                    />
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
        title="Add Exam"
        onClose={() => crud.setShowCreate(false)}
      >
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="Title"
            value={createForm.title}
            onChange={(e) =>
              setCreateForm({ ...createForm, title: e.target.value })
            }
            error={crud.fieldErrors.title}
          />
          <InputField
            label="Description (optional)"
            value={createForm.description}
            onChange={(e) =>
              setCreateForm({ ...createForm, description: e.target.value })
            }
          />
          <SelectField
            label="Subject (optional)"
            value={createForm.subjectId}
            onChange={(val) => setCreateForm({ ...createForm, subjectId: val })}
            options={[
              { value: "", label: "None" },
              ...subjects.map((s) => ({ value: s.id, label: s.name })),
            ]}
          />
          <SelectField
            label="Exam Type"
            value={createForm.examType}
            onChange={(val) => setCreateForm({ ...createForm, examType: val })}
            options={examTypeOptions}
          />
          <InputField
            label="Duration (minutes, optional)"
            type="number"
            value={createForm.durationMinutes}
            onChange={(e) =>
              setCreateForm({ ...createForm, durationMinutes: e.target.value })
            }
          />
          <div className="grid grid-cols-2 gap-3">
            <InputField
              label="Max Score"
              type="number"
              value={createForm.maxScore}
              onChange={(e) =>
                setCreateForm({ ...createForm, maxScore: e.target.value })
              }
            />
            <InputField
              label="Passing Score"
              type="number"
              value={createForm.passingScore}
              onChange={(e) =>
                setCreateForm({ ...createForm, passingScore: e.target.value })
              }
            />
          </div>
          <label className="flex items-center gap-2 text-[12px] text-[var(--foreground)]">
            <input
              type="checkbox"
              checked={createForm.shuffleQuestions}
              onChange={(e) =>
                setCreateForm({ ...createForm, shuffleQuestions: e.target.checked })
              }
            />
            Shuffle questions
          </label>
          <label className="flex items-center gap-2 text-[12px] text-[var(--foreground)]">
            <input
              type="checkbox"
              checked={createForm.shuffleOptions}
              onChange={(e) =>
                setCreateForm({ ...createForm, shuffleOptions: e.target.checked })
              }
            />
            Shuffle options
          </label>
          <div className="flex gap-2 justify-end pt-3">
            <button
              type="button"
              onClick={() => crud.setShowCreate(false)}
              className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={crud.creating}
              className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all"
            >
              {crud.creating && (
                <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />
              )}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit Sheet */}
      <RightPullSheet
        open={!!crud.editTarget}
        title="Edit Exam"
        onClose={() => crud.setEditTarget(null)}
      >
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="Title"
            value={editForm.title}
            onChange={(e) => setEditForm({ ...editForm, title: e.target.value })}
            error={crud.fieldErrors.title}
          />
          <InputField
            label="Description"
            value={editForm.description}
            onChange={(e) =>
              setEditForm({ ...editForm, description: e.target.value })
            }
          />
          <SelectField
            label="Subject"
            value={editForm.subjectId}
            onChange={(val) => setEditForm({ ...editForm, subjectId: val })}
            options={[
              { value: "", label: "None" },
              ...subjects.map((s) => ({ value: s.id, label: s.name })),
            ]}
          />
          <SelectField
            label="Exam Type"
            value={editForm.examType}
            onChange={(val) => setEditForm({ ...editForm, examType: val })}
            options={examTypeOptions}
          />
          <InputField
            label="Duration (minutes)"
            type="number"
            value={editForm.durationMinutes}
            onChange={(e) =>
              setEditForm({ ...editForm, durationMinutes: e.target.value })
            }
          />
          <div className="grid grid-cols-2 gap-3">
            <InputField
              label="Max Score"
              type="number"
              value={editForm.maxScore}
              onChange={(e) => setEditForm({ ...editForm, maxScore: e.target.value })}
            />
            <InputField
              label="Passing Score"
              type="number"
              value={editForm.passingScore}
              onChange={(e) =>
                setEditForm({ ...editForm, passingScore: e.target.value })
              }
            />
          </div>
          <label className="flex items-center gap-2 text-[12px] text-[var(--foreground)]">
            <input
              type="checkbox"
              checked={editForm.shuffleQuestions}
              onChange={(e) =>
                setEditForm({ ...editForm, shuffleQuestions: e.target.checked })
              }
            />
            Shuffle questions
          </label>
          <label className="flex items-center gap-2 text-[12px] text-[var(--foreground)]">
            <input
              type="checkbox"
              checked={editForm.shuffleOptions}
              onChange={(e) =>
                setEditForm({ ...editForm, shuffleOptions: e.target.checked })
              }
            />
            Shuffle options
          </label>
          <div className="flex gap-2 justify-end pt-3">
            <button
              type="button"
              onClick={() => crud.setEditTarget(null)}
              className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={crud.editing}
              className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all"
            >
              {crud.editing ? (
                <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />
              ) : (
                "Save Changes"
              )}
            </button>
          </div>
        </form>
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
    </>
  );
}
