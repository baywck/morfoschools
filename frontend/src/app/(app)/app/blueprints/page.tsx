"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useCRUD } from "@/lib/use-crud";
import {
  listBlueprintTemplates,
  createBlueprintTemplate,
  updateBlueprintTemplate,
  archiveBlueprintTemplate,
  restoreBlueprintTemplate,
  publishBlueprintTemplate,
  listSubjects,
  type BlueprintTemplate,
  type CurriculumCode,
  type BlueprintType,
  type BlueprintStatus,
  type Subject,
} from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import {
  ClipboardList,
  Pencil,
  Trash2,
  Send,
  RotateCcw,
  Lock,
} from "lucide-react";
import { cn } from "@/lib/cn";

const curriculumOptions: { value: CurriculumCode; label: string }[] = [
  { value: "k13", label: "K13 (Kurikulum 2013)" },
  { value: "merdeka", label: "Kurikulum Merdeka" },
  { value: "akm_numerasi", label: "AKM Numerasi" },
  { value: "akm_literasi", label: "AKM Literasi" },
];

const blueprintTypeOptions: { value: BlueprintType; label: string }[] = [
  { value: "reguler", label: "Reguler" },
  { value: "akm_literasi", label: "AKM Literasi" },
  { value: "akm_numerasi", label: "AKM Numerasi" },
];

const statusFilterOptions: { value: "" | BlueprintStatus; label: string }[] = [
  { value: "", label: "All" },
  { value: "draft", label: "Draft" },
  { value: "published", label: "Published" },
  { value: "archived", label: "Archived" },
];

const statusTone = (s: string) => {
  switch (s) {
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

export default function BlueprintsPage() {
  const router = useRouter();
  const { toast } = useToast();

  const [statusFilter, setStatusFilter] = useState<"" | BlueprintStatus>("");
  const [curriculumFilter, setCurriculumFilter] = useState<"" | CurriculumCode>("");

  const crud = useCRUD<BlueprintTemplate>({
    name: "Blueprint",
    list: (params) =>
      listBlueprintTemplates({
        ...params,
        status: statusFilter || undefined,
        curriculum: curriculumFilter || undefined,
      }),
    create: createBlueprintTemplate,
    update: updateBlueprintTemplate,
    archive: archiveBlueprintTemplate,
    restore: restoreBlueprintTemplate,
  });

  // Load active subjects from master data so the create/edit forms
  // can offer a select instead of free-text. Per user feedback, the
  // tenant already curates this list under /app/academic/subjects.
  const [subjects, setSubjects] = useState<Subject[]>([]);
  useEffect(() => {
    listSubjects({ status: "active" }).then((res) => {
      if (res.data) setSubjects(res.data.data);
    });
  }, []);
  const subjectOptions = [
    { value: "", label: "— Tidak terkait subject —" },
    ...subjects.map((s) => ({
      value: s.code,
      label: `${s.name} (${s.code})`,
    })),
  ];

  const [createForm, setCreateForm] = useState({
    title: "",
    description: "",
    curriculumCode: "k13" as CurriculumCode,
    blueprintType: "reguler" as BlueprintType,
    subjectCode: "",
    gradeOrPhase: "",
  });
  const [editForm, setEditForm] = useState({
    title: "",
    description: "",
    subjectCode: "",
    gradeOrPhase: "",
  });

  function openEdit(t: BlueprintTemplate) {
    crud.setEditTarget(t);
    crud.setFieldErrors({});
    setEditForm({
      title: t.title,
      description: t.description ?? "",
      subjectCode: t.subjectCode ?? "",
      gradeOrPhase: t.gradeOrPhase ?? "",
    });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const ok = await crud.handleCreate({
      title: createForm.title,
      description: createForm.description || undefined,
      curriculumCode: createForm.curriculumCode,
      blueprintType: createForm.blueprintType,
      subjectCode: createForm.subjectCode || undefined,
      gradeOrPhase: createForm.gradeOrPhase || undefined,
    });
    if (ok) {
      setCreateForm({
        title: "",
        description: "",
        curriculumCode: "k13",
        blueprintType: "reguler",
        subjectCode: "",
        gradeOrPhase: "",
      });
    }
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    await crud.handleEdit(crud.editTarget.id, {
      title: editForm.title,
      description: editForm.description,
      subjectCode: editForm.subjectCode,
      gradeOrPhase: editForm.gradeOrPhase,
    });
  }

  async function handlePublish(t: BlueprintTemplate) {
    if (t.totalSlots === 0) {
      toast({
        tone: "error",
        title: "Cannot publish",
        description: "Tambah minimal satu slot dulu.",
      });
      return;
    }
    const res = await publishBlueprintTemplate(t.id);
    if (res.error) {
      toast({ tone: "error", title: "Publish failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Blueprint published" });
    crud.reload();
  }

  return (
    <>
      <PageShell
        title="Blueprints"
        subtitle={`${crud.total} kisi-kisi`}
        search={{
          value: crud.search,
          onChange: crud.setSearch,
          placeholder: "Search blueprints...",
        }}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Blueprint"
      >
        {/* Filters */}
        <div className="mb-3 flex flex-wrap gap-2">
          <div className="flex flex-wrap gap-1.5">
            {statusFilterOptions.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setStatusFilter(opt.value)}
                className={cn(
                  "h-7 rounded-md border px-2.5 text-[11px] font-medium transition-colors",
                  statusFilter === opt.value
                    ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]"
                    : "border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] hover:text-[var(--foreground)]",
                )}
              >
                {opt.label}
              </button>
            ))}
          </div>
          <div className="ml-auto flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={() => setCurriculumFilter("")}
              className={cn(
                "h-7 rounded-md border px-2.5 text-[11px] font-medium transition-colors",
                curriculumFilter === ""
                  ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]"
                  : "border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] hover:text-[var(--foreground)]",
              )}
            >
              All curriculum
            </button>
            {curriculumOptions.map((c) => (
              <button
                key={c.value}
                type="button"
                onClick={() => setCurriculumFilter(c.value)}
                className={cn(
                  "h-7 rounded-md border px-2.5 text-[11px] font-medium transition-colors",
                  curriculumFilter === c.value
                    ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]"
                    : "border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] hover:text-[var(--foreground)]",
                )}
              >
                {c.value.replace("akm_", "AKM ").replace("_", " ")}
              </button>
            ))}
          </div>
        </div>

        {crud.loading ? (
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        ) : crud.items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <ClipboardList size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">
              Belum ada blueprint
            </p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">
              Buat blueprint template pertama untuk mulai membangun kisi-kisi reusable.
            </p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((t) => (
                <div
                  key={t.id}
                  className={cn(
                    "group flex items-center gap-4 px-3 py-3 transition-colors",
                    t.canAccess
                      ? "hover:bg-[var(--muted)]/50 cursor-pointer"
                      : "opacity-60 cursor-not-allowed",
                  )}
                  onClick={() => t.canAccess && router.push(`/app/blueprints/${t.id}`)}
                >
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]">
                    {t.canAccess ? <ClipboardList size={16} /> : <Lock size={14} />}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">
                      {t.title}
                    </p>
                    <p className="text-[11px] text-[var(--muted-foreground)] truncate">
                      {t.curriculumCode.toUpperCase()} · {t.blueprintType.replace("akm_", "AKM ")}
                      {" · "}
                      {t.totalSlots} slot{t.totalSlots !== 1 ? "s" : ""} · {t.totalPoints} pts
                      {t.subjectCode ? ` · ${t.subjectCode}` : ""}
                      {t.gradeOrPhase ? ` · ${t.gradeOrPhase}` : ""}
                    </p>
                  </div>
                  <span
                    className={cn(
                      "rounded-md px-2 py-0.5 text-[10px] font-medium",
                      statusTone(t.status),
                    )}
                  >
                    {t.status}
                  </span>
                  {t.canAccess && (
                    <div onClick={(ev) => ev.stopPropagation()}>
                      <RowActions
                        actions={
                          t.status === "archived"
                            ? [
                                {
                                  label: "Restore",
                                  icon: <RotateCcw size={14} />,
                                  onClick: () => crud.handleRestore(t.id),
                                },
                              ]
                            : [
                                {
                                  label: "Edit",
                                  icon: <Pencil size={14} />,
                                  onClick: () => openEdit(t),
                                },
                                ...(t.status === "draft"
                                  ? [
                                      {
                                        label: "Publish",
                                        icon: <Send size={14} />,
                                        onClick: () => handlePublish(t),
                                      },
                                    ]
                                  : []),
                                {
                                  label: "Archive",
                                  icon: <Trash2 size={14} />,
                                  onClick: () => crud.setArchiveTarget(t),
                                  variant: "danger" as const,
                                },
                              ]
                        }
                      />
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create sheet */}
      <RightPullSheet
        open={crud.showCreate}
        title="Add Blueprint Template"
        onClose={() => crud.setShowCreate(false)}
      >
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="Title"
            value={createForm.title}
            onChange={(e) => setCreateForm({ ...createForm, title: e.target.value })}
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
            label="Curriculum"
            value={createForm.curriculumCode}
            onChange={(v) =>
              setCreateForm({ ...createForm, curriculumCode: v as CurriculumCode })
            }
            options={curriculumOptions}
          />
          <SelectField
            label="Blueprint Type"
            value={createForm.blueprintType}
            onChange={(v) =>
              setCreateForm({ ...createForm, blueprintType: v as BlueprintType })
            }
            options={blueprintTypeOptions}
          />
          <SelectField
            label="Subject"
            value={createForm.subjectCode}
            onChange={(v) =>
              setCreateForm({ ...createForm, subjectCode: v })
            }
            options={subjectOptions}
          />
          <InputField
            label="Grade / phase (optional)"
            value={createForm.gradeOrPhase}
            onChange={(e) =>
              setCreateForm({ ...createForm, gradeOrPhase: e.target.value })
            }
          />
          <div className="flex justify-end gap-2 pt-2">
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
              Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit sheet */}
      <RightPullSheet
        open={!!crud.editTarget}
        title="Edit Blueprint"
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
            value={editForm.subjectCode}
            onChange={(v) =>
              setEditForm({ ...editForm, subjectCode: v })
            }
            options={subjectOptions}
          />
          <InputField
            label="Grade / phase"
            value={editForm.gradeOrPhase}
            onChange={(e) =>
              setEditForm({ ...editForm, gradeOrPhase: e.target.value })
            }
          />
          <div className="flex justify-end gap-2 pt-2">
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
              {crud.editing && (
                <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />
              )}
              Save
            </button>
          </div>
        </form>
      </RightPullSheet>

      <ConfirmDialog
        open={!!crud.archiveTarget}
        title="Archive Blueprint?"
        description={`Blueprint "${crud.archiveTarget?.title}" akan diarsipkan. Exam yang sudah pakai (snapshot) tidak terpengaruh.`}
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
