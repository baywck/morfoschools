"use client";

import { useState } from "react";
import { useCRUD } from "@/lib/use-crud";
import {
  listStimuli,
  createStimulus,
  updateStimulus,
  archiveStimulus,
  promoteStimulus,
  type Stimulus,
  type StimulusLifecycle,
} from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RenderedContent } from "@/components/ui/rendered-content";
import { useToast } from "@/components/ui/toast";
import { Library, Pencil, Trash2, Send, Eye } from "lucide-react";
import { cn } from "@/lib/cn";

const lifecycleFilterOptions: { value: StimulusLifecycle | "all"; label: string }[] = [
  { value: "shared", label: "Library (shared)" },
  { value: "exam_scoped", label: "Exam-scoped" },
  { value: "archived", label: "Archived" },
  { value: "all", label: "All" },
];

const lifecycleTone = (lifecycle: string) => {
  switch (lifecycle) {
    case "shared":
      return "bg-[var(--success-soft)] text-[var(--success)]";
    case "exam_scoped":
      return "bg-[var(--warning-soft)] text-[var(--warning)]";
    case "archived":
      return "bg-[var(--muted)] text-[var(--muted-foreground)]";
    default:
      return "bg-[var(--muted)] text-[var(--muted-foreground)]";
  }
};

export default function StimuliPage() {
  const { toast } = useToast();
  const [lifecycle, setLifecycle] = useState<StimulusLifecycle | "all">("shared");

  // Custom list call to honor lifecycle filter; useCRUD doesn't know about it.
  const crud = useCRUD<Stimulus>({
    name: "Stimulus",
    list: (params) => listStimuli({ ...params, lifecycle }),
    create: (data) => createStimulus(data),
    update: (id, data) => updateStimulus(id, data),
    archive: archiveStimulus,
  });

  const [createForm, setCreateForm] = useState({
    title: "",
    content: "",
    source: "",
    lifecycle: "shared" as "shared" | "exam_scoped",
  });
  const [editForm, setEditForm] = useState({
    title: "",
    content: "",
    source: "",
  });
  const [viewTarget, setViewTarget] = useState<Stimulus | null>(null);

  function openEdit(s: Stimulus) {
    crud.setEditTarget(s);
    crud.setFieldErrors({});
    setEditForm({
      title: s.title,
      content: s.content,
      source: s.source ?? "",
    });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const ok = await crud.handleCreate({
      title: createForm.title,
      content: createForm.content,
      source: createForm.source || undefined,
      lifecycle: createForm.lifecycle,
    });
    if (ok) {
      setCreateForm({ title: "", content: "", source: "", lifecycle: "shared" });
    }
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    await crud.handleEdit(crud.editTarget.id, {
      title: editForm.title,
      content: editForm.content,
      source: editForm.source || undefined,
    });
  }

  async function handlePromote(s: Stimulus) {
    const res = await promoteStimulus(s.id);
    if (res.error) {
      toast({ tone: "error", title: "Promote failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Stimulus promoted to library" });
    crud.reload();
  }

  return (
    <>
      <PageShell
        title="Stimuli"
        subtitle={`${crud.total} stimulus${crud.total !== 1 ? "" : ""}`}
        search={{
          value: crud.search,
          onChange: crud.setSearch,
          placeholder: "Search stimuli...",
        }}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Stimulus"
      >
        {/* Lifecycle filter */}
        <div className="mb-3 flex flex-wrap gap-1.5">
          {lifecycleFilterOptions.map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => setLifecycle(opt.value)}
              className={cn(
                "h-7 rounded-md border px-2.5 text-[11px] font-medium transition-colors",
                lifecycle === opt.value
                  ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]"
                  : "border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] hover:text-[var(--foreground)]",
              )}
            >
              {opt.label}
            </button>
          ))}
        </div>

        {crud.loading ? (
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        ) : crud.items.length === 0 ? (
          <EmptyState lifecycle={lifecycle} />
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((s) => (
                <div
                  key={s.id}
                  className="group flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors"
                >
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]">
                    <Library size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">
                      {s.title}
                    </p>
                    <p className="text-[11px] text-[var(--muted-foreground)] truncate">
                      Owner: {s.ownerName || "—"} · used in {s.usageCount} group
                      {s.usageCount !== 1 ? "s" : ""}
                      {s.source ? ` · ${s.source}` : ""}
                    </p>
                  </div>
                  <span
                    className={cn(
                      "rounded-md px-2 py-0.5 text-[10px] font-medium",
                      lifecycleTone(s.lifecycle),
                    )}
                  >
                    {s.lifecycle.replace("_", " ")}
                  </span>
                  <RowActions
                    actions={[
                      {
                        label: "View",
                        icon: <Eye size={14} />,
                        onClick: () => setViewTarget(s),
                      },
                      ...(s.lifecycle !== "archived"
                        ? [
                            {
                              label: "Edit",
                              icon: <Pencil size={14} />,
                              onClick: () => openEdit(s),
                            },
                          ]
                        : []),
                      ...(s.lifecycle === "exam_scoped"
                        ? [
                            {
                              label: "Promote to library",
                              icon: <Send size={14} />,
                              onClick: () => handlePromote(s),
                            },
                          ]
                        : []),
                      ...(s.lifecycle !== "archived"
                        ? [
                            {
                              label: "Archive",
                              icon: <Trash2 size={14} />,
                              onClick: () => crud.setArchiveTarget(s),
                              variant: "danger" as const,
                            },
                          ]
                        : []),
                    ]}
                  />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* View sheet (read-only preview) */}
      <RightPullSheet
        open={!!viewTarget}
        title={viewTarget?.title || "Stimulus"}
        onClose={() => setViewTarget(null)}
      >
        {viewTarget && (
          <div className="space-y-3">
            {viewTarget.source && (
              <p className="text-[11px] text-[var(--muted-foreground)]">
                Sumber: {viewTarget.source}
              </p>
            )}
            <div className="rounded-lg border border-[var(--border)] bg-[var(--accent)] p-3">
              <RenderedContent html={viewTarget.content} />
            </div>
          </div>
        )}
      </RightPullSheet>

      {/* Create sheet */}
      <RightPullSheet
        open={crud.showCreate}
        title="Add Stimulus"
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
          <div>
            <label className="mb-1 block text-[11px] font-medium text-[var(--muted-foreground)]">
              Content (markdown supported)
            </label>
            <textarea
              value={createForm.content}
              onChange={(e) =>
                setCreateForm({ ...createForm, content: e.target.value })
              }
              rows={10}
              className="w-full rounded-lg border border-[var(--border)] bg-[var(--background)] p-3 text-[13px] text-[var(--foreground)] outline-none focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)]"
            />
            {crud.fieldErrors.content && (
              <p className="mt-1 text-[11px] text-[var(--danger)]">
                {crud.fieldErrors.content}
              </p>
            )}
          </div>
          <InputField
            label="Source (optional)"
            value={createForm.source}
            onChange={(e) =>
              setCreateForm({ ...createForm, source: e.target.value })
            }
          />
          <SelectField
            label="Lifecycle"
            value={createForm.lifecycle}
            onChange={(v) =>
              setCreateForm({ ...createForm, lifecycle: v as "shared" | "exam_scoped" })
            }
            options={[
              { value: "shared", label: "Shared (library-wide)" },
            ]}
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
              Save
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit sheet */}
      <RightPullSheet
        open={!!crud.editTarget}
        title="Edit Stimulus"
        onClose={() => crud.setEditTarget(null)}
      >
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="Title"
            value={editForm.title}
            onChange={(e) =>
              setEditForm({ ...editForm, title: e.target.value })
            }
            error={crud.fieldErrors.title}
          />
          <div>
            <label className="mb-1 block text-[11px] font-medium text-[var(--muted-foreground)]">
              Content
            </label>
            <textarea
              value={editForm.content}
              onChange={(e) =>
                setEditForm({ ...editForm, content: e.target.value })
              }
              rows={10}
              className="w-full rounded-lg border border-[var(--border)] bg-[var(--background)] p-3 text-[13px] text-[var(--foreground)] outline-none focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)]"
            />
          </div>
          <InputField
            label="Source"
            value={editForm.source}
            onChange={(e) =>
              setEditForm({ ...editForm, source: e.target.value })
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
        title="Archive Stimulus?"
        description={`Stimulus "${crud.archiveTarget?.title}" akan diarsipkan. Soal yang sudah pakai (snapshot) tidak terpengaruh.`}
        confirmLabel="Archive"
        destructive
        loading={crud.archiving}
        onConfirm={() => crud.archiveTarget && crud.handleArchive(crud.archiveTarget.id)}
        onCancel={() => crud.setArchiveTarget(null)}
      />
    </>
  );
}

function EmptyState({ lifecycle }: { lifecycle: string }) {
  const messages: Record<string, { title: string; sub: string }> = {
    shared: {
      title: "Belum ada stimulus di library",
      sub: "Buat stimulus untuk dibagikan ke seluruh tenant, atau promote stimulus exam-scoped ke library.",
    },
    exam_scoped: {
      title: "Belum ada stimulus exam-scoped",
      sub: "Stimulus exam-scoped dibuat dari halaman exam (inline) — bukan dari library ini.",
    },
    archived: {
      title: "Tidak ada stimulus diarsipkan",
      sub: "Stimulus yang diarsipkan akan muncul di sini.",
    },
    all: {
      title: "Belum ada stimulus",
      sub: "Buat stimulus pertama untuk mulai.",
    },
  };
  const msg = messages[lifecycle] ?? messages.all;
  return (
    <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
      <Library size={24} className="text-[var(--muted-foreground)] mb-2" />
      <p className="text-[13px] font-semibold text-[var(--foreground)]">
        {msg.title}
      </p>
      <p className="text-[11px] text-[var(--muted-foreground)] mt-1">{msg.sub}</p>
    </div>
  );
}
