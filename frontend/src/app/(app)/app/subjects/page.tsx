"use client";

import { useState, useEffect } from "react";
import { useAuth } from "@/lib/auth-provider";
import { useToast } from "@/components/ui/toast";
import { listSubjects, createSubject, updateSubject, archiveSubject, type Subject } from "@/lib/modules-api";
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
  const { toast } = useToast();
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  // Create sheet
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newCode, setNewCode] = useState("");
  const [newDescription, setNewDescription] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<Subject | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ name: "", description: "", status: "" });

  // Confirm dialogs
  const [archiveTarget, setArchiveTarget] = useState<Subject | null>(null);
  const [archiving, setArchiving] = useState(false);

  async function load() {
    setLoading(true);
    const res = await listSubjects({ search: search || undefined });
    if (res.data) {
      setSubjects(res.data.data);
      setTotal(res.data.pagination.total);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createSubject({ name: newName, code: newCode, description: newDescription || undefined });
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Subject created" });
    setShowCreate(false);
    setNewName("");
    setNewCode("");
    setNewDescription("");
    setCreating(false);
    load();
  }

  function openEdit(subject: Subject) {
    setEditTarget(subject);
    setEditForm({ name: subject.name, description: subject.description || "", status: subject.status });
    setFieldErrors({});
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setFieldErrors({});
    setEditing(true);
    const res = await updateSubject(editTarget.id, {
      name: editForm.name,
      description: editForm.description || undefined,
      status: editForm.status
    });
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    toast({ tone: "success", title: "Subject updated" });
    setEditTarget(null);
    setEditing(false);
    load();
  }

  async function confirmArchive() {
    if (!archiveTarget) return;
    setArchiving(true);
    const res = await archiveSubject(archiveTarget.id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
    } else {
      toast({ tone: "success", title: "Subject archived" });
      load();
    }
    setArchiving(false);
    setArchiveTarget(null);
  }

  return (
    <>
      <PageShell
        title="Subjects"
        subtitle={`${total} subject${total !== 1 ? "s" : ""} registered`}
        search={{ value: search, onChange: setSearch, placeholder: "Search subjects..." }}
        onAdd={() => setShowCreate(true)}
        addLabel="Add Subject"
      >
        {/* List */}
        {loading ? (
          <div className="space-y-2">
            {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : subjects.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <BookOpen size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No subjects yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Create your first subject to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {subjects.map((s) => (
                <div key={s.id} className="flex items-center gap-3 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
                    <BookOpen size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{s.name}</p>
                    <p className="text-[11px] text-[var(--muted-foreground)]">{s.code}</p>
                  </div>
                  <span className={cn(
                    "rounded-md px-2 py-0.5 text-[10px] font-medium",
                    s.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  )}>
                    {s.status}
                  </span>
                  <RowActions actions={[
                    { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(s) },
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => setArchiveTarget(s), variant: "danger" },
                  ]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet open={showCreate} title="Add Subject" onClose={() => setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="Subject Code"
            value={newCode}
            onChange={(e) => setNewCode(e.target.value)}
            error={fieldErrors.code}
            helperText="Unique identifier (e.g. MATH-101)"
          />
          <InputField
            label="Subject Name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            error={fieldErrors.name}
            prefix={<BookOpen size={14} />}
          />
          <InputField
            label="Description"
            value={newDescription}
            onChange={(e) => setNewDescription(e.target.value)}
            error={fieldErrors.description}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => setShowCreate(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={creating} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {creating && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit Sheet */}
      <RightPullSheet open={!!editTarget} title="Edit Subject" onClose={() => setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="Subject Name"
            value={editForm.name}
            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
            error={fieldErrors.name}
            prefix={<BookOpen size={14} />}
          />
          <InputField
            label="Description"
            value={editForm.description}
            onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
            error={fieldErrors.description}
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
            <button type="button" onClick={() => setEditTarget(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={editing} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {editing ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : "Save Changes"}
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Archive Confirm */}
      <ConfirmDialog
        open={!!archiveTarget}
        title="Archive Subject"
        description={`Are you sure you want to archive "${archiveTarget?.name}"?`}
        confirmLabel="Archive"
        destructive
        loading={archiving}
        onConfirm={confirmArchive}
        onCancel={() => setArchiveTarget(null)}
      />
    </>
  );
}
