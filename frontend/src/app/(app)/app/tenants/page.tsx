"use client";

import { useState, useEffect } from "react";
import { useAuth } from "@/lib/auth-provider";
import { useToast } from "@/components/ui/toast";
import { listTenants, createTenant, archiveTenant, switchTenant, type Tenant } from "@/lib/modules-api";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/text-field";
import { Skeleton } from "@/components/ui/skeleton";
import { Plus, Search, Building2, Trash2, ArrowRightLeft } from "lucide-react";
import { cn } from "@/lib/cn";

export default function TenantsPage() {
  const { refresh } = useAuth();
  const { toast } = useToast();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newCode, setNewCode] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  async function load() {
    setLoading(true);
    const res = await listTenants({ search: search || undefined });
    if (res.data) {
      setTenants(res.data.data);
      setTotal(res.data.pagination.total);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createTenant({ name: newName, code: newCode });
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Tenant created" });
    setShowCreate(false);
    setNewName("");
    setNewCode("");
    setCreating(false);
    load();
  }

  async function handleArchive(id: string) {
    const res = await archiveTenant(id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Tenant archived" });
    load();
  }

  async function handleSwitch(id: string) {
    const res = await switchTenant(id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Switched tenant" });
    await refresh();
  }

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight">Tenants</h2>
          <p className="text-[12px] text-[var(--muted-foreground)] mt-0.5">{total} school{total !== 1 ? "s" : ""} registered</p>
        </div>
        <Button size="sm" onClick={() => setShowCreate(true)}>
          <Plus size={14} /> Add Tenant
        </Button>
      </div>

      {/* Search */}
      <div className="max-w-xs">
        <TextField
          label="Search tenants..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          prefix={<Search size={15} />}
        />
      </div>

      {/* Create form */}
      {showCreate && (
        <form onSubmit={handleCreate} className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-5 space-y-3">
          <p className="text-[12px] font-semibold text-[var(--foreground)]">New Tenant</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <TextField label="School Name" value={newName} onChange={(e) => setNewName(e.target.value)} error={fieldErrors.name} prefix={<Building2 size={15} />} />
            <TextField label="Code" value={newCode} onChange={(e) => setNewCode(e.target.value)} error={fieldErrors.code} />
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
      ) : tenants.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
          <Building2 size={24} className="text-[var(--muted-foreground)] mb-2" />
          <p className="text-[13px] font-semibold text-[var(--foreground)]">No tenants yet</p>
          <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Create your first school tenant to get started.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
          <div className="divide-y divide-[var(--border)]">
            {tenants.map((t) => (
              <div key={t.id} className="flex items-center gap-4 px-5 py-3.5 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
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
                <div className="flex items-center gap-1">
                  <Button variant="ghost" size="sm" onClick={() => handleSwitch(t.id)} title="Switch to this tenant">
                    <ArrowRightLeft size={13} />
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => handleArchive(t.id)} title="Archive">
                    <Trash2 size={13} />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
