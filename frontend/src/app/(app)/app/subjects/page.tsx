"use client";

import { useState, useEffect } from "react";
import { useCRUD } from "@/lib/use-crud";
import { listSubjects, createSubject, updateSubject, archiveSubject, listSubjectTeachers, type Subject, type SubjectTeacher } from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, BookOpen, Trash2, Pencil } from "lucide-react";
import { cn } from "@/lib/cn";

export default function SubjectsPage() {
  const crud = useCRUD<Subject>({
    name: "Subject",
    list: listSubjects,
    create: createSubject,
    update: updateSubject,
    archive: archiveSubject,
  });

  // Load teachers per subject for list display
  const [subjectTeachersMap, setSubjectTeachersMap] = useState<Record<string, SubjectTeacher[]>>({});

  useEffect(() => {
    if (crud.items.length === 0) return;
    async function loadTeachers() {
      const map: Record<string, SubjectTeacher[]> = {};
      await Promise.all(crud.items.map(async (s) => {
        const res = await listSubjectTeachers(s.id);
        if (res.data) map[s.id] = res.data.data;
      }));
      setSubjectTeachersMap(map);
    }
    loadTeachers();
  }, [crud.items]);

  // Create form
  const [createForm, setCreateForm] = useState({ code: "", name: "", description: "" });

  // Edit form
  const [editForm, setEditForm] = useState({ name: "", description: "", status: "" });

  function openEdit(subject: Subject) {
    crud.setEditTarget(subject);
    crud.setFieldErrors({});
    setEditForm({ name: subject.name, description: subject.description || "", status: subject.status });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const success = await crud.handleCreate({ ...createForm, description: createForm.description || undefined });
    if (success) setCreateForm({ code: "", name: "", description: "" });
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    await crud.handleEdit(crud.editTarget.id, {
      name: editForm.name,
      description: editForm.description || undefined,
      status: editForm.status,
    });
  }

  async function confirmArchive() {
    if (!crud.archiveTarget) return;
    await crud.handleArchive(crud.archiveTarget.id);
  }

  return (
    <>
      <PageShell
        title="Subjects"
        subtitle={`${crud.total} subject${crud.total !== 1 ? "s" : ""} registered`}
        search={{ value: crud.search, onChange: crud.setSearch, placeholder: "Search subjects..." }}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Subject"
      >
        {crud.loading ? (
          <div className="space-y-2">
            {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : crud.items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <BookOpen size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No subjects yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Create your first subject to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((s) => (
                <div key={s.id} className="flex items-center gap-3 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
                    <BookOpen size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{s.name}</p>
                    <p className="text-[11px] text-[var(--muted-foreground)]">{s.code}</p>
                  </div>
                  {/* Assigned teachers — desktop only */}
                  {subjectTeachersMap[s.id]?.length > 0 && (
                    <div className="hidden md:flex items-center gap-1 shrink-0">
                      {subjectTeachersMap[s.id].slice(0, 3).map((t) => (
                        <span key={t.id} className="inline-flex items-center rounded-md bg-[var(--info-soft)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--info)]">
                          {t.displayName.split(" ")[0]}
                        </span>
                      ))}
                      {subjectTeachersMap[s.id].length > 3 && (
                        <span className="text-[10px] text-[var(--muted-foreground)]">+{subjectTeachersMap[s.id].length - 3}</span>
                      )}
                    </div>
                  )}
                  <span className={cn(
                    "rounded-md px-2 py-0.5 text-[10px] font-medium",
                    s.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  )}>
                    {s.status}
                  </span>
                  <RowActions actions={[
                    { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(s) },
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => crud.setArchiveTarget(s), variant: "danger" },
                  ]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet open={crud.showCreate} title="Add Subject" onClose={() => crud.setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="Subject Code"
            value={createForm.code}
            onChange={(e) => setCreateForm({ ...createForm, code: e.target.value })}
            error={crud.fieldErrors.code}
            helperText="Unique identifier (e.g. MATH-101)"
          />
          <InputField
            label="Subject Name"
            value={createForm.name}
            onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
            error={crud.fieldErrors.name}
            prefix={<BookOpen size={14} />}
          />
          <InputField
            label="Description"
            value={createForm.description}
            onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })}
            error={crud.fieldErrors.description}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => crud.setShowCreate(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={crud.creating} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {crud.creating && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit Sheet */}
      <RightPullSheet open={!!crud.editTarget} title="Edit Subject" onClose={() => crud.setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="Subject Name"
            value={editForm.name}
            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
            error={crud.fieldErrors.name}
            prefix={<BookOpen size={14} />}
          />
          <InputField
            label="Description"
            value={editForm.description}
            onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
            error={crud.fieldErrors.description}
          />
          <SelectField
            label="Status"
            value={editForm.status}
            onChange={(val) => setEditForm({ ...editForm, status: val })}
            options={[
              { value: "active", label: "Active" },
              { value: "inactive", label: "Inactive" },
              { value: "archived", label: "Archived" }
            ]}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => crud.setEditTarget(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={crud.editing} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {crud.editing ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : "Save Changes"}
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Archive Confirm */}
      <ConfirmDialog
        open={!!crud.archiveTarget}
        title="Archive Subject"
        description={`Are you sure you want to archive "${crud.archiveTarget?.name}"?`}
        confirmLabel="Archive"
        destructive
        loading={crud.archiving}
        onConfirm={confirmArchive}
        onCancel={() => crud.setArchiveTarget(null)}
      />
    </>
  );
}
