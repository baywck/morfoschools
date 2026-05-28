const PHASE_GRADES: Record<string, string[]> = {
  a: ["1", "2"],
  b: ["3", "4"],
  c: ["5", "6"],
  d: ["7", "8", "9"],
  e: ["10"],
  f: ["11", "12"],
};

export function gradeOptionsForPhases(phases: string[]) {
  const seen = new Set<string>();
  const grades: string[] = [];
  for (const phase of phases) {
    for (const grade of PHASE_GRADES[phase.toLowerCase()] || []) {
      if (!seen.has(grade)) {
        seen.add(grade);
        grades.push(grade);
      }
    }
  }
  return grades;
}

export function phaseForGrade(grade: string) {
  for (const [phase, grades] of Object.entries(PHASE_GRADES)) {
    if (grades.includes(grade)) return phase;
  }
  return "";
}
