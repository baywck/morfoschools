"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listTeachers, archiveTeacher, createTeacher, updateTeacher, type Teacher, listUsers, type User } from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { GraduationCap, Trash2, Pencil, Plus } from "lucide-react";
import { cn } from "@/lib/cn";

export default function TeachersPage() {
  const { toast } = useToast();
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [users, setUsers] = useState<User[]>([]);
  
  // Create sheet
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createForm, setCreateForm] = useState({ userId: "", employeeId: "", specialization: "" });
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<Teacher | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ employeeId: "", specialization: "", status: "" });

  const [teacherToArchive, setTeacherToArchive] = useState<Teacher | null>(null);

  async function load() {
    setLoading(true);
    const [res, usersRes] = await Promise.all([
      listTeachers({ search: search || undefined }),
      listUsers()
    ]);
    if (res.data) {
      setTeachers(res.data.data);
      setTotal(res.data.pagination.total);
    }
    if (usersRes.data) {
      setUsers(usersRes.data.data);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createTeacher(createForm);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Teacher created" });
    setShowCreate(false);
    setCreateForm({ userId: "", employeeId: "", specialization: "" });
    setCreating(false);
    load();
  }

  function openEdit(teacher: Teacher) {
    setEditTarget(teacher);
    setEditForm({ 
      employeeId: teacher.employeeId || "", 
      specialization: teacher.specialization || "", 
      status: teacher.status 
    });
    setFieldErrors({});
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setFieldErrors({});
    setEditing(true);
    const res = await updateTeacher(editTarget.id, editForm);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    toast({ tone: "success", title: "Teacher updated" });
    setEditTarget(null);
    setEditing(false);
    load();
  }

  async function handleArchive(id: string) {
    const res = await archiveTeacher(id);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Teacher archived" });
    setTeacherToArchive(null);
    load();
  }

  const userOptions = users.map(u => ({ value: u.id, label: `${u.displayName} (${u.email})` }));

  return (
    <>
    <PageShell
      title="Teachers"
      subtitle={`${total} teacher${total !== 1 ? "s" : ""}`}
      search={{ value: search, onChange: setSearch }}
      onAdd={() => setShowCreate(true)}
      addLabel="Add Teacher"
    >
      <ConfirmDialog
        open={!!teacherToArchive}
        onCancel={() => setTeacherToArchive(null)}
        onConfirm={() => teacherToArchive && handleArchive(teacherToArchive.id)}
        title="Archive Teacher"
        description={`Are you sure you want to archive ${teacherToArchive?.displayName}? This action can be undone later.`}
        confirmLabel="Archive Teacher"
        destructive
      />

      {loading ? (
        <div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}</div>
      ) : teachers.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
          <GraduationCap size={24} className="text-[var(--muted-foreground)] mb-2" />
          <p className="text-[13px] font-semibold text-[var(--foreground)]">No teachers yet</p>
          <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Register users as teachers from the Users module.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
          <div className="divide-y divide-[var(--border)]">
            {teachers.map((t) => (
              <div key={t.id} className="flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--info-soft)] text-[var(--info)]">
                  <GraduationCap size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{t.displayName}</p>
                  <p className="text-[11px] text-[var(--muted-foreground)]">{t.specialization || t.email}</p>
                </div>
                {t.employeeId && <span className="text-[10px] text-[var(--muted-foreground)] font-mono">{t.employeeId}</span>}
                <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", t.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{t.status}</span>
                <RowActions
                  actions={[
                    { label: "Edit", icon: <Pencil size={14} />, onClick: () => openEdit(t) },
                    { label: "Archive", icon: <Trash2 size={14} />, onClick: () => setTeacherToArchive(t), variant: "danger" }
                  ]}
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </PageShell>

    {/* Create Sheet */}
    <RightPullSheet open={showCreate} title="Add Teacher" onClose={() => setShowCreate(false)}>
      <form onSubmit={handleCreate} className="space-y-3">
        <SelectField
          label="User"
          value={createForm.userId}
          onChange={(val) => setCreateForm({ ...createForm, userId: val })}
          error={fieldErrors.userId}
          options={userOptions}
        />
        <InputField
          label="Employee ID"
          value={createForm.employeeId}
          onChange={(e) => setCreateForm({ ...createForm, employeeId: e.target.value })}
          error={fieldErrors.employeeId}
        />
        <InputField
          label="Specialization"
          value={createForm.specialization}
          onChange={(e) => setCreateForm({ ...createForm, specialization: e.target.value })}
          error={fieldErrors.specialization}
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
    <RightPullSheet open={!!editTarget} title="Edit Teacher" onClose={() => setEditTarget(null)}>
      <form onSubmit={handleEdit} className="space-y-3">
        <InputField
          label="Employee ID"
          value={editForm.employeeId}
          onChange={(e) => setEditForm({ ...editForm, employeeId: e.target.value })}
          error={fieldErrors.employeeId}
        />
        <InputField
          label="Specialization"
          value={editForm.specialization}
          onChange={(e) => setEditForm({ ...editForm, specialization: e.target.value })}
          error={fieldErrors.specialization}
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
    </>
  );
}
