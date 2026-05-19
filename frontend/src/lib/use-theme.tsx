"use client";

import { useEffect, useState, useCallback, createContext, useContext } from "react";

export type ShellMode = "dark" | "light";
export type AccentColor = "blue" | "violet" | "emerald" | "rose" | "amber" | "indigo";

interface ThemeState {
  dark: boolean;
  shellMode: ShellMode;
  accent: AccentColor;
}

interface ThemeContextValue extends ThemeState {
  toggle: () => void;
  setShellMode: (mode: ShellMode) => void;
  setAccent: (color: AccentColor) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

const STORAGE_KEY = "morfoschools-theme";
const SHELL_KEY = "morfoschools-shell-mode";
const ACCENT_KEY = "morfoschools-accent";

const accentTokens: Record<AccentColor, { brand: string; brandStrong: string; brandSoft: string }> = {
  blue: {
    brand: "oklch(62.3% 0.214 259.815)",
    brandStrong: "oklch(54.6% 0.245 262.881)",
    brandSoft: "oklch(97% 0.014 254.604)",
  },
  violet: {
    brand: "oklch(60.6% 0.25 292.717)",
    brandStrong: "oklch(54.1% 0.281 293.009)",
    brandSoft: "oklch(96.9% 0.016 293.756)",
  },
  emerald: {
    brand: "oklch(69.6% 0.17 162.48)",
    brandStrong: "oklch(60% 0.118 184.704)",
    brandSoft: "oklch(98.2% 0.018 155.826)",
  },
  rose: {
    brand: "oklch(65.6% 0.241 354.308)",
    brandStrong: "oklch(63.7% 0.237 25.331)",
    brandSoft: "oklch(97.1% 0.013 17.38)",
  },
  amber: {
    brand: "oklch(76.9% 0.188 70.08)",
    brandStrong: "oklch(68.1% 0.162 75.834)",
    brandSoft: "oklch(98.7% 0.026 102.212)",
  },
  indigo: {
    brand: "oklch(58.5% 0.233 277.117)",
    brandStrong: "oklch(48.8% 0.243 264.376)",
    brandSoft: "oklch(96.9% 0.016 293.756)",
  },
};

function applyShellMode(mode: ShellMode) {
  document.documentElement.setAttribute("data-shell", mode);
}

function applyAccent(color: AccentColor) {
  const tokens = accentTokens[color];
  const root = document.documentElement;
  root.style.setProperty("--brand", tokens.brand);
  root.style.setProperty("--brand-strong", tokens.brandStrong);
  root.style.setProperty("--brand-soft", tokens.brandSoft);
  root.setAttribute("data-accent", color);
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<ThemeState>({
    dark: false,
    shellMode: "dark",
    accent: "blue",
  });

  useEffect(() => {
    const storedTheme = localStorage.getItem(STORAGE_KEY);
    const storedShell = localStorage.getItem(SHELL_KEY) as ShellMode | null;
    const storedAccent = localStorage.getItem(ACCENT_KEY) as AccentColor | null;

    const dark = storedTheme === "dark";
    const shellMode = storedShell || "dark";
    const accent = storedAccent || "blue";

    if (dark) document.documentElement.setAttribute("data-theme", "dark");
    applyShellMode(shellMode);
    applyAccent(accent);

    setState({ dark, shellMode, accent });
  }, []);

  const toggle = useCallback(() => {
    setState((prev) => {
      const next = !prev.dark;
      if (next) {
        document.documentElement.setAttribute("data-theme", "dark");
        localStorage.setItem(STORAGE_KEY, "dark");
      } else {
        document.documentElement.removeAttribute("data-theme");
        localStorage.setItem(STORAGE_KEY, "light");
      }
      return { ...prev, dark: next };
    });
  }, []);

  const setShellMode = useCallback((mode: ShellMode) => {
    setState((prev) => {
      applyShellMode(mode);
      localStorage.setItem(SHELL_KEY, mode);
      return { ...prev, shellMode: mode };
    });
  }, []);

  const setAccent = useCallback((color: AccentColor) => {
    setState((prev) => {
      applyAccent(color);
      localStorage.setItem(ACCENT_KEY, color);
      return { ...prev, accent: color };
    });
  }, []);

  return (
    <ThemeContext.Provider value={{ ...state, toggle, setShellMode, setAccent }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider");
  return ctx;
}
