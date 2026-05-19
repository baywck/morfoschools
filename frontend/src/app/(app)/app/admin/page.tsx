"use client";

import { useState } from "react";
import { useCRUD } from "@/lib/use-crud";
import { listUsers, createUser, updateUser, archiveUser, restoreUser, type User } from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, Trash2, Pencil, Users, Mail, Lock, User as UserIcon, RotateCcw } from "lucide-react";
import { cn } from "@/lib/cn";
import type { ApiResponse } from "@/lib/api-client";

// Wrap listUsers to always filter by school_admin role
function listAdmins(params?: { search?: string; page?: number }): Promise<ApiResponse<any>> {
  return listUsers({ ...params, role: "school_admin" });
}

// Wrap createUser to always assign school_admin role
function createAdmin(data: { email: string; displayName: string; password: string }): Promise<ApiResponse<any>> {
  return createUser({ ...data, roleSlug: "school_admin" });
}

export default function AdminPage() {
  const crud = useCRUD<User>({
    name: "Admin",
    list: listAdmins,
    create: createAdmin,
    update: updateUser,
    archive: archiveUser,
    restore: (id: string) => restoreUser(id),
  });

  const [createForm, setCreateForm] = useState({ email: "", displayName: "", password: "" });
  const [editForm, setEditForm] = useState({ displayName: "", status: "", email: "", password: "" });

  function openEdit(user: User) {
    crud.setEditTarget(user);
    crud.setFieldErrors({});
    setEditForm({ displayName: user.displayName, status: user.status, email: user.email, password: "" });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const success = await crud.handleCreate(createForm);
    if (success) setCreateForm({ email: "", displayName: "", password: "" });
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    const data: Record<string, string> = { displayName: editForm.displayName, status: editForm.status };
    if (editForm.email && editForm.email !== crud.editTarget.email) data.email = editForm.email;
    if (editForm.password) data.password = editForm.password;
    await crud.handleEdit(crud.editTarget.id, data);
  }

  return (
    <>
      <PageShell
        title="Admin"
        subtitle={`${crud.total} admin${crud.total !== 1 ? "s" : ""} in this tenant`}
        search={{ value: crud.search, onChange: crud.setSearch, placeholder: "Search admins..." }}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Admin"
      >
        {crud.loading ? (
          <div className="space-y-2">
            {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : crud.items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <Users size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No admins yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Add admins to this tenant to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((u) => (
                <div key={u.id} className="flex items-center gap-3 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-[var(--primary)] text-[10px] font-bold text-[var(--primary-foreground)]">
                    {u.displayName.charAt(0)}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{u.displayName}</p>
                    <p className="text-[11px] text-[var(--muted-foreground)]">{u.email}</p>
                  </div>
                  <span className={cn(
                    "rounded-md px-2 py-0.5 text-[10px] font-medium",
                    u.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  )}>
                    {u.status}
                  </span>
                  <RowActions actions={
                    u.status === "archived"
                      ? [
                          { label: "Restore", icon: <RotateCcw size={13} />, onClick: () => crud.handleRestore(u.id) },
                        ]
                      : [
                          { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(u) },
                          { label: "Archive", icon: <Trash2 size={13} />, onClick: () => crud.setArchiveTarget(u), variant: "danger" },
                        ]
                  } />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create */}
      <RightPullSheet open={crud.showCreate} title="Add Admin" onClose={() => crud.setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField label="Display Name" value={createForm.displayName} onChange={(e) => setCreateForm({ ...createForm, displayName: e.target.value })} error={crud.fieldErrors.displayName} prefix={<UserIcon size={14} />} />
          <InputField label="Email" value={createForm.email} onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })} error={crud.fieldErrors.email} prefix={<Mail size={14} />} />
          <InputField label="Password" type="password" value={createForm.password} onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })} error={crud.fieldErrors.password} prefix={<Lock size={14} />} />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => crud.setShowCreate(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
            <button type="submit" disabled={crud.creating} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {crud.creating && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit */}
      <RightPullSheet open={!!crud.editTarget} title="Edit Admin" onClose={() => crud.setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField label="Display Name" value={editForm.displayName} onChange={(e) => setEditForm({ ...editForm, displayName: e.target.value })} error={crud.fieldErrors.displayName} prefix={<UserIcon size={14} />} />
          <InputField label="Email" type="email" value={editForm.email} onChange={(e) => setEditForm({ ...editForm, email: e.target.value })} error={crud.fieldErrors.email} prefix={<Mail size={14} />} />
          <InputField label="New Password (leave blank to keep)" type="password" value={editForm.password} onChange={(e) => setEditForm({ ...editForm, password: e.target.value })} error={crud.fieldErrors.password} prefix={<Lock size={14} />} />
          <SelectField label="Status" value={editForm.status} onChange={(v) => setEditForm({ ...editForm, status: v })} options={[{ value: "active", label: "Active" }, { value: "suspended", label: "Suspended" }]} />
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
        title="Archive Admin"
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
