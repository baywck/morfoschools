import type { SchoolType, Tenant } from "@/lib/modules-api";
import { KEMENDIKDASMEN_SUBJECTS } from "@/lib/kemendikdasmen-subjects";

export const SCHOOL_TYPE_DEFAULTS: Record<SchoolType, { phases: string[]; includeVocationalSubjects: boolean }> = {
  sd: { phases: ["a", "b", "c"], includeVocationalSubjects: false },
  smp: { phases: ["d"], includeVocationalSubjects: false },
  sma: { phases: ["e", "f"], includeVocationalSubjects: false },
  smk: { phases: ["e", "f"], includeVocationalSubjects: true },
  mixed: { phases: ["e", "f"], includeVocationalSubjects: false },
};

export function tenantEnabledPhases(tenant?: Pick<Tenant, "schoolType" | "enabledPhases"> | null) {
  if (tenant?.enabledPhases?.length) return tenant.enabledPhases.map((phase) => phase.toLowerCase());
  return SCHOOL_TYPE_DEFAULTS[tenant?.schoolType || "sma"].phases;
}

export function tenantIncludesVocationalSubjects(tenant?: Pick<Tenant, "schoolType" | "includeVocationalSubjects"> | null) {
  if (!tenant) return false;
  return tenant.schoolType === "smk" || !!tenant.includeVocationalSubjects;
}

export function subjectsForTenant(tenant?: Pick<Tenant, "schoolType" | "includeVocationalSubjects"> | null) {
  const includeVocational = tenantIncludesVocationalSubjects(tenant);
  return KEMENDIKDASMEN_SUBJECTS.filter((subject) => includeVocational || subject.level !== "smk");
}
