import { get, post, patch } from "./api-client";

// --- Users ---
export interface User {
  id: string;
  email: string;
  displayName: string;
  status: string;
  isPlatformAdmin: boolean;
  createdAt: string;
}

export interface UserListResponse {
  data: User[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listUsers(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<UserListResponse>(`/api/v1/users${qs ? `?${qs}` : ""}`);
}

export function createUser(data: { email: string; displayName: string; password: string; roleSlug?: string }) {
  return post<User>("/api/v1/users", data);
}

export function updateUser(id: string, data: { displayName?: string; status?: string }) {
  return patch<User>(`/api/v1/users/${id}`, data);
}

export function archiveUser(id: string) {
  return patch<{ status: string }>(`/api/v1/users/${id}/archive`);
}

// --- Tenants ---
export interface Tenant {
  id: string;
  name: string;
  code: string;
  status: string;
  logoUrl: string | null;
  createdAt: string;
}

export interface TenantListResponse {
  data: Tenant[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listTenants(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<TenantListResponse>(`/api/v1/tenants${qs ? `?${qs}` : ""}`);
}

export function createTenant(data: { name: string; code: string }) {
  return post<Tenant>("/api/v1/tenants", data);
}

export function updateTenant(id: string, data: { name?: string; status?: string }) {
  return patch<Tenant>(`/api/v1/tenants/${id}`, data);
}

export function archiveTenant(id: string) {
  return patch<{ status: string }>(`/api/v1/tenants/${id}/archive`);
}

export function switchTenant(tenantId: string) {
  return post<{ effectiveTenantId: string }>("/api/v1/tenants/switch", { tenantId });
}

// --- Teachers ---
export interface Teacher {
  id: string;
  userId: string;
  email: string;
  displayName: string;
  employeeId: string | null;
  specialization: string | null;
  status: string;
  createdAt: string;
}

export interface TeacherListResponse {
  data: Teacher[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listTeachers(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<TeacherListResponse>(`/api/v1/teachers${qs ? `?${qs}` : ""}`);
}

export function createTeacher(data: { userId: string; employeeId?: string; specialization?: string }) {
  return post<{ id: string }>("/api/v1/teachers", data);
}

export function archiveTeacher(id: string) {
  return patch<{ status: string }>(`/api/v1/teachers/${id}/archive`);
}

// --- Students ---
export interface Student {
  id: string;
  userId: string;
  email: string;
  displayName: string;
  studentIdNumber: string | null;
  gradeLevel: string | null;
  status: string;
  createdAt: string;
}

export interface StudentListResponse {
  data: Student[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listStudents(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<StudentListResponse>(`/api/v1/students${qs ? `?${qs}` : ""}`);
}

export function createStudent(data: { userId: string; studentIdNumber?: string; gradeLevel?: string }) {
  return post<{ id: string }>("/api/v1/students", data);
}

export function archiveStudent(id: string) {
  return patch<{ status: string }>(`/api/v1/students/${id}/archive`);
}

// --- Staff ---
export interface Staff {
  id: string;
  userId: string;
  email: string;
  displayName: string;
  employeeId: string | null;
  department: string | null;
  position: string | null;
  status: string;
  createdAt: string;
}

export interface StaffListResponse {
  data: Staff[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listStaff(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<StaffListResponse>(`/api/v1/staff${qs ? `?${qs}` : ""}`);
}

export function createStaff(data: { userId: string; employeeId?: string; department?: string; position?: string }) {
  return post<{ id: string }>("/api/v1/staff", data);
}

export function archiveStaff(id: string) {
  return patch<{ status: string }>(`/api/v1/staff/${id}/archive`);
}

// --- Guardians ---
export interface Guardian {
  id: string;
  userId: string | null;
  name: string;
  phone: string | null;
  email: string | null;
  relationship: string | null;
  status: string;
  createdAt: string;
}

export interface GuardianListResponse {
  data: Guardian[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listGuardians(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<GuardianListResponse>(`/api/v1/guardians${qs ? `?${qs}` : ""}`);
}

export function createGuardian(data: { name: string; phone?: string; email?: string; relationship?: string }) {
  return post<{ id: string }>("/api/v1/guardians", data);
}

export function archiveGuardian(id: string) {
  return patch<{ status: string }>(`/api/v1/guardians/${id}/archive`);
}

export function linkStudentGuardian(guardianId: string, data: { studentId: string; isPrimary?: boolean }) {
  return post<{ id: string }>(`/api/v1/guardians/${guardianId}/link-student`, data);
}
