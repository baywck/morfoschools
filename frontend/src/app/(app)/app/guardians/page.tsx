"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listGuardians, createGuardian, archiveGuardian, updateGuardian, type Guardian } from "@/lib/modules-api";
import { Button } from "@/components/ui/button";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { Plus, Heart, Trash2, User, Pencil } from "lucide-react";
import { cn } from "@/lib/cn";

export default function GuardiansPage() {
  const { toast } = useToast();
  const [guardians, setGuardians] = useState<Guardian[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ name: "", phone: "", email: "", relationship: "" });
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<Guardian | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ name: "", phone: "", email: "", relationship: "", status: "" });

  const [guardianToArchive, setGuardianToArchive] = useState<Guardian | null>(null);

  async function load() {
    setLoading(true);
    const res = await listGuardians({ search: search || undefined });
    if (res.data) {
      setGuardians(res.data.data);
      setTotal(res.data.pagination.total);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createGuardian(form);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Guardian created" });
    setShowCreate(false);
    setForm({ name: "", phone: "", email: "", relationship: "" });
    setCreating(false);
    load();
  }

  function openEdit(guardian: Guardian) {
    setEditTarget(guardian);
    setEditForm({ 
      name: guardian.name || "", 
      phone: guardian.phone || "", 
      email: guardian.email || "", 
      relationship: guardian.relationship || "",
      status: guardian.status 
    });
    setFieldErrors({});
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setFieldErrors({});
    setEditing(true);
    const res = await updateGuardian(editTarget.id, editForm);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    toast({ tone: "success", title: "Guardian updated" });
    setEditTarget(null);
    setEditing(false);
    load();
  }

  async function handleArchive(id: string) {
    const res = await archiveGuardian(id);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Guardian archived" });
    setGuardianToArchive(null);
    load();
  }

  return (
    <>
    <PageShell
      title="Guardians"
      subtitle={`${total} guardian${total !== 1 ? "s" : ""}`}
      search={{ value: search, onChange: setSearch }}
      onAdd={() => setShowCreate(true)}
      addLabel="Add Guardian"
    >
      <RightPullSheet
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="New Guardian"
      >
        <form onSubmit={handleCreate} className="space-y-4 pt-4">
          <InputField label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} error={fieldErrors.name} prefix={<User size={14} />} />
          <InputField label="Relationship" value={form.relationship} onChange={(e) => setForm({ ...form, relationship: e.target.value })} prefix={<Heart size={14} />} />
          <InputField label="Phone" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
          <InputField label="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
          
          <div className="flex gap-2 justify-end pt-4">
            <Button variant="ghost" size="sm" type="button" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button size="sm" type="submit" loading={creating}><Plus size={14} /> Create</Button>
          </div>
        </form>
      </RightPullSheet>

      <ConfirmDialog
        open={!!guardianToArchive}
        onCancel={() => setGuardianToArchive(null)}
        onConfirm={() => guardianToArchive && handleArchive(guardianToArchive.id)}
        title="Archive Guardian"
        description={`Are you sure you want to archive ${guardianToArchive?.name}? This action can be undone later.`}
        confirmLabel="Archive Guardian"
        destructive
      />

      {loading ? (
        <div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}</div>
      ) : guardians.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
          <Heart size={24} className="text-[var(--muted-foreground)] mb-2" />
          <p className="text-[13px] font-semibold text-[var(--foreground)]">No guardians yet</p>
          <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Add guardians and link them to students.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
          <div className="divide-y divide-[var(--border)]">
            {guardians.map((g) => (
              <div key={g.id} className="flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--danger-soft)] text-[var(--danger)]">
                  <Heart size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{g.name}</p>
                  <p className="text-[11px] text-[var(--muted-foreground)]">{[g.relationship, g.phone].filter(Boolean).join(" • ")}</p>
                </div>
                <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", g.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{g.status}</span>
                <RowActions
                  actions={[
                    { label: "Edit", icon: <Pencil size={14} />, onClick: () => openEdit(g) },
                    { label: "Archive", icon: <Trash2 size={14} />, onClick: () => setGuardianToArchive(g), variant: "danger" }
                  ]}
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </PageShell>
    
    <RightPullSheet open={!!editTarget} title="Edit Guardian" onClose={() => setEditTarget(null)}>
      <form onSubmit={handleEdit} className="space-y-4 pt-4">
        <InputField 
          label="Name" 
          value={editForm.name} 
          onChange={(e) => setEditForm({ ...editForm, name: e.target.value })} 
          error={fieldErrors.name} 
          prefix={<User size={14} />} 
        />
        <InputField 
          label="Relationship" 
          value={editForm.relationship} 
          onChange={(e) => setEditForm({ ...editForm, relationship: e.target.value })} 
          error={fieldErrors.relationship}
          prefix={<Heart size={14} />} 
        />
        <InputField 
          label="Phone" 
          value={editForm.phone} 
          onChange={(e) => setEditForm({ ...editForm, phone: e.target.value })} 
          error={fieldErrors.phone}
        />
        <InputField 
          label="Email" 
          type="email" 
          value={editForm.email} 
          onChange={(e) => setEditForm({ ...editForm, email: e.target.value })} 
          error={fieldErrors.email}
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
        
        <div className="flex gap-2 justify-end pt-4">
          <Button variant="ghost" size="sm" type="button" onClick={() => setEditTarget(null)}>Cancel</Button>
          <Button size="sm" type="submit" loading={editing}>Save Changes</Button>
        </div>
      </form>
    </RightPullSheet>
    </>
  );
}
