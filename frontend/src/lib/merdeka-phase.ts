export function phaseForGradeLevel(gradeLevel: string): string | null {
  const n = Number(String(gradeLevel).match(/\d+/)?.[0] || "");
  if (!Number.isFinite(n) || n <= 0) return null;
  if (n === 1 || n === 2) return "a";
  if (n === 3 || n === 4) return "b";
  if (n === 5 || n === 6) return "c";
  if (n >= 7 && n <= 9) return "d";
  if (n === 10) return "e";
  if (n === 11 || n === 12) return "f";
  return null;
}

export function tenantPhasesFromGrades(grades: string[]): string[] {
  const order = ["a", "b", "c", "d", "e", "f"];
  const set = new Set(grades.map(phaseForGradeLevel).filter(Boolean) as string[]);
  return order.filter((p) => set.has(p));
}

export function gradeOptionsFromLevels(grades: string[]) {
  return [...new Set(grades)].sort((a, b) => Number(a.match(/\d+/)?.[0] || 0) - Number(b.match(/\d+/)?.[0] || 0));
}
