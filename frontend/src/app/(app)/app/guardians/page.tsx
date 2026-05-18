"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listGuardians, createGuardian, archiveGuardian, type Guardian } from "@/lib/modules-api";
import { Button } from "@/components/ui/button";
import { InputField } from "@/components/ui/input-field";
import { SearchInput } from "@/components/ui/search-input";
import { Skeleton } from "@/components/ui/skeleton";
import { Plus, Search, Heart, Trash2, User } from "lucide-react";
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

  async function handleArchive(id: string) {
    const res = await archiveGuardian(id);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Guardian archived" });
    load();
  }

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight">Guardians</h2>
          <p className="text-[12px] text-[var(--muted-foreground)] mt-0.5">{total} guardian{total !== 1 ? "s" : ""}</p>
        </div>
        <Button size="sm" onClick={() => setShowCreate(true)}>
          <Plus size={14} /> Add Guardian
        </Button>
      </div>

      <div className="max-w-xs">
        <SearchInput value={search} onChange={setSearch} placeholder="Search guardians..." />
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-5 space-y-3">
          <p className="text-[12px] font-semibold text-[var(--foreground)]">New Guardian</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <InputField label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} error={fieldErrors.name} prefix={<User size={14} />} />
            <InputField label="Relationship" value={form.relationship} onChange={(e) => setForm({ ...form, relationship: e.target.value })} prefix={<Heart size={14} />} />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <InputField label="Phone" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
            <InputField label="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="ghost" size="sm" type="button" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button size="sm" type="submit" loading={creating}><Plus size={14} /> Create</Button>
          </div>
        </form>
      )}

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
              <div key={g.id} className="flex items-center gap-4 px-5 py-3.5 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--danger-soft)] text-[var(--danger)]">
                  <Heart size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{g.name}</p>
                  <p className="text-[11px] text-[var(--muted-foreground)]">{[g.relationship, g.phone].filter(Boolean).join(" • ")}</p>
                </div>
                <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", g.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{g.status}</span>
                <Button variant="ghost" size="sm" onClick={() => handleArchive(g.id)}><Trash2 size={13} /></Button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
