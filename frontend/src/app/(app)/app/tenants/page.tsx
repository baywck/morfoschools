"use client";

import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/lib/auth-provider";
import { useCRUD } from "@/lib/use-crud";
import { useToast } from "@/components/ui/toast";
import { listTenants, createTenant, updateTenant, archiveTenant, switchTenant, uploadTenantLogo, getTenantAISettings, patchTenantAISettings, saveTenantAISettings, listRoles, type AIProviderSettings, type Role, type SchoolType, type Tenant } from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, Building2, Trash2, ArrowRightLeft, Pencil, Upload, Loader2, KeyRound, ShieldCheck } from "lucide-react";
import { cn } from "@/lib/cn";

const SCHOOL_TYPES: Array<{ value: SchoolType; label: string; hint: string }> = [
  { value: "sd", label: "SD / MI", hint: "Fase A-C" },
  { value: "smp", label: "SMP / MTs", hint: "Fase D" },
  { value: "sma", label: "SMA / MA", hint: "Fase E-F" },
  { value: "smk", label: "SMK", hint: "Fase E-F + Vokasi" },
  { value: "mixed", label: "Mixed", hint: "Atur fase sendiri" },
];

const PHASES = ["a", "b", "c", "d", "e", "f"];

function schoolTypeLabel(value?: SchoolType | null) {
  const safeValue = value || "sma";
  return SCHOOL_TYPES.find((type) => type.value === safeValue)?.label || safeValue.toUpperCase();
}

function safeTenantPhases(tenant: Tenant) {
  return tenant.enabledPhases?.length ? tenant.enabledPhases : ["e", "f"];
}

function EducationProfileEditor({
  schoolType,
  enabledPhases,
  includeVocationalSubjects,
  onSchoolTypeChange,
  onPhaseToggle,
  onVocationalChange,
}: {
  schoolType: SchoolType;
  enabledPhases: string[];
  includeVocationalSubjects: boolean;
  onSchoolTypeChange: (value: SchoolType) => void;
  onPhaseToggle: (phase: string) => void;
  onVocationalChange: (value: boolean) => void;
}) {
  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--accent)] p-3">
      <div className="mb-3">
        <p className="text-[12px] font-semibold text-[var(--foreground)]">Education Profile</p>
        <p className="text-[11px] text-[var(--muted-foreground)]">Menentukan CP, subject, blueprint, dan exam yang relevan untuk tenant.</p>
      </div>
      <div className="grid grid-cols-2 gap-2">
        {SCHOOL_TYPES.map((type) => (
          <button
            key={type.value}
            type="button"
            onClick={() => onSchoolTypeChange(type.value)}
            className={cn(
              "rounded-lg border p-2 text-left transition-all",
              schoolType === type.value
                ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--foreground)] shadow-sm"
                : "border-[var(--border)] bg-[var(--card)] text-[var(--muted-foreground)] hover:border-[var(--border-strong)]",
            )}
          >
            <span className="block text-[12px] font-semibold">{type.label}</span>
            <span className="text-[10px]">{type.hint}</span>
          </button>
        ))}
      </div>
      {schoolType === "mixed" && (
        <div className="mt-3 space-y-3">
          <div>
            <p className="mb-2 text-[11px] font-medium text-[var(--foreground)]">Enabled phases</p>
            <div className="flex flex-wrap gap-1.5">
              {PHASES.map((phase) => (
                <button
                  key={phase}
                  type="button"
                  onClick={() => onPhaseToggle(phase)}
                  className={cn(
                    "rounded-full border px-3 py-1 text-[11px] font-semibold transition-all",
                    enabledPhases.includes(phase)
                      ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]"
                      : "border-[var(--border)] bg-[var(--card)] text-[var(--muted-foreground)]",
                  )}
                >
                  Fase {phase.toUpperCase()}
                </button>
              ))}
            </div>
          </div>
          <button
            type="button"
            onClick={() => onVocationalChange(!includeVocationalSubjects)}
            className={cn(
              "flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-[12px] transition-all",
              includeVocationalSubjects ? "border-[var(--brand)] bg-[var(--brand-soft)]" : "border-[var(--border)] bg-[var(--card)]",
            )}
          >
            <span>
              <span className="block font-semibold text-[var(--foreground)]">Include SMK subjects</span>
              <span className="text-[10px] text-[var(--muted-foreground)]">Aktifkan subject vokasi untuk tenant mixed.</span>
            </span>
            <span className={cn("h-5 w-9 rounded-full p-0.5 transition-colors", includeVocationalSubjects ? "bg-[var(--brand)]" : "bg-[var(--border-strong)]")}>
              <span className={cn("block h-4 w-4 rounded-full bg-white transition-transform", includeVocationalSubjects && "translate-x-4")} />
            </span>
          </button>
        </div>
      )}
    </section>
  );
}

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

  const [createForm, setCreateForm] = useState({ name: "", code: "", schoolType: "sma" as SchoolType, enabledPhases: ["e", "f"], includeVocationalSubjects: false });
  const [editForm, setEditForm] = useState({ name: "", status: "", schoolType: "sma" as SchoolType, enabledPhases: ["e", "f"], includeVocationalSubjects: false });
  const [logoLoadingId, setLogoLoadingId] = useState<string | null>(null);
  const [brokenLogos, setBrokenLogos] = useState<Record<string, boolean>>({});
  const [roles, setRoles] = useState<Role[]>([]);
  const [tenantAI, setTenantAI] = useState<AIProviderSettings | null>(null);
  const [tenantAILoading, setTenantAILoading] = useState(false);
  const [tenantAISaving, setTenantAISaving] = useState(false);
  const [tenantAIFields, setTenantAIFields] = useState<Record<string, string>>({});
  const MASKED_KEY = "*********************Jks";
  const [tenantAIForm, setTenantAIForm] = useState({ baseUrl: "", apiKey: "", defaultModel: "", enabled: true, allowedRoles: [] as string[], chatbotModels: [] as string[] });

  // Switch tenant
  const [switchTarget, setSwitchTarget] = useState<Tenant | null>(null);
  const [switching, setSwitching] = useState(false);

  useEffect(() => {
    listRoles().then((res) => {
      if (res.data) setRoles(res.data.data || []);
    });
  }, []);

  async function loadTenantAISettings(tenantID: string) {
    setTenantAILoading(true);
    setTenantAIFields({});
    const res = await getTenantAISettings(tenantID);
    setTenantAILoading(false);
    if (res.error) {
      toast({ tone: "error", title: "AI settings gagal dimuat", description: res.error.message });
      return;
    }
    if (res.data) {
      setTenantAI(res.data);
      setTenantAIForm({
        baseUrl: res.data.baseUrl || "",
        apiKey: res.data.hasApiKey ? MASKED_KEY : "",
        defaultModel: res.data.defaultModel || "",
        enabled: res.data.enabled,
        allowedRoles: res.data.allowedRoles || [],
        chatbotModels: (res.data.chatbotModels || []).map((m) => m.id),
      });
    }
  }

  function openEdit(tenant: Tenant) {
    crud.setEditTarget(tenant);
    crud.setFieldErrors({});
    setEditForm({ name: tenant.name, status: tenant.status, schoolType: tenant.schoolType || "sma", enabledPhases: tenant.enabledPhases?.length ? tenant.enabledPhases : ["e", "f"], includeVocationalSubjects: tenant.includeVocationalSubjects });
    setTenantAI(null);
    setTenantAIForm({ baseUrl: "", apiKey: "", defaultModel: "", enabled: true, allowedRoles: [], chatbotModels: [] });
    void loadTenantAISettings(tenant.id);
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const success = await crud.handleCreate(createForm);
    if (success) setCreateForm({ name: "", code: "", schoolType: "sma", enabledPhases: ["e", "f"], includeVocationalSubjects: false });
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    await crud.handleEdit(crud.editTarget.id, editForm);
  }

  async function handleTenantAISave() {
    if (!crud.editTarget) return;
    setTenantAISaving(true);
    setTenantAIFields({});
    const res = await saveTenantAISettings(tenantAIForm, crud.editTarget.id);
    setTenantAISaving(false);
    if (res.error) {
      setTenantAIFields(res.error.fields || {});
      toast({ tone: "error", title: "AI provider gagal disimpan", description: res.error.message });
      return;
    }
    if (res.data) {
      setTenantAI(res.data);
      setTenantAIForm((form) => ({ ...form, apiKey: res.data!.hasApiKey ? MASKED_KEY : "", defaultModel: res.data!.defaultModel || form.defaultModel, allowedRoles: res.data!.allowedRoles || [], chatbotModels: (res.data!.chatbotModels || []).map((m) => m.id) }));
    }
    window.dispatchEvent(new Event("morfoschools:ai-settings-changed"));
    toast({ tone: "success", title: "AI provider tersimpan", description: "Connection OK, daftar model berhasil diambil." });
  }

  async function handleLogoUpload(tenant: Tenant, file?: File) {
    if (!file) return;
    setLogoLoadingId(tenant.id);
    const res = await uploadTenantLogo(tenant.id, file);
    const rowInput = document.getElementById(`tenant-logo-${tenant.id}`) as HTMLInputElement | null;
    const editInput = document.getElementById(`tenant-logo-edit-${tenant.id}`) as HTMLInputElement | null;
    if (rowInput) rowInput.value = "";
    if (editInput) editInput.value = "";
    setLogoLoadingId(null);
    if (res.error) {
      toast({ tone: "error", title: "Logo gagal diupload", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Logo tenant diperbarui" });
    setBrokenLogos((prev) => ({ ...prev, [tenant.id]: false }));
    if (res.data?.logoUrl) {
      crud.setItems((items) => items.map((item) => item.id === tenant.id ? { ...item, logoUrl: res.data!.logoUrl } : item));
      if (crud.editTarget?.id === tenant.id) {
        crud.setEditTarget({ ...crud.editTarget, logoUrl: res.data.logoUrl });
      }
    }
    await crud.reload();
  }

  function applySchoolType<T extends { schoolType: SchoolType; enabledPhases: string[]; includeVocationalSubjects: boolean }>(form: T, schoolType: SchoolType): T {
    const defaults: Record<SchoolType, { phases: string[]; vocational: boolean }> = {
      sd: { phases: ["a", "b", "c"], vocational: false },
      smp: { phases: ["d"], vocational: false },
      sma: { phases: ["e", "f"], vocational: false },
      smk: { phases: ["e", "f"], vocational: true },
      mixed: { phases: form.enabledPhases.length ? form.enabledPhases : ["e", "f"], vocational: form.includeVocationalSubjects },
    };
    return { ...form, schoolType, enabledPhases: defaults[schoolType].phases, includeVocationalSubjects: defaults[schoolType].vocational };
  }

  function togglePhase<T extends { enabledPhases: string[] }>(form: T, phase: string): T {
    const exists = form.enabledPhases.includes(phase);
    const next = exists ? form.enabledPhases.filter((p) => p !== phase) : [...form.enabledPhases, phase];
    return { ...form, enabledPhases: next.length ? next : [phase] };
  }

  function toggleAIAllowedRole(roleSlug: string) {
    setTenantAIForm((form) => ({
      ...form,
      allowedRoles: form.allowedRoles.includes(roleSlug)
        ? form.allowedRoles.filter((role) => role !== roleSlug)
        : [...form.allowedRoles, roleSlug],
    }));
  }

  const tenantAIModelOptions = useMemo(() => {
    const models = tenantAI?.availableModels || [];
    if (models.length === 0) return [{ value: "", label: "Save untuk mengambil model" }];
    return models.map((model) => ({ value: model.id, label: model.id }));
  }, [tenantAI]);

  function toggleTenantAIChatbotModel(modelID: string) {
    setTenantAIForm((form) => ({
      ...form,
      chatbotModels: form.chatbotModels.includes(modelID)
        ? form.chatbotModels.filter((id) => id !== modelID)
        : [...form.chatbotModels, modelID],
    }));
  }

  async function toggleTenantAIEnabled(enabled: boolean) {
    if (!crud.editTarget) return;
    const previous = tenantAIForm.enabled;
    setTenantAIForm((form) => ({ ...form, enabled }));
    const res = await patchTenantAISettings({ enabled }, crud.editTarget.id);
    if (res.error) {
      setTenantAIForm((form) => ({ ...form, enabled: previous }));
      toast({ tone: "error", title: "Toggle AI gagal", description: res.error.message });
      return;
    }
    if (res.data) setTenantAI(res.data);
    window.dispatchEvent(new Event("morfoschools:ai-settings-changed"));
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
                  <div className="relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
                    {t.logoUrl && !brokenLogos[t.id] ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={t.logoUrl} alt={`${t.name} logo`} className="h-full w-full object-cover" onError={() => setBrokenLogos((prev) => ({ ...prev, [t.id]: true }))} />
                    ) : (
                      <span className="text-[11px] font-bold text-[var(--foreground)]">{t.name.split(/\s+/).slice(0, 2).map((part) => part[0]).join("").toUpperCase()}</span>
                    )}
                    {logoLoadingId === t.id && (
                      <div className="absolute inset-0 flex items-center justify-center bg-[var(--background)]/75 backdrop-blur-sm">
                        <Loader2 size={16} className="animate-spin text-[var(--primary)]" />
                      </div>
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{t.name}</p>
                    <p className="text-[11px] text-[var(--muted-foreground)]">{t.code} · {schoolTypeLabel(t.schoolType)} · Fase {safeTenantPhases(t).map((p) => p.toUpperCase()).join(", ")}</p>
                  </div>
                  <span className={cn(
                    "rounded-md px-2 py-0.5 text-[10px] font-medium",
                    t.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  )}>
                    {t.status}
                  </span>
                  <RowActions actions={[
                    { label: logoLoadingId === t.id ? "Uploading..." : "Upload Logo", icon: logoLoadingId === t.id ? <Loader2 size={13} className="animate-spin" /> : <Upload size={13} />, disabled: logoLoadingId === t.id, onClick: () => document.getElementById(`tenant-logo-${t.id}`)?.click() },
                    { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(t) },
                    { label: "Switch", icon: <ArrowRightLeft size={13} />, onClick: () => setSwitchTarget(t) },
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => crud.setArchiveTarget(t), variant: "danger" },
                  ]} />
                  <input id={`tenant-logo-${t.id}`} type="file" disabled={logoLoadingId === t.id} accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml" className="hidden" onChange={(e) => handleLogoUpload(t, e.target.files?.[0])} />
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
          <EducationProfileEditor
            schoolType={createForm.schoolType}
            enabledPhases={createForm.enabledPhases}
            includeVocationalSubjects={createForm.includeVocationalSubjects}
            onSchoolTypeChange={(schoolType) => setCreateForm((f) => applySchoolType(f, schoolType))}
            onPhaseToggle={(phase) => setCreateForm((f) => togglePhase(f, phase))}
            onVocationalChange={(value) => setCreateForm((f) => ({ ...f, includeVocationalSubjects: value }))}
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
          {crud.editTarget && (
            <div className="rounded-xl border border-[var(--border)] bg-[var(--accent)] p-3">
              <div className="flex items-center gap-3">
                <div className="relative flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-[var(--muted)]">
                  {crud.editTarget.logoUrl && !brokenLogos[crud.editTarget.id] ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={crud.editTarget.logoUrl} alt={`${crud.editTarget.name} logo`} className="h-full w-full object-cover" onError={() => setBrokenLogos((prev) => ({ ...prev, [crud.editTarget!.id]: true }))} />
                  ) : (
                    <span className="text-[12px] font-bold text-[var(--foreground)]">{crud.editTarget.name.split(/\s+/).slice(0, 2).map((part) => part[0]).join("").toUpperCase()}</span>
                  )}
                  {logoLoadingId === crud.editTarget.id && (
                    <div className="absolute inset-0 flex items-center justify-center bg-[var(--background)]/75 backdrop-blur-sm">
                      <Loader2 size={18} className="animate-spin text-[var(--primary)]" />
                    </div>
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-[12px] font-semibold text-[var(--foreground)]">School logo</p>
                  <p className="text-[11px] text-[var(--muted-foreground)]">PNG/JPG/WEBP/GIF/SVG, maksimal 2MB. Jika URL R2 tidak bisa diakses, UI fallback ke initials.</p>
                </div>
                <button type="button" disabled={logoLoadingId === crud.editTarget.id} onClick={() => document.getElementById(`tenant-logo-edit-${crud.editTarget!.id}`)?.click()} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm disabled:opacity-50">
                  {logoLoadingId === crud.editTarget.id && <Loader2 size={13} className="animate-spin" />}
                  {logoLoadingId === crud.editTarget.id ? "Uploading..." : "Upload"}
                </button>
                <input id={`tenant-logo-edit-${crud.editTarget.id}`} type="file" disabled={logoLoadingId === crud.editTarget.id} accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml" className="hidden" onChange={(e) => handleLogoUpload(crud.editTarget!, e.target.files?.[0])} />
              </div>
            </div>
          )}
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
          <EducationProfileEditor
            schoolType={editForm.schoolType}
            enabledPhases={editForm.enabledPhases}
            includeVocationalSubjects={editForm.includeVocationalSubjects}
            onSchoolTypeChange={(schoolType) => setEditForm((f) => applySchoolType(f, schoolType))}
            onPhaseToggle={(phase) => setEditForm((f) => togglePhase(f, phase))}
            onVocationalChange={(value) => setEditForm((f) => ({ ...f, includeVocationalSubjects: value }))}
          />
          <section className="rounded-xl border border-[var(--border)] bg-[var(--accent)] p-3">
            <div className="mb-3 flex items-start gap-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--brand-soft)] text-[var(--brand)]">
                <ShieldCheck size={15} />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-[12px] font-semibold text-[var(--foreground)]">AI API / BYOK</p>
                <p className="text-[11px] text-[var(--muted-foreground)]">Provider AI untuk tenant ini. Saat disimpan, backend cek koneksi dan mengambil /models dulu. API key terenkripsi dan tidak pernah ditampilkan ulang.</p>
              </div>
              {tenantAILoading && <Loader2 size={14} className="animate-spin text-[var(--muted-foreground)]" />}
            </div>
            <div className="mb-3 rounded-lg border border-[var(--border)] bg-[var(--card)] p-2 text-[11px] text-[var(--muted-foreground)]">
              <span className={cn("mr-2 inline-block h-2 w-2 rounded-full", tenantAI?.hasApiKey ? "bg-[var(--success)]" : "bg-[var(--muted-foreground)]")} />
              {tenantAI?.hasApiKey ? "Configured" : "Not configured"}
              {tenantAI?.defaultModel ? ` · default: ${tenantAI.defaultModel}` : ""}
              {tenantAI?.availableModels?.length ? ` · ${tenantAI.availableModels.length} models` : ""}
            </div>
            <div className="space-y-3">
              <InputField
                label="AI Base URL"
                value={tenantAIForm.baseUrl}
                onChange={(e) => setTenantAIForm((form) => ({ ...form, baseUrl: e.target.value }))}
                error={tenantAIFields.baseUrl}
                helperText="Contoh: https://provider.example/v1"
              />
              <InputField
                label={tenantAI?.hasApiKey ? "API Key baru" : "API Key"}
                type="password"
                value={tenantAIForm.apiKey}
                onChange={(e) => setTenantAIForm((form) => ({ ...form, apiKey: e.target.value }))}
                error={tenantAIFields.apiKey}
                prefix={<KeyRound size={14} />}
                helperText="Wajib untuk save. Tidak disimpan plaintext."
              />
              <SelectField
                label="Default model"
                value={tenantAIForm.defaultModel}
                onChange={(value) => setTenantAIForm((form) => ({ ...form, defaultModel: value }))}
                options={tenantAIModelOptions}
              />
              <div>
                <p className="mb-2 text-[11px] font-semibold text-[var(--muted-foreground)]">Models shown in chatbot</p>
                <div className="max-h-36 overflow-auto rounded-lg border border-[var(--border)] bg-[var(--card)] p-2">
                  <div className="flex flex-wrap gap-1.5">
                    {(tenantAI?.availableModels || []).map((model) => (
                      <button key={model.id} type="button" onClick={() => toggleTenantAIChatbotModel(model.id)} className={cn("rounded-lg border px-2.5 py-1 text-[11px]", tenantAIForm.chatbotModels.includes(model.id) ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)] text-[var(--muted-foreground)]")}>{model.id}</button>
                    ))}
                    {(tenantAI?.availableModels || []).length === 0 && <span className="text-[11px] text-[var(--muted-foreground)]">Save provider dulu untuk mengambil models.</span>}
                  </div>
                </div>
              </div>
              <button
                type="button"
                onClick={() => toggleTenantAIEnabled(!tenantAIForm.enabled)}
                className={cn("flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-[12px] transition-all", tenantAIForm.enabled ? "border-[var(--brand)] bg-[var(--brand-soft)]" : "border-[var(--border)] bg-[var(--card)]")}
              >
                <span>
                  <span className="block font-semibold text-[var(--foreground)]">Enabled</span>
                  <span className="text-[10px] text-[var(--muted-foreground)]">Nonaktif = tenant provider tidak dipakai; user fallback ke personal/environment.</span>
                </span>
                <span className={cn("h-5 w-9 rounded-full p-0.5 transition-colors", tenantAIForm.enabled ? "bg-[var(--brand)]" : "bg-[var(--border-strong)]")}>
                  <span className={cn("block h-4 w-4 rounded-full bg-white transition-transform", tenantAIForm.enabled && "translate-x-4")} />
                </span>
              </button>
              <div>
                <p className="mb-2 text-[11px] font-semibold text-[var(--muted-foreground)]">Allowed roles</p>
                <div className="flex flex-wrap gap-1.5">
                  <button type="button" onClick={() => setTenantAIForm((form) => ({ ...form, allowedRoles: [] }))} className={cn("rounded-lg border px-2.5 py-1 text-[11px]", tenantAIForm.allowedRoles.length === 0 ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)] bg-[var(--card)] text-[var(--muted-foreground)]")}>All roles</button>
                  {roles.map((role) => (
                    <button key={role.slug} type="button" onClick={() => toggleAIAllowedRole(role.slug)} className={cn("rounded-lg border px-2.5 py-1 text-[11px]", tenantAIForm.allowedRoles.includes(role.slug) ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)] bg-[var(--card)] text-[var(--muted-foreground)]")}>{role.name}</button>
                  ))}
                </div>
              </div>
              <button type="button" disabled={tenantAISaving || tenantAILoading} onClick={handleTenantAISave} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 disabled:opacity-50">
                {tenantAISaving && <Loader2 size={13} className="animate-spin" />}
                Save AI Provider & Check Models
              </button>
            </div>
          </section>
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
