"use client";

import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/lib/auth-provider";
import { BookOpen, DatabaseZap, ExternalLink, Pencil, Plus, Search, Trash2 } from "lucide-react";
import { PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { InputField } from "@/components/ui/input-field";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { RowActions } from "@/components/ui/row-actions";
import { SelectField } from "@/components/ui/select-field";
import { ComboboxField } from "@/components/ui/combobox-field";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import {
  getCurriculumCPReference,
  listCurriculumCPReferences,
  createCurriculumCPElement,
  createCurriculumCPReference,
  deleteCurriculumCPElement,
  seedCurriculumCPReference,
  updateCurriculumCPElement,
  updateCurriculumCPReference,
  type CurriculumCPElement,
  type CurriculumCPReference,
} from "@/lib/modules-api";
import { subjectsForTenant, tenantEnabledPhases, tenantIncludesVocationalSubjects } from "@/lib/tenant-education";

const PHASES = ["a", "b", "c", "d", "e", "f"];

export default function CurriculumCPPage() {
  const { toast } = useToast();
  const { session } = useAuth();
  const [items, setItems] = useState<CurriculumCPReference[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  const [seedOpen, setSeedOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [detail, setDetail] = useState<CurriculumCPReference | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [seeding, setSeeding] = useState(false);
  const canManageCP = !!session?.user.isPlatformAdmin || !!session?.roles.includes("master_admin") || !!session?.roles.includes("platform_admin");
  const tenantPhases = useMemo(() => tenantEnabledPhases(session?.effectiveTenant), [session?.effectiveTenant]);
  const tenantLevelCode = tenantIncludesVocationalSubjects(session?.effectiveTenant) ? "smk" : "sd-sma";
  const tenantLevelName = tenantLevelCode === "smk" ? "SMK/Sederajat" : "SD-SMA/Sederajat";
  const [seedForm, setSeedForm] = useState({ subjectCode: "pendidikan-pancasila", customSubjectCode: "", subjectInput: "Pendidikan Pancasila" });
  const [createForm, setCreateForm] = useState({ levelCode: "sd-sma", levelName: "SD-SMA/Sederajat", subjectCode: "", subjectName: "", phase: "f", generalCp: "", sourceUrl: "" });
  const [newElement, setNewElement] = useState({ name: "", content: "" });

  const [editRef, setEditRef] = useState({ subjectName: "", generalCp: "", status: "active" });
  const [editElements, setEditElements] = useState<CurriculumCPElement[]>([]);

  const subtitle = useMemo(() => `${total} CP reference${total === 1 ? "" : "s"} tersimpan · Fase ${tenantPhases.map((p) => p.toUpperCase()).join(", ")}`, [tenantPhases, total]);
  const tenantSubjectOptions = useMemo(() => subjectsForTenant(session?.effectiveTenant).map((subject) => ({ value: subject.value, label: subject.label })), [session?.effectiveTenant]);

  async function load() {
    setLoading(true);
    const phaseFilter = tenantPhases.join(",");
    const res = await listCurriculumCPReferences({ search, level: "", phase: phaseFilter });
    setLoading(false);
    if (res.error) {
      toast({ tone: "error", title: "Gagal memuat CP", description: res.error.message });
      return;
    }
    setItems(res.data?.data || []);
    setTotal(res.data?.pagination.total || 0);
  }

  useEffect(() => {
    const t = window.setTimeout(load, 250);
    return () => window.clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, tenantPhases.join(",")]);

  async function openDetail(row: CurriculumCPReference) {
    setDetailLoading(true);
    const res = await getCurriculumCPReference(row.id);
    setDetailLoading(false);
    if (res.error || !res.data) {
      toast({ tone: "error", title: "Gagal membuka CP", description: res.error?.message });
      return;
    }
    setDetail(res.data);
    setEditRef({ subjectName: res.data.subjectName, generalCp: res.data.generalCp, status: res.data.status });
    setEditElements(res.data.elements || []);
  }

  async function handleSeed(e: React.FormEvent) {
    e.preventDefault();
    setSeeding(true);
    const subjectCode = seedForm.subjectCode === "__custom__" ? seedForm.customSubjectCode.trim() : seedForm.subjectCode;
    const phases = tenantPhases;
    let last: CurriculumCPReference | undefined;
    for (const ph of phases) {
      const res = await seedCurriculumCPReference({ levelCode: tenantLevelCode, subjectCode, phase: ph });
      if (res.error) {
        setSeeding(false);
        toast({ tone: "error", title: `Seed fase ${ph.toUpperCase()} gagal`, description: res.error.message });
        return;
      }
      last = res.data;
    }
    setSeeding(false);
    toast({ tone: "success", title: "Fase tenant berhasil di-seed", description: last?.subjectName });
    setSeedOpen(false);
    await load();
    if (last) await openDetail(last);
  }

  async function handleCreateManual(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    const res = await createCurriculumCPReference({ ...createForm, elements: newElement.name && newElement.content ? [newElement] : [] });
    setSaving(false);
    if (res.error) {
      toast({ tone: "error", title: "Gagal membuat CP", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Master CP dibuat", description: res.data?.subjectName });
    setCreateOpen(false);
    setCreateForm({ levelCode: tenantLevelCode, levelName: tenantLevelName, subjectCode: "", subjectName: "", phase: tenantPhases[0] || "e", generalCp: "", sourceUrl: "" });
    setNewElement({ name: "", content: "" });
    await load();
    if (res.data) await openDetail(res.data);
  }

  async function handleAddElement() {
    if (!detail || !newElement.name.trim() || !newElement.content.trim()) return;
    setSaving(true);
    const res = await createCurriculumCPElement(detail.id, { ...newElement, sortOrder: editElements.length + 1 });
    setSaving(false);
    if (res.error) { toast({ tone: "error", title: "Gagal menambah elemen", description: res.error.message }); return; }
    toast({ tone: "success", title: "Elemen CP ditambahkan" });
    setNewElement({ name: "", content: "" });
    await openDetail(detail);
  }

  async function handleDeleteElement(id: string) {
    setSaving(true);
    const res = await deleteCurriculumCPElement(id);
    setSaving(false);
    if (res.error) { toast({ tone: "error", title: "Gagal menghapus elemen", description: res.error.message }); return; }
    toast({ tone: "success", title: "Elemen CP dihapus" });
    if (detail) await openDetail(detail);
  }

  async function handleSave() {
    if (!detail) return;
    setSaving(true);
    const refRes = await updateCurriculumCPReference(detail.id, editRef);
    if (refRes.error) {
      setSaving(false);
      toast({ tone: "error", title: "Gagal menyimpan CP", description: refRes.error.message });
      return;
    }
    for (const el of editElements) {
      const res = await updateCurriculumCPElement(el.id, { name: el.name, content: el.content, sortOrder: el.sortOrder });
      if (res.error) {
        setSaving(false);
        toast({ tone: "error", title: "Gagal menyimpan elemen", description: res.error.message });
        return;
      }
    }
    setSaving(false);
    toast({ tone: "success", title: "CP tersimpan" });
    await load();
    await openDetail(detail);
  }

  return (
    <>
      <PageShell
        title="Master CP Kurikulum"
        subtitle={subtitle}
        search={{ value: search, onChange: setSearch, placeholder: "Cari mapel atau isi CP..." }}
        actions={canManageCP ? (
          <>
            <Button type="button" variant="secondary" onClick={() => setCreateOpen(true)}><Plus size={14} /> Buat Manual</Button>
            <Button type="button" onClick={() => setSeedOpen(true)}><DatabaseZap size={14} /> Seed CP</Button>
          </>
        ) : null}
      >
        <div className="mb-3 rounded-xl border border-[var(--border)] bg-[var(--card)] p-3 text-[12px] text-[var(--muted-foreground)]">
          CP ditampilkan strict sesuai tenant: <span className="font-semibold text-[var(--foreground)]">{tenantLevelName}</span> · Fase {tenantPhases.map((p) => p.toUpperCase()).join(", ")}.
          {!canManageCP && <span> Master CP resmi bersifat read-only; perubahan hanya dapat dilakukan platform admin.</span>}
        </div>

        {loading ? (
          <div className="space-y-2">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-20 w-full" />)}</div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <Search size={24} className="mb-2 text-[var(--muted-foreground)]" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">Belum ada master CP</p>
            <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">Klik Seed CP untuk mengambil data dari endpoint Kemendikdasmen.</p>
          </div>
        ) : (
          <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)]">
            <div className="divide-y divide-[var(--border)]">
              {items.map((item) => (
                <div key={item.id} className="flex items-start gap-3 px-3 py-3 transition-colors hover:bg-[var(--muted)]/50">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]"><BookOpen size={17} /></div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-[13px] font-semibold text-[var(--foreground)]">{item.subjectName}</p>
                      <span className="rounded-md bg-[var(--info-soft)] px-2 py-0.5 text-[10px] font-semibold uppercase text-[var(--info)]">{item.levelCode}</span>
                      <span className="rounded-md bg-[var(--muted)] px-2 py-0.5 text-[10px] font-semibold text-[var(--muted-foreground)]">Fase {item.phase.toUpperCase()}</span>
                    </div>
                    <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{item.subjectCode} · {item.elementsCount} elemen CP</p>
                    <p className="mt-2 line-clamp-2 text-[12px] leading-relaxed text-[var(--foreground)]/80">{item.generalCp}</p>
                  </div>
                  <RowActions actions={[{ label: canManageCP ? "Edit" : "Lihat", icon: <Pencil size={13} />, onClick: () => openDetail(item) }, ...(item.sourceUrl ? [{ label: "Source", icon: <ExternalLink size={13} />, onClick: () => window.open(item.sourceUrl || "", "_blank") }] : [])]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      <RightPullSheet open={seedOpen} title="Seed CP Kemendikdasmen" onClose={() => setSeedOpen(false)}>
        <form onSubmit={handleSeed} className="space-y-3">
          <ComboboxField
            label="Subject"
            value={seedForm.subjectCode === "__custom__" ? "" : seedForm.subjectCode}
            inputValue={seedForm.subjectInput}
            options={tenantSubjectOptions}
            onInputChange={(value) => setSeedForm((f) => ({ ...f, subjectInput: value, subjectCode: "__custom__", customSubjectCode: value.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") }))}
            onSelect={(option) => setSeedForm((f) => ({ ...f, subjectCode: option.value, customSubjectCode: "", subjectInput: option.label }))}
            onCreate={(typed) => setSeedForm((f) => ({ ...f, subjectCode: "__custom__", customSubjectCode: typed.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""), subjectInput: typed }))}
            helperText={`Seed otomatis untuk fase tenant: ${tenantPhases.map((p) => p.toUpperCase()).join(", ")}.`}
          />
          {seedForm.subjectCode === "__custom__" && (
            <InputField label="Subject slug custom" value={seedForm.customSubjectCode} onChange={(e) => setSeedForm((f) => ({ ...f, customSubjectCode: e.target.value }))} helperText="Gunakan slug endpoint resmi jika belum ada di daftar." />
          )}
          <div className="rounded-xl border border-[var(--border)] bg-[var(--muted)]/40 p-3 text-[11px] leading-relaxed text-[var(--muted-foreground)]">
            Sistem akan mengambil JSON CP resmi dari endpoint Kemendikdasmen, menyimpan general CP dan setiap elemen CP sebagai master data read-mostly.
          </div>
          <div className="flex justify-end gap-2 pt-3">
            <Button type="button" variant="secondary" onClick={() => setSeedOpen(false)} disabled={seeding}>Batal</Button>
            <Button type="submit" loading={seeding}><DatabaseZap size={14} /> Seed</Button>
          </div>
        </form>
      </RightPullSheet>


      <RightPullSheet open={createOpen} title="Buat Master CP Manual" onClose={() => setCreateOpen(false)}>
        <form onSubmit={handleCreateManual} className="space-y-3">
          <div className="rounded-xl border border-[var(--border)] bg-[var(--muted)]/30 p-3 text-[11px] text-[var(--muted-foreground)]">Manual CP mengikuti tenant: {tenantLevelName}. Pilih salah satu fase tenant.</div>
          <ComboboxField
            label="Official subject"
            value={createForm.subjectCode}
            inputValue={createForm.subjectName}
            options={tenantSubjectOptions}
            onInputChange={(value) => setCreateForm({ ...createForm, subjectName: value, subjectCode: value.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""), levelCode: tenantLevelCode, levelName: tenantLevelName })}
            onSelect={(option) => setCreateForm({ ...createForm, subjectCode: option.value, subjectName: option.label, levelCode: tenantLevelCode, levelName: tenantLevelName })}
            onCreate={(typed) => setCreateForm({ ...createForm, subjectName: typed, subjectCode: typed.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""), levelCode: tenantLevelCode, levelName: tenantLevelName })}
            createLabel={(typed) => `Tambah “${typed}”`}
          />
          <InputField label="Subject Name" value={createForm.subjectName} onChange={(e) => setCreateForm({ ...createForm, subjectName: e.target.value })} />
          <InputField label="Subject slug" value={createForm.subjectCode} onChange={(e) => setCreateForm({ ...createForm, subjectCode: e.target.value })} helperText="Internal CP source slug; auto-filled from official subject, editable for custom subjects." />
          <SelectField label="Fase" value={createForm.phase} onChange={(v) => setCreateForm({ ...createForm, phase: v })} options={tenantPhases.map((p) => ({ value: p, label: `Fase ${p.toUpperCase()}` }))} />
          <InputField label="Source URL" value={createForm.sourceUrl} onChange={(e) => setCreateForm({ ...createForm, sourceUrl: e.target.value })} />
          <label className="block text-[11px] font-semibold text-[var(--muted-foreground)]">General CP</label>
          <textarea value={createForm.generalCp} onChange={(e) => setCreateForm({ ...createForm, generalCp: e.target.value })} className="min-h-32 w-full rounded-xl border border-[var(--border)] bg-[var(--background)] p-3 text-[12px] leading-relaxed outline-none focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)]" />
          <div className="rounded-xl border border-[var(--border)] bg-[var(--muted)]/30 p-3 space-y-3">
            <p className="text-[12px] font-semibold text-[var(--foreground)]">Elemen pertama opsional</p>
            <InputField label="Nama Elemen" value={newElement.name} onChange={(e) => setNewElement({ ...newElement, name: e.target.value })} />
            <textarea value={newElement.content} onChange={(e) => setNewElement({ ...newElement, content: e.target.value })} className="min-h-24 w-full rounded-xl border border-[var(--border)] bg-[var(--background)] p-3 text-[12px] leading-relaxed outline-none focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)]" />
          </div>
          <div className="flex justify-end gap-2 pt-3">
            <Button type="button" variant="secondary" onClick={() => setCreateOpen(false)} disabled={saving}>Batal</Button>
            <Button type="submit" loading={saving}><Plus size={14} /> Buat</Button>
          </div>
        </form>
      </RightPullSheet>

      <RightPullSheet open={!!detail} title={canManageCP ? "Edit Master CP" : "Lihat Master CP"} onClose={() => setDetail(null)}>
        {detailLoading || !detail ? <Skeleton className="h-40 w-full" /> : (
          <div className="space-y-4">
            <InputField label="Subject Name" value={editRef.subjectName} disabled={!canManageCP} onChange={(e) => setEditRef({ ...editRef, subjectName: e.target.value })} />
            <SelectField label="Status" value={editRef.status} disabled={!canManageCP} onChange={(v) => setEditRef({ ...editRef, status: v })} options={[{ value: "active", label: "Active" }, { value: "archived", label: "Archived" }]} />
            <label className="block text-[11px] font-semibold text-[var(--muted-foreground)]">General CP</label>
            <textarea value={editRef.generalCp} disabled={!canManageCP} onChange={(e) => setEditRef({ ...editRef, generalCp: e.target.value })} className="min-h-32 w-full rounded-xl border border-[var(--border)] bg-[var(--background)] p-3 text-[12px] leading-relaxed outline-none focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)] disabled:opacity-70" />
            <div className="space-y-3">
              <p className="text-[12px] font-semibold text-[var(--foreground)]">Elemen CP</p>
              {editElements.map((el, idx) => (
                <div key={el.id} className="rounded-xl border border-[var(--border)] bg-[var(--muted)]/30 p-3">
                  <div className="flex items-center gap-2">
                    <div className="flex-1"><InputField label="Nama Elemen" value={el.name} disabled={!canManageCP} onChange={(e) => setEditElements((arr) => arr.map((x, i) => i === idx ? { ...x, name: e.target.value } : x))} /></div>
                    {canManageCP && <Button type="button" variant="secondary" onClick={() => handleDeleteElement(el.id)} disabled={saving}><Trash2 size={14} /></Button>}
                  </div>
                  <textarea value={el.content} disabled={!canManageCP} onChange={(e) => setEditElements((arr) => arr.map((x, i) => i === idx ? { ...x, content: e.target.value } : x))} className="mt-3 min-h-28 w-full rounded-xl border border-[var(--border)] bg-[var(--background)] p-3 text-[12px] leading-relaxed outline-none focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)] disabled:opacity-70" />
                </div>
              ))}
            </div>
            {canManageCP && (
              <div className="rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-3 space-y-3">
                <p className="text-[12px] font-semibold text-[var(--foreground)]">Tambah Elemen CP</p>
                <InputField label="Nama Elemen Baru" value={newElement.name} onChange={(e) => setNewElement({ ...newElement, name: e.target.value })} />
                <textarea value={newElement.content} onChange={(e) => setNewElement({ ...newElement, content: e.target.value })} className="min-h-24 w-full rounded-xl border border-[var(--border)] bg-[var(--background)] p-3 text-[12px] leading-relaxed outline-none focus:border-[var(--field-focus)] focus:ring-2 focus:ring-[var(--field-ring)]" />
                <Button type="button" variant="secondary" onClick={handleAddElement} loading={saving}><Plus size={14} /> Tambah Elemen</Button>
              </div>
            )}
            <div className="flex justify-end gap-2 pt-3">
              <Button type="button" variant="secondary" onClick={() => setDetail(null)} disabled={saving}>{canManageCP ? "Batal" : "Tutup"}</Button>
              {canManageCP && <Button type="button" loading={saving} onClick={handleSave}>Simpan</Button>}
            </div>
          </div>
        )}
      </RightPullSheet>
    </>
  );
}
