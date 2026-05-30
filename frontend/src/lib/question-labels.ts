export function questionTypeLabel(type?: string | null) {
  switch (type) {
    case "multiple_choice":
      return "Pilihan Ganda";
    case "true_false":
      return "Benar/Salah";
    case "short_answer":
      return "Isian Singkat";
    case "essay":
      return "Uraian";
    default:
      return type || "-";
  }
}

export function questionTypeShortLabel(type?: string | null) {
  switch (type) {
    case "multiple_choice":
      return "PG";
    case "true_false":
      return "B/S";
    case "short_answer":
      return "Isian";
    case "essay":
      return "Uraian";
    default:
      return type || "-";
  }
}
