"use client";

/**
 * RenderedContent — read-only renderer for HTML produced by the
 * RichEditor. Sanitizes via DOMPurify before injecting; KaTeX styles
 * are loaded once at module level so math rendered into the HTML
 * (span.katex / span.katex-display) keeps its layout.
 *
 * Plain-text fallback: legacy questions stored content as raw text.
 * isHtmlContent() inspects the leading characters; when the string
 * doesn't look like HTML we wrap it in a <p> with whitespace
 * preservation so line breaks survive.
 */

import { useMemo } from "react";
import DOMPurify from "dompurify";
import "katex/dist/katex.min.css";
import { cn } from "@/lib/cn";

export interface RenderedContentProps {
  html: string;
  className?: string;
}

const SANITIZER_CONFIG = {
  ALLOWED_TAGS: [
    "p",
    "br",
    "strong",
    "em",
    "u",
    "s",
    "code",
    "pre",
    "blockquote",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "ul",
    "ol",
    "li",
    "a",
    "img",
    "span",
    "div",
    "hr",
    "table",
    "thead",
    "tbody",
    "tr",
    "th",
    "td",
    // KaTeX-rendered math leaves behind these. Allow them all so the
    // rendered output keeps its layout — DOMPurify still strips event
    // handlers, scripts, and unknown protocols.
    "math",
    "annotation",
    "semantics",
    "mrow",
    "mi",
    "mo",
    "mn",
    "ms",
    "mtext",
    "mspace",
    "mfrac",
    "msup",
    "msub",
    "msubsup",
    "msqrt",
    "mroot",
    "munder",
    "mover",
    "munderover",
    "mtable",
    "mtr",
    "mtd",
    "mstyle",
    "mpadded",
    "mphantom",
    "mglyph",
    "menclose",
  ],
  ALLOWED_ATTR: [
    "href",
    "src",
    "alt",
    "title",
    "target",
    "rel",
    "class",
    "style",
    "data-latex",
    "data-evaluate",
    "data-display",
    "data-type",
    "aria-hidden",
    "aria-label",
    "role",
    "colspan",
    "rowspan",
    "scope",
  ],
  ALLOW_DATA_ATTR: true,
};

// DOMPurify 3.x ships its own typings; @types/dompurify has stale
// types that disagree with the runtime API. We cast at the call site
// so the strict ALLOWED_ATTR list keeps its narrow literal types
// without leaking that into DOMPurify's union.
type DomPurifyConfig = Parameters<typeof DOMPurify.sanitize>[1];

export function isHtmlContent(s: string): boolean {
  if (!s) return false;
  const trimmed = s.trimStart();
  return trimmed.startsWith("<");
}

export function RenderedContent({ html, className }: RenderedContentProps) {
  const sanitized = useMemo(() => {
    if (!html) return "";
    const source = isHtmlContent(html)
      ? html
      : `<p style="white-space: pre-wrap;">${escapeHtml(html)}</p>`;
    return DOMPurify.sanitize(source, SANITIZER_CONFIG as DomPurifyConfig);
  }, [html]);

  if (!sanitized) {
    return (
      <span className={cn("italic text-[var(--muted-foreground)]", className)}>
        (kosong)
      </span>
    );
  }

  return (
    <div
      className={cn(
        "prose prose-sm max-w-none text-[13px] leading-relaxed text-[var(--foreground)]",
        "[&_p]:my-1 [&_h2]:mt-2 [&_h2]:mb-1 [&_h3]:mt-2 [&_h3]:mb-1",
        "[&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5",
        "[&_a]:text-[var(--brand)] [&_a]:underline",
        "[&_img]:rounded-md [&_img]:max-w-full",
        className,
      )}
      // sanitized via DOMPurify above; safe to inject.
      dangerouslySetInnerHTML={{ __html: sanitized }}
    />
  );
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

/**
 * Strip HTML tags from a value for use in a compact preview slot
 * (collapsed accordion header). Preserves spaces between block
 * elements and truncates to maxLen with an ellipsis.
 */
export function stripHtmlPreview(value: string, maxLen = 120): string {
  if (!value) return "";
  if (!isHtmlContent(value)) {
    return value.length > maxLen ? value.slice(0, maxLen - 1) + "…" : value;
  }
  // Insert a space before opening block tags so words don't fuse when
  // tags are stripped, then drop tags and collapse whitespace.
  const withSpaces = value.replace(/<\/(p|div|li|h[1-6]|br)>/gi, " ");
  const stripped = withSpaces.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
  if (!stripped) return "";
  return stripped.length > maxLen
    ? stripped.slice(0, maxLen - 1) + "…"
    : stripped;
}
