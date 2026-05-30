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

import { useEffect, useMemo, useRef } from "react";
import DOMPurify from "dompurify";
import katex from "katex";
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
  const containerRef = useRef<HTMLDivElement | null>(null);
  const sanitized = useMemo(() => {
    if (!html) return "";
    // Plain text path: escape HTML first, THEN run LaTeX preprocess so
    // dollar delimiters survive escaping (\$ stays \$ in the escaped
    // string). Wrapping in <p style=pre-wrap> preserves user newlines
    // for prose paragraphs.
    const prepared = isHtmlContent(html)
      ? preprocessLatexDelimiters(html)
      : preprocessLatexDelimiters(
          `<p style="white-space: pre-wrap;">${escapeHtml(html)}</p>`,
        );
    return DOMPurify.sanitize(prepared, SANITIZER_CONFIG as DomPurifyConfig);
  }, [html]);

  // After the sanitized HTML mounts, walk every math-node span and
  // render KaTeX into it. The math extension only renders inside
  // the TipTap editor (NodeViewRenderer); for read-only consumers we
  // need a one-shot render here so the saved HTML displays properly.
  useEffect(() => {
    const root = containerRef.current;
    if (!root) return;
    const nodes = root.querySelectorAll<HTMLSpanElement>('span[data-type="inlineMath"][data-latex]');
    nodes.forEach((node) => {
      if (node.dataset.katexRendered === "1") return;
      const latex = node.getAttribute("data-latex") ?? "";
      const display = node.getAttribute("data-display") === "yes";
      try {
        katex.render(latex, node, {
          displayMode: display,
          throwOnError: false,
          output: "html",
        });
        node.dataset.katexRendered = "1";
      } catch {
        // Leave latex source visible if KaTeX can't parse — better
        // than blanking the slot silently.
        node.textContent = display ? `$$${latex}$$` : `$${latex}$`;
      }
    });
  }, [sanitized]);

  if (!sanitized) {
    return (
      <span className={cn("italic text-[var(--muted-foreground)]", className)}>
        (kosong)
      </span>
    );
  }

  return (
    <div
      ref={containerRef}
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

/**
 * Pre-render raw `$...$` / `$$...$$` LaTeX delimiters into the math
 * node span shape used by @aarkue/tiptap-math-extension. Mirror of the
 * preprocessor in rich-editor.tsx so saved + raw content display
 * identically.
 */
function preprocessLatexDelimiters(html: string): string {
  if (!html || !html.includes("$")) return html;
  if (html.includes('data-type="inlineMath"')) return html;
  const guardRe = /(<code\b[^>]*>[\s\S]*?<\/code>|<pre\b[^>]*>[\s\S]*?<\/pre>|<span\b[^>]*data-type="inlineMath"[^>]*>[\s\S]*?<\/span>)/gi;
  const escapeAttr = (s: string) =>
    s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const transformChunk = (chunk: string): string => {
    chunk = chunk.replace(/\$\$([\s\S]+?)\$\$/g, (_, latex: string) => {
      const trimmed = latex.trim();
      if (!trimmed) return `$$${latex}$$`;
      return `<span data-type="inlineMath" data-latex="${escapeAttr(trimmed)}" data-display="yes"></span>`;
    });
    chunk = chunk.replace(/(?<![\\$])\$(?!\s)([^\n$]+?)(?<!\s)\$(?!\$)/g, (_, latex: string) => {
      const trimmed = latex.trim();
      if (!trimmed) return `$${latex}$`;
      return `<span data-type="inlineMath" data-latex="${escapeAttr(trimmed)}" data-display="no"></span>`;
    });
    return chunk;
  };
  const out: string[] = [];
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = guardRe.exec(html)) !== null) {
    if (m.index > last) out.push(transformChunk(html.slice(last, m.index)));
    out.push(m[0]);
    last = m.index + m[0].length;
  }
  if (last < html.length) out.push(transformChunk(html.slice(last)));
  return out.join("");
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
