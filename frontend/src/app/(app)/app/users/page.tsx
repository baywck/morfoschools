"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listUsers, createUser, archiveUser, type User } from "@/lib/modules-api";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/text-field";
import { Skeleton } from "@/components/ui/skeleton";
import { Plus, Search, Users, Trash2, Mail, Lock, User as UserIcon } from "lucide-react";
import { cn } from "@/lib/cn";

export default function UsersPage() {
  const { toast } = useToast();
  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ email: "", displayName: "", password: "", roleSlug: "" });
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  async function load() {
    setLoading(true);
    const res = await listUsers({ search: search || undefined });
    if (res.data) {
      setUsers(res.data.data);
      setTotal(res.data.pagination.total);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createUser(form);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "User created" });
    setShowCreate(false);
    setForm({ email: "", displayName: "", password: "", roleSlug: "" });
    setCreating(false);
    load();
  }

  async function handleArchive(id: string) {
    const res = await archiveUser(id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "User archived" });
    load();
  }

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight">Users</h2>
          <p className="text-[12px] text-[var(--muted-foreground)] mt-0.5">{total} user{total !== 1 ? "s" : ""} in this tenant</p>
        </div>
        <Button size="sm" onClick={() => setShowCreate(true)}>
          <Plus size={14} /> Add User
        </Button>
      </div>

      {/* Search */}
      <div className="max-w-xs">
        <TextField label="Search users..." value={search} onChange={(e) => setSearch(e.target.value)} prefix={<Search size={15} />} />
      </div>

      {/* Create form */}
      {showCreate && (
        <form onSubmit={handleCreate} className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-5 space-y-3">
          <p className="text-[12px] font-semibold text-[var(--foreground)]">New User</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <TextField label="Display Name" value={form.displayName} onChange={(e) => setForm({ ...form, displayName: e.target.value })} error={fieldErrors.displayName} prefix={<UserIcon size={15} />} />
            <TextField label="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} error={fieldErrors.email} prefix={<Mail size={15} />} />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <TextField label="Password" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} error={fieldErrors.password} prefix={<Lock size={15} />} />
            <TextField label="Role (slug)" value={form.roleSlug} onChange={(e) => setForm({ ...form, roleSlug: e.target.value })} />
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="ghost" size="sm" type="button" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button size="sm" type="submit" loading={creating}><Plus size={14} /> Create</Button>
          </div>
        </form>
      )}

      {/* List */}
      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}
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
              <div key={u.id} className="flex items-center gap-4 px-5 py-3.5 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--primary)] text-[10px] font-bold text-[var(--primary-foreground)]">
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
                <Button variant="ghost" size="sm" onClick={() => handleArchive(u.id)} title="Archive">
                  <Trash2 size={13} />
                </Button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
