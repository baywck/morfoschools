"use client";

/**
 * RichEditor — TipTap-based rich text editor with LaTeX support.
 *
 * Used by the exam authoring surfaces (question content, explanation,
 * stimulus body) so teachers can paste formatted prose and inline math
 * notation. The editor stores HTML via TipTap's getHTML(); KaTeX-
 * rendered math is preserved in the document JSON via the math
 * extension's data attributes so re-loading the HTML re-renders the
 * formula intact.
 *
 * Storage shape: HTML string. Display-side rendering uses the
 * companion RenderedContent component (DOMPurify-sanitized).
 *
 * Toolbar: Bold / Italic / Underline / Bullet / Numbered / H2 / H3 /
 * Link / Image / Math (inline) / Math (block) / Clear formatting.
 */

import { useEffect, useRef } from "react";
import { useEditor, EditorContent, type Editor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import Placeholder from "@tiptap/extension-placeholder";
import { MathExtension } from "@aarkue/tiptap-math-extension";
import "katex/dist/katex.min.css";
import { usePrompt } from "@/components/ui/prompt-dialog";
import {
  Bold,
  Italic,
  Underline as UnderlineIcon,
  List,
  ListOrdered,
  Heading2,
  Heading3,
  Link as LinkIcon,
  Image as ImageIcon,
  Sigma,
  FunctionSquare,
  Eraser,
} from "lucide-react";
import { cn } from "@/lib/cn";

export interface RichEditorProps {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
  minRows?: number;
  error?: string;
  disabled?: boolean;
  /** Optional aria label for the editor surface. */
  ariaLabel?: string;
}

const TOOLBAR_BTN_BASE =
  "inline-flex h-7 w-7 items-center justify-center rounded-md text-[var(--muted-foreground)] transition-colors hover:bg-[var(--muted)] hover:text-[var(--foreground)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand)]/40 disabled:cursor-not-allowed disabled:opacity-40";

const TOOLBAR_BTN_ACTIVE =
  "bg-[var(--brand-soft)] text-[var(--brand)] hover:bg-[var(--brand-soft)] hover:text-[var(--brand)]";

/**
 * Pre-render raw `$...$` / `$$...$$` LaTeX delimiters in saved HTML
 * into the math-node span shape that @aarkue/tiptap-math-extension
 * expects. Without this, content saved as plain text containing
 * dollar-delimited LaTeX (typical for AI-generated questions or
 * legacy data) is loaded into the editor verbatim and never
 * upgraded into a math node — so KaTeX never renders.
 *
 * Strategy:
 *  - Walk the HTML as text outside of existing math node spans
 *    (avoid double-wrapping pre-converted nodes)
 *  - Replace `$$ ... $$` with display=yes spans first (longer wins)
 *  - Then replace `$ ... $` with inline (display=no) spans
 *  - Skip code/pre regions (LaTeX inside code blocks is intentional)
 *  - Escape attribute payload so quotes in latex don't break HTML
 */
function preprocessLatexDelimiters(html: string): string {
  if (!html) return html;
  if (!html.includes("$")) return html;

  // Skip if already contains math nodes — saved-by-editor content
  // already has the proper spans, no need to re-walk.
  if (html.includes('data-type="inlineMath"')) return html;

  // Tokenize on existing tags to avoid mutating attributes / inner HTML
  // of code blocks. Cheap heuristic: split on `<code...>...</code>`,
  // `<pre...>...</pre>`, and existing `<span data-type="inlineMath"...>...</span>`
  // and only transform the gaps between.
  const guardRe = /(<code\b[^>]*>[\s\S]*?<\/code>|<pre\b[^>]*>[\s\S]*?<\/pre>|<span\b[^>]*data-type="inlineMath"[^>]*>[\s\S]*?<\/span>)/gi;

  const escapeAttr = (s: string) =>
    s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

  const transformChunk = (chunk: string): string => {
    // Display math: $$ ... $$ (non-greedy, must not contain inner $$)
    chunk = chunk.replace(/\$\$([\s\S]+?)\$\$/g, (_, latex: string) => {
      const trimmed = latex.trim();
      if (!trimmed) return `$$${latex}$$`;
      return `<span data-type="inlineMath" data-latex="${escapeAttr(trimmed)}" data-display="yes"></span>`;
    });
    // Inline math: $ ... $ (single-line, no nested $, no leading/trailing space)
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
    if (m.index > last) {
      out.push(transformChunk(html.slice(last, m.index)));
    }
    out.push(m[0]);
    last = m.index + m[0].length;
  }
  if (last < html.length) {
    out.push(transformChunk(html.slice(last)));
  }
  return out.join("");
}

/**
 * Detect whether a string is HTML (vs plain text). Same heuristic
 * RenderedContent uses — leading character is `<`.
 */
function looksLikeHtml(s: string): boolean {
  if (!s) return false;
  return s.trimStart().startsWith("<");
}

/**
 * Composed preprocessor for any value flowing INTO TipTap. Handles:
 *  1. Plain text from AI / legacy storage — escape HTML, split on
 *     blank lines into <p> blocks, single \n becomes <br>
 *  2. Raw `$...$` / `$$...$$` delimiters — expand into math node spans
 *
 * This is the single entry point for all RichEditor content so every
 * consumer (question editor, stimulus editor, options) benefits.
 * Exported so other surfaces (e.g. read-only viewers built on the
 * same input shape) can reuse it.
 */
export function prepareEditorContent(raw: string): string {
  if (!raw) return "";
  let html = raw;
  if (!looksLikeHtml(html)) {
    const escapeText = (s: string) =>
      s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
    html = html
      .split(/\n{2,}/)
      .map((para) => {
        const escaped = escapeText(para);
        return `<p>${escaped.replace(/\n/g, "<br>")}</p>`;
      })
      .join("");
  }
  return preprocessLatexDelimiters(html);
}

export function RichEditor({
  value,
  onChange,
  placeholder,
  minRows = 3,
  error,
  disabled = false,
  ariaLabel,
}: RichEditorProps) {
  // Track the last value emitted from the editor so external resets
  // (e.g. parent wipes the value back to "") force a setContent without
  // triggering an infinite onChange loop.
  const lastEmittedRef = useRef<string>(value);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [2, 3] },
      }),
      Image.configure({ inline: false, allowBase64: false }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: { target: "_blank", rel: "noopener noreferrer" },
      }),
      Placeholder.configure({
        placeholder: placeholder ?? "Tulis di sini...",
        emptyEditorClass:
          "before:content-[attr(data-placeholder)] before:text-[var(--muted-foreground)] before:float-left before:pointer-events-none before:h-0",
      }),
      MathExtension.configure({
        evaluation: false,
        addInlineMath: true,
        renderTextMode: "raw-latex",
      }),
    ],
    content: prepareEditorContent(value || ""),
    editable: !disabled,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        class: cn(
          "prose prose-sm max-w-none w-full px-3 py-2 text-[13px] leading-relaxed text-[var(--foreground)] outline-none",
          "[&_p]:my-1 [&_h2]:mt-2 [&_h2]:mb-1 [&_h3]:mt-2 [&_h3]:mb-1",
          "[&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5",
          "[&_a]:text-[var(--brand)] [&_a]:underline",
          "[&_img]:rounded-md [&_img]:max-w-full",
        ),
        ...(ariaLabel ? { "aria-label": ariaLabel } : {}),
      },
    },
    onUpdate: ({ editor: ed }) => {
      const html = ed.getHTML();
      lastEmittedRef.current = html;
      onChange(html);
    },
  });

  // External value sync — when the parent forcibly resets the value
  // (e.g. clearing a draft), push it into the editor without nuking
  // the user's mid-edit state.
  useEffect(() => {
    if (!editor) return;
    if (value === lastEmittedRef.current) return;
    if (value === editor.getHTML()) return;
    editor.commands.setContent(prepareEditorContent(value || ""), false);
    lastEmittedRef.current = value;
  }, [value, editor]);

  // Disabled state passes through to TipTap.
  useEffect(() => {
    if (!editor) return;
    editor.setOptions({ editable: !disabled });
  }, [disabled, editor]);

  const minHeight = Math.max(minRows, 2) * 22 + 16;

  return (
    <div
      className={cn(
        "rounded-lg border bg-[var(--card)] transition-colors",
        error
          ? "border-[var(--danger)] focus-within:border-[var(--danger)]"
          : "border-[var(--border)] focus-within:border-[var(--brand)] focus-within:ring-2 focus-within:ring-[var(--field-ring)]",
        disabled && "opacity-60 cursor-not-allowed",
      )}
    >
      <Toolbar editor={editor} disabled={disabled} />
      <div
        className="overflow-y-auto"
        style={{ minHeight }}
        onClick={() => editor?.chain().focus().run()}
      >
        <EditorContent editor={editor} />
      </div>
      {error && (
        <p
          className="border-t border-[var(--danger)]/30 px-3 py-1.5 text-[11px] font-medium text-[var(--danger)]"
          role="alert"
        >
          {error}
        </p>
      )}
    </div>
  );
}

function Toolbar({
  editor,
  disabled,
}: {
  editor: Editor | null;
  disabled: boolean;
}) {
  const prompt = usePrompt();
  if (!editor) {
    return (
      <div
        className="flex items-center gap-0.5 border-b border-[var(--border)] bg-[var(--accent)]/40 px-1.5 py-1"
        aria-hidden
      >
        <div className="h-7 w-full" />
      </div>
    );
  }
  // After the early return TypeScript can't narrow `editor` inside
  // nested closures, so we re-bind it to a local const.
  const ed: Editor = editor;

  function btn(props: {
    onClick: () => void;
    active?: boolean;
    label: string;
    icon: React.ReactNode;
  }) {
    return (
      <button
        type="button"
        onClick={props.onClick}
        disabled={disabled}
        aria-pressed={!!props.active}
        aria-label={props.label}
        title={props.label}
        className={cn(TOOLBAR_BTN_BASE, props.active && TOOLBAR_BTN_ACTIVE)}
      >
        {props.icon}
      </button>
    );
  }

  async function handleLink() {
    const previous = ed.getAttributes("link").href as string | undefined;
    const url = await prompt({
      title: previous ? "Edit link" : "Tambah link",
      label: "URL",
      defaultValue: previous ?? "https://",
      placeholder: "https://example.com",
      confirmLabel: previous ? "Update" : "Tambah",
    });
    if (url === null) return;
    if (url === "") {
      ed.chain().focus().extendMarkRange("link").unsetLink().run();
      return;
    }
    ed.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
  }

  async function handleImage() {
    const url = await prompt({
      title: "Tambah gambar",
      label: "URL gambar",
      defaultValue: "https://",
      placeholder: "https://example.com/image.png",
      confirmLabel: "Tambah",
    });
    if (!url) return;
    ed.chain().focus().setImage({ src: url }).run();
  }

  async function handleInlineMath() {
    const latex = await prompt({
      title: "Math (inline)",
      description: "Tulis ekspresi LaTeX. Akan dirender inline dengan teks.",
      label: "LaTeX",
      defaultValue: "x^2 + y^2 = z^2",
      placeholder: "x^2 + y^2 = z^2",
      confirmLabel: "Sisipkan",
      multiline: true,
    });
    if (!latex) return;
    // Insert delimiter-wrapped text so the math extension's input
    // rule converts it into an inline-math node.
    ed.chain().focus().insertContent(`$${latex}$`).run();
  }

  async function handleBlockMath() {
    const latex = await prompt({
      title: "Math (block)",
      description:
        "Tulis ekspresi LaTeX. Akan dirender sebagai block math (centered, baris sendiri).",
      label: "LaTeX",
      defaultValue: "\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}",
      placeholder: "\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}",
      confirmLabel: "Sisipkan",
      multiline: true,
    });
    if (!latex) return;
    ed.chain().focus().insertContent(`$$${latex}$$`).run();
  }

  return (
    <div
      className="flex flex-wrap items-center gap-0.5 border-b border-[var(--border)] bg-[var(--accent)]/40 px-1.5 py-1"
      role="toolbar"
      aria-label="Format teks"
    >
      {btn({
        onClick: () => ed.chain().focus().toggleBold().run(),
        active: ed.isActive("bold"),
        label: "Bold",
        icon: <Bold size={13} />,
      })}
      {btn({
        onClick: () => ed.chain().focus().toggleItalic().run(),
        active: ed.isActive("italic"),
        label: "Italic",
        icon: <Italic size={13} />,
      })}
      {btn({
        onClick: () => ed.chain().focus().toggleStrike().run(),
        active: ed.isActive("strike"),
        label: "Strikethrough",
        icon: <UnderlineIcon size={13} />,
      })}
      <span className="mx-0.5 h-4 w-px bg-[var(--border)]" aria-hidden />
      {btn({
        onClick: () =>
          ed.chain().focus().toggleHeading({ level: 2 }).run(),
        active: ed.isActive("heading", { level: 2 }),
        label: "Heading 2",
        icon: <Heading2 size={13} />,
      })}
      {btn({
        onClick: () =>
          ed.chain().focus().toggleHeading({ level: 3 }).run(),
        active: ed.isActive("heading", { level: 3 }),
        label: "Heading 3",
        icon: <Heading3 size={13} />,
      })}
      <span className="mx-0.5 h-4 w-px bg-[var(--border)]" aria-hidden />
      {btn({
        onClick: () => ed.chain().focus().toggleBulletList().run(),
        active: ed.isActive("bulletList"),
        label: "Bullet list",
        icon: <List size={13} />,
      })}
      {btn({
        onClick: () => ed.chain().focus().toggleOrderedList().run(),
        active: ed.isActive("orderedList"),
        label: "Numbered list",
        icon: <ListOrdered size={13} />,
      })}
      <span className="mx-0.5 h-4 w-px bg-[var(--border)]" aria-hidden />
      {btn({
        onClick: handleLink,
        active: ed.isActive("link"),
        label: "Tambah link",
        icon: <LinkIcon size={13} />,
      })}
      {btn({
        onClick: handleImage,
        label: "Tambah gambar (URL)",
        icon: <ImageIcon size={13} />,
      })}
      <span className="mx-0.5 h-4 w-px bg-[var(--border)]" aria-hidden />
      {btn({
        onClick: handleInlineMath,
        label: "Math (inline)",
        icon: <Sigma size={13} />,
      })}
      {btn({
        onClick: handleBlockMath,
        label: "Math (block)",
        icon: <FunctionSquare size={13} />,
      })}
      <span className="mx-0.5 h-4 w-px bg-[var(--border)]" aria-hidden />
      {btn({
        onClick: () =>
          ed.chain().focus().clearNodes().unsetAllMarks().run(),
        label: "Hapus formatting",
        icon: <Eraser size={13} />,
      })}
    </div>
  );
}
