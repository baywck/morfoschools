"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listUsers, createUser, updateUser, archiveUser, listRoles, type User, type Role } from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, Trash2, Pencil, Users, Mail, Lock, User as UserIcon, Shield } from "lucide-react";
import { cn } from "@/lib/cn";

export default function UsersPage() {
  const { toast } = useToast();
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  // Create
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createForm, setCreateForm] = useState({ email: "", displayName: "", password: "", roleSlug: "" });
  const [createErrors, setCreateErrors] = useState<Record<string, string>>({});

  // Edit
  const [editTarget, setEditTarget] = useState<User | null>(null);
  const [editForm, setEditForm] = useState({ displayName: "", status: "" });
  const [editing, setEditing] = useState(false);
  const [editErrors, setEditErrors] = useState<Record<string, string>>({});

  // Archive
  const [archiveTarget, setArchiveTarget] = useState<User | null>(null);
  const [archiving, setArchiving] = useState(false);

  async function load() {
    setLoading(true);
    const [usersRes, rolesRes] = await Promise.all([
      listUsers({ search: search || undefined }),
      listRoles(),
    ]);
    if (usersRes.data) {
      setUsers(usersRes.data.data);
      setTotal(usersRes.data.pagination.total);
    }
    if (rolesRes.data) {
      setRoles(rolesRes.data.data);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreateErrors({});
    setCreating(true);
    const res = await createUser(createForm);
    if (res.error) {
      if (res.error.fields) setCreateErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "User created" });
    setShowCreate(false);
    setCreateForm({ email: "", displayName: "", password: "", roleSlug: "" });
    setCreating(false);
    load();
  }

  function openEdit(user: User) {
    setEditTarget(user);
    setEditForm({ displayName: user.displayName, status: user.status });
    setEditErrors({});
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setEditErrors({});
    setEditing(true);
    const res = await updateUser(editTarget.id, editForm);
    if (res.error) {
      if (res.error.fields) setEditErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    toast({ tone: "success", title: "User updated" });
    setEditTarget(null);
    setEditing(false);
    load();
  }

  async function confirmArchive() {
    if (!archiveTarget) return;
    setArchiving(true);
    const res = await archiveUser(archiveTarget.id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
    } else {
      toast({ tone: "success", title: "User archived" });
      load();
    }
    setArchiving(false);
    setArchiveTarget(null);
  }

  const roleOptions = roles.map((r) => ({ value: r.slug, label: r.name }));
  const statusOptions = [
    { value: "active", label: "Active" },
    { value: "suspended", label: "Suspended" },
  ];

  return (
    <>
      <PageShell
        title="Users"
        subtitle={`${total} user${total !== 1 ? "s" : ""} in this tenant`}
        search={{ value: search, onChange: setSearch, placeholder: "Search users..." }}
        onAdd={() => setShowCreate(true)}
        addLabel="Add User"
      >
        {loading ? (
          <div className="space-y-2">
            {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : users.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <Users size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No users yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Add users to this tenant to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {users.map((u) => (
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
                  <RowActions actions={[
                    { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(u) },
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => setArchiveTarget(u), variant: "danger" },
                  ]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create */}
      <RightPullSheet open={showCreate} title="Add User" onClose={() => setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField label="Display Name" value={createForm.displayName} onChange={(e) => setCreateForm({ ...createForm, displayName: e.target.value })} error={createErrors.displayName} prefix={<UserIcon size={14} />} />
          <InputField label="Email" value={createForm.email} onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })} error={createErrors.email} prefix={<Mail size={14} />} />
          <InputField label="Password" type="password" value={createForm.password} onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })} error={createErrors.password} prefix={<Lock size={14} />} />
          <SelectField label="Role" value={createForm.roleSlug} options={roleOptions} onChange={(v) => setCreateForm({ ...createForm, roleSlug: v })} prefix={<Shield size={14} />} />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => setShowCreate(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
            <button type="submit" disabled={creating} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {creating && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit */}
      <RightPullSheet open={!!editTarget} title="Edit User" onClose={() => setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField label="Display Name" value={editForm.displayName} onChange={(e) => setEditForm({ ...editForm, displayName: e.target.value })} error={editErrors.displayName} prefix={<UserIcon size={14} />} />
          <SelectField label="Status" value={editForm.status} options={statusOptions} onChange={(v) => setEditForm({ ...editForm, status: v })} />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => setEditTarget(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
            <button type="submit" disabled={editing} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {editing && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Pencil size={14} /> Save
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Archive Confirm */}
      <ConfirmDialog
        open={!!archiveTarget}
        title="Archive User"
        description={`Are you sure you want to archive "${archiveTarget?.displayName}"?`}
        confirmLabel="Archive"
        destructive
        loading={archiving}
        onConfirm={confirmArchive}
        onCancel={() => setArchiveTarget(null)}
      />
    </>
  );
}
