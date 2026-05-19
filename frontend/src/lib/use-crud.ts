"use client";

import { useState, useEffect, useCallback } from "react";
import { useToast } from "@/components/ui/toast";
import type { ApiResponse } from "@/lib/api-client";

/* ─── Types ─── */

interface PaginatedResponse<T> {
  data: T[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

interface UseCRUDOptions<T, TCreate = any, TUpdate = any> {
  /** Display name for toast messages (e.g. "Teacher") */
  name: string;
  /** API function to list items */
  list: (params?: { search?: string; page?: number }) => Promise<ApiResponse<PaginatedResponse<T>>>;
  /** API function to create item */
  create?: (data: TCreate) => Promise<ApiResponse<any>>;
  /** API function to update item */
  update?: (id: string, data: TUpdate) => Promise<ApiResponse<any>>;
  /** API function to archive item */
  archive?: (id: string) => Promise<ApiResponse<any>>;
  /** API function to restore an archived item */
  restore?: (id: string) => Promise<ApiResponse<any>>;
  /** Extract ID from item */
  getId?: (item: T) => string;
}

interface CRUDState<T> {
  /* List */
  items: T[];
  total: number;
  loading: boolean;
  search: string;
  setSearch: (v: string) => void;
  reload: () => void;

  /* Create */
  showCreate: boolean;
  setShowCreate: (v: boolean) => void;
  creating: boolean;
  handleCreate: (data: any) => Promise<boolean>;

  /* Edit */
  editTarget: T | null;
  setEditTarget: (item: T | null) => void;
  editing: boolean;
  handleEdit: (id: string, data: any) => Promise<boolean>;

  /* Archive */
  archiveTarget: T | null;
  setArchiveTarget: (item: T | null) => void;
  archiving: boolean;
  handleArchive: (id: string) => Promise<boolean>;

  /* Restore */
  restoring: boolean;
  handleRestore: (id: string) => Promise<boolean>;

  /* Errors */
  fieldErrors: Record<string, string>;
  setFieldErrors: (errors: Record<string, string>) => void;
  clearFieldError: (field: string) => void;
}

/* ─── Hook ─── */

export function useCRUD<T, TCreate = any, TUpdate = any>(
  options: UseCRUDOptions<T, TCreate, TUpdate>
): CRUDState<T> {
  const { name, list, create, update, archive, restore } = options;
  const { toast } = useToast();

  // List state
  const [items, setItems] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  // Create state
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);

  // Edit state
  const [editTarget, setEditTarget] = useState<T | null>(null);
  const [editing, setEditing] = useState(false);

  // Archive state
  const [archiveTarget, setArchiveTarget] = useState<T | null>(null);
  const [archiving, setArchiving] = useState(false);

  // Restore state
  const [restoring, setRestoring] = useState(false);

  // Errors
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const clearFieldError = useCallback((field: string) => {
    setFieldErrors((prev) => {
      if (!prev[field]) return prev;
      const { [field]: _, ...rest } = prev;
      return rest;
    });
  }, []);

  // Load
  const reload = useCallback(async () => {
    setLoading(true);
    const res = await list({ search: search || undefined });
    if (res.data) {
      setItems(res.data.data);
      setTotal(res.data.pagination.total);
    }
    setLoading(false);
  }, [list, search]);

  useEffect(() => { reload(); }, [reload]);

  // Listen for data changes from AI chatbot
  useEffect(() => {
    function handleDataChanged() { reload(); }
    window.addEventListener("morfoschools:data-changed", handleDataChanged);
    return () => window.removeEventListener("morfoschools:data-changed", handleDataChanged);
  }, [reload]);

  // Create
  const handleCreate = useCallback(async (data: any): Promise<boolean> => {
    if (!create) return false;
    setFieldErrors({});
    setCreating(true);
    const res = await create(data);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return false;
    }
    toast({ tone: "success", title: `${name} created` });
    setShowCreate(false);
    setCreating(false);
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
    return true;
  }, [create, name, toast, reload]);

  // Edit
  const handleEdit = useCallback(async (id: string, data: any): Promise<boolean> => {
    if (!update) return false;
    setFieldErrors({});
    setEditing(true);
    const res = await update(id, data);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return false;
    }
    toast({ tone: "success", title: `${name} updated` });
    setEditTarget(null);
    setEditing(false);
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
    return true;
  }, [update, name, toast, reload]);

  // Archive
  const handleArchive = useCallback(async (id: string): Promise<boolean> => {
    if (!archive) return false;
    setArchiving(true);
    const res = await archive(id);
    setArchiving(false);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
      return false;
    }
    toast({ tone: "success", title: `${name} archived` });
    setArchiveTarget(null);
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
    return true;
  }, [archive, name, toast, reload]);

  // Restore
  const handleRestore = useCallback(async (id: string): Promise<boolean> => {
    if (!restore) return false;
    setRestoring(true);
    const res = await restore(id);
    setRestoring(false);
    if (res.error) {
      // Backend returns a structured `fields` error when the original email
      // is taken. We don't have an inline form for restore, so surface the
      // hint as a toast and let the admin resolve it via the user edit flow.
      const emailMsg = res.error.fields?.email;
      toast({
        tone: "error",
        title: "Restore failed",
        description: emailMsg || res.error.message,
      });
      return false;
    }
    toast({ tone: "success", title: `${name} restored` });
    reload();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
    return true;
  }, [restore, name, toast, reload]);

  return {
    items, total, loading, search, setSearch, reload,
    showCreate, setShowCreate, creating, handleCreate,
    editTarget, setEditTarget, editing, handleEdit,
    archiveTarget, setArchiveTarget, archiving, handleArchive,
    restoring, handleRestore,
    fieldErrors, setFieldErrors, clearFieldError,
  };
}
