"use client";

import { useEffect, useState, useCallback } from "react";

export function useTheme() {
  const [dark, setDark] = useState(false);

  useEffect(() => {
    const stored = localStorage.getItem("morfoschools-theme");
    if (stored === "dark") {
      setDark(true);
      document.documentElement.setAttribute("data-theme", "dark");
    }
  }, []);

  const toggle = useCallback(() => {
    setDark((prev) => {
      const next = !prev;
      if (next) {
        document.documentElement.setAttribute("data-theme", "dark");
        localStorage.setItem("morfoschools-theme", "dark");
      } else {
        document.documentElement.removeAttribute("data-theme");
        localStorage.setItem("morfoschools-theme", "light");
      }
      return next;
    });
  }, []);

  return { dark, toggle };
}
