"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth-provider";
import { useCRUD } from "@/lib/use-crud";
import { useToast } from "@/components/ui/toast";
import { listTenants, createTenant, updateTenant, archiveTenant, switchTenant, type Tenant } from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, Building2, Trash2, ArrowRightLeft, Pencil } from "lucide-react";
import { cn } from "@/lib/cn";

export default function TenantsPage() {
  const { refresh } = useAuth();
  const { toast } = useToast();

  const crud = useCRUD<Tenant>({
    name: "Tenant",
    list: listTenants,
    create: createTenant,
    update: updateTenant,
    archive: archiveTenant,
  });

  const [createForm, setCreateForm] = useState({ name: "", code: "" });
  const [editForm, setEditForm] = useState({ name: "", status: "" });

  // Switch tenant
  const [switchTarget, setSwitchTarget] = useState<Tenant | null>(null);
  const [switching, setSwitching] = useState(false);

  function openEdit(tenant: Tenant) {
    crud.setEditTarget(tenant);
    crud.setFieldErrors({});
    setEditForm({ name: tenant.name, status: tenant.status });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const success = await crud.handleCreate(createForm);
    if (success) setCreateForm({ name: "", code: "" });
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    await crud.handleEdit(crud.editTarget.id, editForm);
  }

  async function confirmSwitch() {
    if (!switchTarget) return;
    setSwitching(true);
    const res = await switchTenant(switchTarget.id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
    } else {
      toast({ tone: "success", title: `Switched to ${switchTarget.name}` });
      await refresh();
    }
    setSwitching(false);
    setSwitchTarget(null);
  }

  return (
    <>
      <PageShell
        title="Tenants"
        subtitle={`${crud.total} school${crud.total !== 1 ? "s" : ""} registered`}
        search={{ value: crud.search, onChange: crud.setSearch, placeholder: "Search tenants..." }}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Tenant"
      >
        {crud.loading ? (
          <div className="space-y-2">
            {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : crud.items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <Building2 size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No tenants yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Create your first school tenant to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((t) => (
                <div key={t.id} className="flex items-center gap-3 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
                    <Building2 size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{t.name}</p>
                    <p className="text-[11px] text-[var(--muted-foreground)]">{t.code}</p>
                  </div>
                  <span className={cn(
                    "rounded-md px-2 py-0.5 text-[10px] font-medium",
                    t.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  )}>
                    {t.status}
                  </span>
                  <RowActions actions={[
                    { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(t) },
                    { label: "Switch", icon: <ArrowRightLeft size={13} />, onClick: () => setSwitchTarget(t) },
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => crud.setArchiveTarget(t), variant: "danger" },
                  ]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet open={crud.showCreate} title="Add Tenant" onClose={() => crud.setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="School Name"
            value={createForm.name}
            onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
            error={crud.fieldErrors.name}
            prefix={<Building2 size={14} />}
          />
          <InputField
            label="Code"
            value={createForm.code}
            onChange={(e) => setCreateForm({ ...createForm, code: e.target.value })}
            error={crud.fieldErrors.code}
            helperText="Unique identifier (e.g. sman1-jkt)"
          />
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
      <RightPullSheet open={!!crud.editTarget} title="Edit Tenant" onClose={() => crud.setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="School Name"
            value={editForm.name}
            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
            error={crud.fieldErrors.name}
            prefix={<Building2 size={14} />}
          />
          <SelectField
            label="Status"
            value={editForm.status}
            onChange={(val) => setEditForm({ ...editForm, status: val })}
            options={[
              { value: "active", label: "Active" },
              { value: "inactive", label: "Inactive" },
              { value: "archived", label: "Archived" },
            ]}
          />
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
        title="Archive Tenant"
        description={`Are you sure you want to archive "${crud.archiveTarget?.name}"? This will suspend access for all users in this tenant.`}
        confirmLabel="Archive"
        destructive
        loading={crud.archiving}
        onConfirm={() => crud.archiveTarget && crud.handleArchive(crud.archiveTarget.id)}
        onCancel={() => crud.setArchiveTarget(null)}
      />

      {/* Switch Confirm */}
      <ConfirmDialog
        open={!!switchTarget}
        title="Switch Tenant"
        description={`Switch your active context to "${switchTarget?.name}"? You will see data from this tenant.`}
        confirmLabel="Switch"
        loading={switching}
        onConfirm={confirmSwitch}
        onCancel={() => setSwitchTarget(null)}
      />
    </>
  );
}
