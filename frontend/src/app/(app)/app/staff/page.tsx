"use client";

import { useState } from "react";
import { useCRUD } from "@/lib/use-crud";
import { listStaff, archiveStaff, restoreStaff, createStaffFull, updateStaff, updateUser, type Staff } from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { Briefcase, Trash2, Pencil, Plus, RotateCcw } from "lucide-react";
import { cn } from "@/lib/cn";
import { useToast } from "@/components/ui/toast";

export default function StaffPage() {
  const { toast } = useToast();

  const crud = useCRUD<Staff>({
    name: "Staff",
    list: listStaff,
    create: createStaffFull,
    update: updateStaff,
    archive: archiveStaff,
    restore: restoreStaff,
  });

  const [createForm, setCreateForm] = useState({ displayName: "", email: "", password: "", employeeId: "", department: "", position: "" });
  const [editForm, setEditForm] = useState({ employeeId: "", department: "", position: "", status: "", email: "", password: "" });

  function openEdit(staff: Staff) {
    crud.setEditTarget(staff);
    crud.setFieldErrors({});
    setEditForm({
      employeeId: staff.employeeId || "",
      department: staff.department || "",
      position: staff.position || "",
      status: staff.status,
      email: staff.email || "",
      password: "",
    });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const success = await crud.handleCreate(createForm);
    if (success) setCreateForm({ displayName: "", email: "", password: "", employeeId: "", department: "", position: "" });
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    crud.setFieldErrors({});

    // Update user-level fields (email, password)
    const userUpdate: Record<string, string> = {};
    if (editForm.email && editForm.email !== crud.editTarget.email) userUpdate.email = editForm.email;
    if (editForm.password) userUpdate.password = editForm.password;
    if (Object.keys(userUpdate).length > 0) {
      const userRes = await updateUser(crud.editTarget.userId, userUpdate);
      if (userRes.error) {
        if (userRes.error.fields) crud.setFieldErrors(userRes.error.fields);
        else toast({ tone: "error", title: "Failed", description: userRes.error.message });
        return;
      }
    }

    await crud.handleEdit(crud.editTarget.id, {
      employeeId: editForm.employeeId,
      department: editForm.department,
      position: editForm.position,
      status: editForm.status,
    });
  }

  return (
    <>
      <PageShell
        title="Staff"
        subtitle={`${crud.total} staff member${crud.total !== 1 ? "s" : ""}`}
        search={{ value: crud.search, onChange: crud.setSearch, placeholder: "Search staff..." }}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Staff"
      >
        {crud.loading ? (
          <div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}</div>
        ) : crud.items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <Briefcase size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No staff yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Add staff members to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((s) => (
                <div key={s.id} className="flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--warning-soft)] text-[var(--warning)]">
                    <Briefcase size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{s.displayName}</p>
                    <p className="text-[11px] text-[var(--muted-foreground)]">{s.position || s.department || s.email}</p>
                  </div>
                  {s.employeeId && <span className="text-[10px] text-[var(--muted-foreground)] font-mono">{s.employeeId}</span>}
                  <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", s.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{s.status}</span>
                  <RowActions actions={
                    s.status === "archived"
                      ? [{ label: "Restore", icon: <RotateCcw size={14} />, onClick: () => crud.handleRestore(s.id) }]
                      : [
                          { label: "Edit", icon: <Pencil size={14} />, onClick: () => openEdit(s) },
                          { label: "Archive", icon: <Trash2 size={14} />, onClick: () => crud.setArchiveTarget(s), variant: "danger" },
                        ]
                  } />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet open={crud.showCreate} title="Add Staff" onClose={() => crud.setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField label="Display Name" value={createForm.displayName} onChange={(e) => setCreateForm({ ...createForm, displayName: e.target.value })} error={crud.fieldErrors.displayName} />
          <InputField label="Email" value={createForm.email} onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })} error={crud.fieldErrors.email} />
          <InputField label="Password" type="password" value={createForm.password} onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })} error={crud.fieldErrors.password} />
          <InputField label="Employee ID (optional)" value={createForm.employeeId} onChange={(e) => setCreateForm({ ...createForm, employeeId: e.target.value })} error={crud.fieldErrors.employeeId} />
          <InputField label="Department (optional)" value={createForm.department} onChange={(e) => setCreateForm({ ...createForm, department: e.target.value })} error={crud.fieldErrors.department} />
          <InputField label="Position (optional)" value={createForm.position} onChange={(e) => setCreateForm({ ...createForm, position: e.target.value })} error={crud.fieldErrors.position} />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => crud.setShowCreate(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
            <button type="submit" disabled={crud.creating} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {crud.creating && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit Sheet */}
      <RightPullSheet open={!!crud.editTarget} title="Edit Staff" onClose={() => crud.setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField label="Email" type="email" value={editForm.email} onChange={(e) => setEditForm({ ...editForm, email: e.target.value })} error={crud.fieldErrors.email} />
          <InputField label="New Password (leave blank to keep)" type="password" value={editForm.password} onChange={(e) => setEditForm({ ...editForm, password: e.target.value })} error={crud.fieldErrors.password} />
          <InputField label="Employee ID" value={editForm.employeeId} onChange={(e) => setEditForm({ ...editForm, employeeId: e.target.value })} error={crud.fieldErrors.employeeId} />
          <InputField label="Department" value={editForm.department} onChange={(e) => setEditForm({ ...editForm, department: e.target.value })} error={crud.fieldErrors.department} />
          <InputField label="Position" value={editForm.position} onChange={(e) => setEditForm({ ...editForm, position: e.target.value })} error={crud.fieldErrors.position} />
          <SelectField label="Status" value={editForm.status} onChange={(val) => setEditForm({ ...editForm, status: val })} options={[{ value: "active", label: "Active" }, { value: "inactive", label: "Inactive" }, { value: "archived", label: "Archived" }]} />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => crud.setEditTarget(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
            <button type="submit" disabled={crud.editing} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {crud.editing ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : "Save Changes"}
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Archive Confirm */}
      <ConfirmDialog
        open={!!crud.archiveTarget}
        title="Archive Staff"
        description={`Are you sure you want to archive "${crud.archiveTarget?.displayName}"?`}
        confirmLabel="Archive"
        destructive
        loading={crud.archiving}
        onConfirm={() => crud.archiveTarget && crud.handleArchive(crud.archiveTarget.id)}
        onCancel={() => crud.setArchiveTarget(null)}
      />
    </>
  );
}
