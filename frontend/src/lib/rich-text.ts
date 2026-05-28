import { isHtmlContent } from "@/components/ui/rendered-content";

export function escapeHtmlText(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

/**
 * Normalize persisted rich text before feeding it into the editor.
 *
 * New rows are stored as editor HTML. Legacy/AI rows may still be plain
 * text; TipTap collapses plain-text newlines when inserted directly, so
 * convert them into paragraph HTML first. RenderedContent has the same
 * plain-text fallback semantics, which keeps editor and preview aligned.
 */
export function normalizeRichTextForEditor(raw: string | null | undefined): string {
  if (!raw) return "";
  if (isHtmlContent(raw)) return raw;

  const escaped = escapeHtmlText(raw);
  return escaped
    .split(/\n{2,}/)
    .map((para) => `<p>${para.replace(/\n/g, "<br>")}</p>`)
    .join("");
}
