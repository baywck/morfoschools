"use client";

import { useEffect, useMemo, useState } from "react";
import { KeyRound, Loader2, Save, Sparkles, ShieldCheck } from "lucide-react";
import { PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { useAuth } from "@/lib/auth-provider";
import { cn } from "@/lib/cn";
import { getMyAISettings, getTenantAISettings, listRoles, patchMyAISettings, patchTenantAISettings, saveMyAISettings, saveTenantAISettings, type AIProviderSettings, type Role } from "@/lib/modules-api";

type FormState = {
  baseUrl: string;
  apiKey: string;
  defaultModel: string;
  enabled: boolean;
  allowedRoles: string[];
  chatbotModels: string[];
};

const MASKED_KEY = "*********************Jks";
const emptyForm: FormState = { baseUrl: "", apiKey: "", defaultModel: "", enabled: true, allowedRoles: [], chatbotModels: [] };

export default function SettingsPage() {
  const { session } = useAuth();
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);
  const [savingUser, setSavingUser] = useState(false);
  const [savingTenant, setSavingTenant] = useState(false);
  const [roles, setRoles] = useState<Role[]>([]);
  const [userSettings, setUserSettings] = useState<AIProviderSettings | null>(null);
  const [tenantSettings, setTenantSettings] = useState<AIProviderSettings | null>(null);
  const [userForm, setUserForm] = useState<FormState>(emptyForm);
  const [tenantForm, setTenantForm] = useState<FormState>(emptyForm);
  const [userFields, setUserFields] = useState<Record<string, string>>({});
  const [tenantFields, setTenantFields] = useState<Record<string, string>>({});

  const canManageTenant = !!session?.permissions?.includes("tenants:write");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      const [mine, tenant, roleRes] = await Promise.all([
        getMyAISettings(),
        canManageTenant ? getTenantAISettings() : Promise.resolve({ data: undefined }),
        canManageTenant ? listRoles() : Promise.resolve({ data: { data: [] } }),
      ]);
      if (cancelled) return;
      if (mine.data) {
        setUserSettings(mine.data);
        setUserForm({ baseUrl: mine.data.baseUrl || "", apiKey: mine.data.hasApiKey ? MASKED_KEY : "", defaultModel: mine.data.defaultModel || "", enabled: mine.data.enabled, allowedRoles: [], chatbotModels: (mine.data.chatbotModels || []).map((m) => m.id) });
      }
      if (tenant.data) {
        setTenantSettings(tenant.data);
        setTenantForm({ baseUrl: tenant.data.baseUrl || "", apiKey: tenant.data.hasApiKey ? MASKED_KEY : "", defaultModel: tenant.data.defaultModel || "", enabled: tenant.data.enabled, allowedRoles: tenant.data.allowedRoles || [], chatbotModels: (tenant.data.chatbotModels || []).map((m) => m.id) });
      }
      if (roleRes.data) setRoles(roleRes.data.data || []);
      setLoading(false);
    }
    load();
    return () => { cancelled = true; };
  }, [canManageTenant]);

  const userModelOptions = useMemo(() => modelOptions(userSettings), [userSettings]);
  const tenantModelOptions = useMemo(() => modelOptions(tenantSettings), [tenantSettings]);

  async function saveUser(e: React.FormEvent) {
    e.preventDefault();
    setSavingUser(true);
    setUserFields({});
    const res = await saveMyAISettings(userForm);
    setSavingUser(false);
    if (res.error) {
      setUserFields(res.error.fields || {});
      toast({ tone: "error", title: "AI settings gagal disimpan", description: res.error.message });
      return;
    }
    if (res.data) {
      setUserSettings(res.data);
      setUserForm((f) => ({ ...f, apiKey: res.data!.hasApiKey ? MASKED_KEY : "", defaultModel: res.data!.defaultModel || f.defaultModel, chatbotModels: (res.data!.chatbotModels || []).map((m) => m.id) }));
    }
    window.dispatchEvent(new Event("morfoschools:ai-settings-changed"));
    toast({ tone: "success", title: "AI settings tersimpan", description: "Connection OK, models berhasil diambil." });
  }

  async function saveTenant(e: React.FormEvent) {
    e.preventDefault();
    setSavingTenant(true);
    setTenantFields({});
    const res = await saveTenantAISettings(tenantForm);
    setSavingTenant(false);
    if (res.error) {
      setTenantFields(res.error.fields || {});
      toast({ tone: "error", title: "Tenant AI settings gagal disimpan", description: res.error.message });
      return;
    }
    if (res.data) {
      setTenantSettings(res.data);
      setTenantForm((f) => ({ ...f, apiKey: res.data!.hasApiKey ? MASKED_KEY : "", defaultModel: res.data!.defaultModel || f.defaultModel, allowedRoles: res.data!.allowedRoles || [], chatbotModels: (res.data!.chatbotModels || []).map((m) => m.id) }));
    }
    window.dispatchEvent(new Event("morfoschools:ai-settings-changed"));
    toast({ tone: "success", title: "Tenant AI settings tersimpan", description: "Connection OK, models berhasil diambil." });
  }

  return (
    <PageShell title="Settings" subtitle="Konfigurasi AI provider tenant dan personal user override.">
      {loading ? <Skeleton className="h-80 w-full" /> : (
        <div className="grid gap-4 xl:grid-cols-2">
          {canManageTenant && (
            <form onSubmit={saveTenant} className="rounded-2xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
              <div className="mb-4 flex items-start gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-[var(--brand-soft)] text-[var(--brand)]"><ShieldCheck size={18} /></div>
                <div>
                  <h2 className="text-[14px] font-semibold text-[var(--foreground)]">Tenant AI Provider</h2>
                  <p className="text-[11px] text-[var(--muted-foreground)]">Dipakai semua user tenant, kecuali user punya override personal. API key tidak pernah ditampilkan ulang.</p>
                </div>
              </div>
              <ProviderStatus settings={tenantSettings} />
              <div className="mt-4 space-y-3">
                <InputField label="AI Base URL" value={tenantForm.baseUrl} onChange={(e) => setTenantForm({ ...tenantForm, baseUrl: e.target.value })} error={tenantFields.baseUrl} helperText="Contoh: https://provider.example/v1. Saat save, sistem cek /models dulu." />
                <InputField label={tenantSettings?.hasApiKey ? "API Key baru" : "API Key"} type="password" value={tenantForm.apiKey} onChange={(e) => setTenantForm({ ...tenantForm, apiKey: e.target.value })} error={tenantFields.apiKey} prefix={<KeyRound size={14} />} helperText="Wajib saat save; disimpan terenkripsi dan tidak dikirim balik." />
                <SelectField label="Default model" value={tenantForm.defaultModel} onChange={(v) => setTenantForm({ ...tenantForm, defaultModel: v })} options={tenantModelOptions} />
                <ChatbotModelPicker settings={tenantSettings} selected={tenantForm.chatbotModels} onChange={(chatbotModels) => setTenantForm({ ...tenantForm, chatbotModels })} />
                <EnabledToggle enabled={tenantForm.enabled} onChange={async (enabled) => {
                  const previous = tenantForm.enabled;
                  setTenantForm({ ...tenantForm, enabled });
                  const res = await patchTenantAISettings({ enabled });
                  if (res.error) { setTenantForm((f) => ({ ...f, enabled: previous })); toast({ tone: "error", title: "Toggle gagal", description: res.error.message }); }
                  else if (res.data) setTenantSettings(res.data);
                }} />
                <RolePicker roles={roles} selected={tenantForm.allowedRoles} onChange={(allowedRoles) => setTenantForm({ ...tenantForm, allowedRoles })} />
                <Button type="submit" loading={savingTenant}><Save size={14} /> Save & Check Models</Button>
              </div>
            </form>
          )}

          <form onSubmit={saveUser} className="rounded-2xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
            <div className="mb-4 flex items-start gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-[var(--info-soft)] text-[var(--info)]"><Sparkles size={18} /></div>
              <div>
                <h2 className="text-[14px] font-semibold text-[var(--foreground)]">Personal AI Provider</h2>
                <p className="text-[11px] text-[var(--muted-foreground)]">Override untuk akun Anda. Jika kosong/tidak aktif, chatbot pakai tenant provider lalu fallback environment.</p>
              </div>
            </div>
            <ProviderStatus settings={userSettings} />
            <div className="mt-4 space-y-3">
              <InputField label="AI Base URL" value={userForm.baseUrl} onChange={(e) => setUserForm({ ...userForm, baseUrl: e.target.value })} error={userFields.baseUrl} helperText="Saat save, koneksi dan daftar model dicek dulu." />
              <InputField label={userSettings?.hasApiKey ? "API Key baru" : "API Key"} type="password" value={userForm.apiKey} onChange={(e) => setUserForm({ ...userForm, apiKey: e.target.value })} error={userFields.apiKey} prefix={<KeyRound size={14} />} />
              <SelectField label="Default model" value={userForm.defaultModel} onChange={(v) => setUserForm({ ...userForm, defaultModel: v })} options={userModelOptions} />
              <ChatbotModelPicker settings={userSettings} selected={userForm.chatbotModels} onChange={(chatbotModels) => setUserForm({ ...userForm, chatbotModels })} />
              <EnabledToggle enabled={userForm.enabled} onChange={async (enabled) => {
                const previous = userForm.enabled;
                setUserForm({ ...userForm, enabled });
                const res = await patchMyAISettings({ enabled });
                if (res.error) { setUserForm((f) => ({ ...f, enabled: previous })); toast({ tone: "error", title: "Toggle gagal", description: res.error.message }); }
                else if (res.data) setUserSettings(res.data);
              }} />
              <Button type="submit" loading={savingUser}><Save size={14} /> Save & Check Models</Button>
            </div>
          </form>
        </div>
      )}
    </PageShell>
  );
}

function modelOptions(settings: AIProviderSettings | null) {
  const models = settings?.availableModels || [];
  if (models.length === 0) return [{ value: "", label: "Save settings untuk mengambil models" }];
  return models.map((m) => ({ value: m.id, label: m.id }));
}

function ProviderStatus({ settings }: { settings: AIProviderSettings | null }) {
  const models = settings?.availableModels || [];
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--muted)]/30 p-3 text-[11px] text-[var(--muted-foreground)]">
      <div className="flex items-center gap-2">
        {settings?.hasApiKey ? <span className="h-2 w-2 rounded-full bg-[var(--success)]" /> : <span className="h-2 w-2 rounded-full bg-[var(--muted-foreground)]" />}
        <span>{settings?.hasApiKey ? "Configured" : "Not configured"}</span>
        {settings?.scope && <span>· {settings.scope}</span>}
        {settings?.defaultModel && <span>· default: {settings.defaultModel}</span>}
      </div>
      {models.length > 0 && <p className="mt-2">{models.length} model tersedia: {models.slice(0, 5).map((m) => m.id).join(", ")}{models.length > 5 ? "…" : ""}</p>}
    </div>
  );
}

function ChatbotModelPicker({ settings, selected, onChange }: { settings: AIProviderSettings | null; selected: string[]; onChange: (models: string[]) => void }) {
  const models = settings?.availableModels || [];
  function toggle(id: string) {
    onChange(selected.includes(id) ? selected.filter((model) => model !== id) : [...selected, id]);
  }
  if (models.length === 0) {
    return <p className="rounded-lg border border-[var(--border)] bg-[var(--muted)]/30 p-3 text-[11px] text-[var(--muted-foreground)]">Save provider dulu untuk memilih model yang muncul di chatbot.</p>;
  }
  return (
    <div>
      <p className="mb-2 text-[11px] font-semibold text-[var(--muted-foreground)]">Models shown in chatbot</p>
      <div className="max-h-36 overflow-auto rounded-lg border border-[var(--border)] bg-[var(--card)] p-2">
        <div className="flex flex-wrap gap-1.5">
          {models.map((model) => (
            <button key={model.id} type="button" onClick={() => toggle(model.id)} className={cn("rounded-lg border px-2.5 py-1 text-[11px]", selected.includes(model.id) ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)] text-[var(--muted-foreground)]")}>{model.id}</button>
          ))}
        </div>
      </div>
      <p className="mt-1 text-[10px] text-[var(--muted-foreground)]">Kosong = backend fallback ke default model saja.</p>
    </div>
  );
}

function EnabledToggle({ enabled, onChange }: { enabled: boolean; onChange: (enabled: boolean) => void | Promise<void> }) {
  return (
    <button
      type="button"
      onClick={() => onChange(!enabled)}
      className={cn("flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-[12px] transition-all", enabled ? "border-[var(--brand)] bg-[var(--brand-soft)]" : "border-[var(--border)] bg-[var(--card)]")}
    >
      <span>
        <span className="block font-semibold text-[var(--foreground)]">Enabled</span>
        <span className="text-[10px] text-[var(--muted-foreground)]">Nonaktif = jangan pakai setting ini; fallback ke level berikutnya.</span>
      </span>
      <span className={cn("h-5 w-9 rounded-full p-0.5 transition-colors", enabled ? "bg-[var(--brand)]" : "bg-[var(--border-strong)]")}>
        <span className={cn("block h-4 w-4 rounded-full bg-white transition-transform", enabled && "translate-x-4")} />
      </span>
    </button>
  );
}

function RolePicker({ roles, selected, onChange }: { roles: Role[]; selected: string[]; onChange: (roles: string[]) => void }) {
  function toggle(slug: string) {
    onChange(selected.includes(slug) ? selected.filter((r) => r !== slug) : [...selected, slug]);
  }
  return (
    <div>
      <p className="mb-2 text-[11px] font-semibold text-[var(--muted-foreground)]">Roles allowed to use tenant provider</p>
      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={() => onChange([])} className={cn("rounded-lg border px-2.5 py-1 text-[11px]", selected.length === 0 ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)] text-[var(--muted-foreground)]")}>All roles</button>
        {roles.map((role) => (
          <button key={role.slug} type="button" onClick={() => toggle(role.slug)} className={cn("rounded-lg border px-2.5 py-1 text-[11px]", selected.includes(role.slug) ? "border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--brand)]" : "border-[var(--border)] text-[var(--muted-foreground)]")}>{role.name}</button>
        ))}
      </div>
    </div>
  );
}
