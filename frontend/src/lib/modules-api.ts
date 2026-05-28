import { get, post, patch, put, del } from "./api-client";

// --- Roles ---
export interface Role {
  id: string;
  slug: string;
  name: string;
}

export function listRoles() {
  return get<{ data: Role[] }>("/api/v1/roles");
}

// --- AI Provider Settings ---
export interface AIModelInfo { id: string }
export interface AIProviderSettings {
  scope: "user" | "tenant" | "environment";
  baseUrl: string;
  hasApiKey: boolean;
  defaultModel: string;
  availableModels: AIModelInfo[];
  chatbotModels?: AIModelInfo[];
  allowedRoles?: string[];
  enabled: boolean;
  updatedAt?: string;
}

export function getMyAISettings() {
  return get<AIProviderSettings>("/api/v1/ai/settings");
}

export function saveMyAISettings(data: { baseUrl: string; apiKey: string; defaultModel?: string; chatbotModels?: string[]; enabled?: boolean }) {
  const { baseUrl, apiKey, defaultModel, chatbotModels, enabled } = data;
  return put<AIProviderSettings>("/api/v1/ai/settings", { baseUrl, apiKey, defaultModel, chatbotModels, enabled });
}

export function patchMyAISettings(data: { enabled: boolean }) {
  return patch<AIProviderSettings>("/api/v1/ai/settings", data);
}

export function getTenantAISettings(tenantId?: string) {
  return get<AIProviderSettings>(tenantId ? `/api/v1/tenants/${tenantId}/ai-settings` : "/api/v1/ai/tenant-settings");
}

export function saveTenantAISettings(data: { baseUrl: string; apiKey: string; defaultModel?: string; chatbotModels?: string[]; allowedRoles?: string[]; enabled?: boolean }, tenantId?: string) {
  return put<AIProviderSettings>(tenantId ? `/api/v1/tenants/${tenantId}/ai-settings` : "/api/v1/ai/tenant-settings", data);
}

export function patchTenantAISettings(data: { enabled: boolean }, tenantId?: string) {
  return patch<AIProviderSettings>(tenantId ? `/api/v1/tenants/${tenantId}/ai-settings` : "/api/v1/ai/tenant-settings", data);
}

export function listAIModels() {
  return get<{ scope: string; defaultModel: string; models: AIModelInfo[] }>("/api/v1/ai/models");
}

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

export function listUsers(params?: { page?: number; search?: string; status?: string; role?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  if (params?.role) query.set("role", params.role);
  const qs = query.toString();
  return get<UserListResponse>(`/api/v1/users${qs ? `?${qs}` : ""}`);
}

export function createUser(data: { email: string; displayName: string; password: string; roleSlug?: string }) {
  return post<User>("/api/v1/users", data);
}

export function updateUser(id: string, data: { displayName?: string; email?: string; password?: string; status?: string }) {
  return patch<User>(`/api/v1/users/${id}`, data);
}

export function archiveUser(id: string) {
  return patch<{ status: string }>(`/api/v1/users/${id}/archive`);
}

export function restoreUser(id: string, email?: string) {
  return patch<{ id: string; status: string; email: string }>(
    `/api/v1/users/${id}/restore`,
    email ? { email } : undefined,
  );
}

// --- Tenants ---
export type SchoolType = "sd" | "smp" | "sma" | "smk" | "mixed";

export interface Tenant {
  id: string;
  name: string;
  code: string;
  status: string;
  logoUrl: string | null;
  schoolType: SchoolType;
  enabledPhases: string[];
  includeVocationalSubjects: boolean;
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

export function createTenant(data: { name: string; code: string; schoolType?: SchoolType; enabledPhases?: string[]; includeVocationalSubjects?: boolean }) {
  return post<Tenant>("/api/v1/tenants", data);
}

export function updateTenant(id: string, data: { name?: string; status?: string; schoolType?: SchoolType; enabledPhases?: string[]; includeVocationalSubjects?: boolean }) {
  return patch<Tenant>(`/api/v1/tenants/${id}`, data);
}

export function uploadTenantLogo(id: string, file: File) {
  const formData = new FormData();
  formData.append("logo", file);
  return post<{ id: string; logoUrl: string }>(`/api/v1/tenants/${id}/logo`, formData);
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

export function updateTeacher(id: string, data: { employeeId?: string; specialization?: string; status?: string }) {
  return patch<{ id: string }>(`/api/v1/teachers/${id}`, data);
}

export function archiveTeacher(id: string) {
  return patch<{ status: string }>(`/api/v1/teachers/${id}/archive`);
}

export function restoreTeacher(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/teachers/${id}/restore`);
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

export function listStudents(params?: { page?: number; search?: string; status?: string; classSectionId?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  if (params?.classSectionId) query.set("classSectionId", params.classSectionId);
  const qs = query.toString();
  return get<StudentListResponse>(`/api/v1/students${qs ? `?${qs}` : ""}`);
}

export function createStudent(data: { userId: string; studentIdNumber?: string; gradeLevel?: string }) {
  return post<{ id: string }>("/api/v1/students", data);
}

export function updateStudent(id: string, data: { studentIdNumber?: string; gradeLevel?: string; status?: string }) {
  return patch<{ id: string }>(`/api/v1/students/${id}`, data);
}

export function archiveStudent(id: string) {
  return patch<{ status: string }>(`/api/v1/students/${id}/archive`);
}

export function restoreStudent(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/students/${id}/restore`);
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

export function updateStaff(id: string, data: { employeeId?: string; department?: string; position?: string; status?: string }) {
  return patch<{ id: string }>(`/api/v1/staff/${id}`, data);
}

export function archiveStaff(id: string) {
  return patch<{ status: string }>(`/api/v1/staff/${id}/archive`);
}

export function restoreStaff(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/staff/${id}/restore`);
}

// --- Academic Years ---
export interface AcademicYear {
  id: string;
  code: string;
  name: string;
  startsOn: string | null;
  endsOn: string | null;
  status: string;
  createdAt: string;
}

export interface AcademicYearListResponse {
  data: AcademicYear[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listAcademicYears(params?: { page?: number; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<AcademicYearListResponse>(`/api/v1/academic-years${qs ? `?${qs}` : ""}`);
}

export function createAcademicYear(data: { code: string; name: string; startsOn?: string; endsOn?: string }) {
  return post<AcademicYear>("/api/v1/academic-years", data);
}

export function updateAcademicYear(id: string, data: { name?: string; startsOn?: string; endsOn?: string; status?: string }) {
  return patch<{ id: string }>(`/api/v1/academic-years/${id}`, data);
}

export function archiveAcademicYear(id: string) {
  return patch<{ status: string }>(`/api/v1/academic-years/${id}/archive`);
}

// --- Subjects ---
export interface Subject {
  id: string;
  code: string;
  name: string;
  description: string | null;
  status: string;
  createdAt: string;
}

export interface SubjectListResponse {
  data: Subject[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listSubjects(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<SubjectListResponse>(`/api/v1/subjects${qs ? `?${qs}` : ""}`);
}

export function createSubject(data: { code?: string; name: string; description?: string }) {
  return post<Subject>("/api/v1/subjects", data);
}

export function updateSubject(id: string, data: { name?: string; description?: string; status?: string }) {
  return patch<{ id: string }>(`/api/v1/subjects/${id}`, data);
}

export function archiveSubject(id: string) {
  return patch<{ status: string }>(`/api/v1/subjects/${id}/archive`);
}

export interface SubjectTeacher {
  id: string;
  displayName: string;
}

export function listSubjectTeachers(subjectId: string) {
  return get<{ data: SubjectTeacher[] }>(`/api/v1/subjects/${subjectId}/teachers`);
}

// --- Class Sections ---
export interface ClassSection {
  id: string;
  name: string;
  gradeLevel: string;
  homeroomTeacherId: string | null;
  capacity: number | null;
  status: string;
  createdAt: string;
  teacherName: string;
}

export interface ClassSectionListResponse {
  data: ClassSection[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listClassSections(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<ClassSectionListResponse>(`/api/v1/class-sections${qs ? `?${qs}` : ""}`);
}

export function createClassSection(data: { name: string; gradeLevel: string; academicYearId: string; homeroomTeacherId?: string; capacity?: number }) {
  return post<ClassSection>("/api/v1/class-sections", data);
}

export function updateClassSection(id: string, data: { name?: string; gradeLevel?: string; homeroomTeacherId?: string; capacity?: number; status?: string }) {
  return patch<{ id: string }>(`/api/v1/class-sections/${id}`, data);
}

export function archiveClassSection(id: string) {
  return patch<{ status: string }>(`/api/v1/class-sections/${id}/archive`);
}

// --- Teacher Subjects ---
export interface TeacherSubject {
  id: string;
  code: string;
  name: string;
}

export function listTeacherSubjects(teacherId: string) {
  return get<{ data: TeacherSubject[] }>(`/api/v1/teachers/${teacherId}/subjects`);
}

export function assignTeacherSubject(teacherId: string, subjectId: string) {
  return post<{ teacherId: string; subjectId: string }>(`/api/v1/teachers/${teacherId}/subjects`, { subjectId });
}

export function unassignTeacherSubject(teacherId: string, subjectId: string) {
  return del<{ status: string }>(`/api/v1/teachers/${teacherId}/subjects/${subjectId}`);
}
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

export function updateGuardian(id: string, data: { name?: string; phone?: string; email?: string; relationship?: string; status?: string }) {
  return patch<{ id: string }>(`/api/v1/guardians/${id}`, data);
}

export function archiveGuardian(id: string) {
  return patch<{ status: string }>(`/api/v1/guardians/${id}/archive`);
}

export function restoreGuardian(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/guardians/${id}/restore`);
}

export function linkStudentGuardian(guardianId: string, data: { studentId: string; isPrimary?: boolean }) {
  return post<{ id: string }>(`/api/v1/guardians/${guardianId}/link-student`, data);
}

// --- Composite Create ---
export function createTeacherFull(data: { displayName: string; email: string; password: string; employeeId?: string; specialization?: string; subjectIds?: string[] }) {
  return post<{ id: string; userId: string }>("/api/v1/teachers/create-full", data);
}

export function createStudentFull(data: { displayName: string; email: string; password: string; studentIdNumber?: string; gradeLevel?: string; classSectionId?: string }) {
  return post<{ id: string; userId: string }>("/api/v1/students/create-full", data);
}

export function createStaffFull(data: { displayName: string; email: string; password: string; employeeId?: string; department?: string; position?: string }) {
  return post<{ id: string; userId: string }>("/api/v1/staff/create-full", data);
}

// --- Programs ---
export interface Program {
  id: string;
  title: string;
  description: string | null;
  kind: string;
  status: string;
  gradeLevel: string | null;
  publishedAt: string | null;
  createdAt: string;
}

export interface ProgramListResponse {
  data: Program[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listPrograms(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<ProgramListResponse>(`/api/v1/programs${qs ? `?${qs}` : ""}`);
}

export function createProgram(data: { title: string; description?: string; kind?: string; gradeLevel?: string; subjectId?: string }) {
  return post<Program>("/api/v1/programs", data);
}

export function updateProgram(id: string, data: { title?: string; description?: string; kind?: string; gradeLevel?: string }) {
  return patch<{ id: string }>(`/api/v1/programs/${id}`, data);
}

export function archiveProgram(id: string) {
  return patch<{ status: string }>(`/api/v1/programs/${id}/archive`);
}

export function publishProgram(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/programs/${id}/publish`);
}

// --- Program Sections ---
export interface ProgramSection {
  id: string;
  title: string;
  sortOrder: number;
  unlockMode: string;
  isRequired: boolean;
  items: ProgramItem[];
}

export interface ProgramItem {
  id: string;
  itemType: string;
  itemId: string;
  sortOrder: number;
  isRequired: boolean;
  passingGrade: number | null;
  maxAttempts: number;
}

export function listProgramSections(programId: string) {
  return get<{ data: ProgramSection[] }>(`/api/v1/programs/${programId}/sections`);
}

export function createProgramSection(programId: string, data: { title: string; unlockMode?: string; isRequired?: boolean }) {
  return post<{ id: string }>(`/api/v1/programs/${programId}/sections`, data);
}

export function updateProgramSection(sectionId: string, data: { title?: string; sortOrder?: number; unlockMode?: string; isRequired?: boolean }) {
  return patch<{ id: string }>(`/api/v1/program-sections/${sectionId}`, data);
}

export function deleteProgramSection(sectionId: string) {
  return del<{ status: string }>(`/api/v1/program-sections/${sectionId}`);
}

export function createProgramItem(sectionId: string, data: { itemType: string; itemId: string; isRequired?: boolean; passingGrade?: number; maxAttempts?: number }) {
  return post<{ id: string }>(`/api/v1/program-sections/${sectionId}/items`, data);
}

export function updateProgramItem(itemId: string, data: { sortOrder?: number; isRequired?: boolean; passingGrade?: number; maxAttempts?: number }) {
  return patch<{ id: string }>(`/api/v1/program-items/${itemId}`, data);
}

export function deleteProgramItem(itemId: string) {
  return del<{ status: string }>(`/api/v1/program-items/${itemId}`);
}

// --- Courses ---
export interface Course {
  id: string;
  title: string;
  description: string | null;
  status: string;
  publishedAt: string | null;
  createdAt: string;
}

export interface CourseListResponse {
  data: Course[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listCourses(params?: { page?: number; search?: string; status?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  const qs = query.toString();
  return get<CourseListResponse>(`/api/v1/courses${qs ? `?${qs}` : ""}`);
}

export function createCourse(data: { title: string; description?: string; subjectId?: string }) {
  return post<Course>("/api/v1/courses", data);
}

export function updateCourse(id: string, data: { title?: string; description?: string }) {
  return patch<{ id: string }>(`/api/v1/courses/${id}`, data);
}

export function archiveCourse(id: string) {
  return patch<{ status: string }>(`/api/v1/courses/${id}/archive`);
}

export function publishCourse(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/courses/${id}/publish`);
}

// --- Exams ---
export interface Exam {
  id: string;
  title: string;
  description: string | null;
  subjectId: string | null;
  subjectName?: string | null;
  gradeLevel?: string | null;
  examType: string;
  durationMinutes: number | null;
  maxScore: number;
  passingScore: number;
  status: string;
  usesKisiKisi: boolean;
  shuffleQuestions: boolean;
  shuffleOptions: boolean;
  showResultImmediately: boolean;
  publishedAt: string | null;
  createdAt: string;
  questionCount: number;
  totalPoints: number;
  canAccess?: boolean;
  canWrite?: boolean;
  canDelete?: boolean;
}

export interface ExamListResponse {
  data: Exam[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listExams(params?: { page?: number; search?: string; status?: string; subjectId?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.status) query.set("status", params.status);
  if (params?.subjectId) query.set("subjectId", params.subjectId);
  const qs = query.toString();
  return get<ExamListResponse>(`/api/v1/exams${qs ? `?${qs}` : ""}`);
}

export function getExam(id: string) {
  return get<Exam>(`/api/v1/exams/${id}`);
}

export interface CreateExamPayload {
  title: string;
  description?: string;
  subjectId?: string;
  gradeLevel?: string;
  examType?: string;
  durationMinutes?: number;
  maxScore?: number;
  passingScore?: number;
  shuffleQuestions?: boolean;
  shuffleOptions?: boolean;
  showResultImmediately?: boolean;
  usesKisiKisi?: boolean;
}

export function createExam(data: CreateExamPayload) {
  return post<{ id: string; status: string; usesKisiKisi?: boolean }>("/api/v1/exams", data);
}

export function updateExam(id: string, data: Partial<CreateExamPayload>) {
  return patch<{ id: string; status: string; warning?: string; hint?: string }>(
    `/api/v1/exams/${id}`,
    data,
  );
}

/**
 * Convenience helper for the kisi-kisi toggle in the exam header
 * (ADR-0012). The earlier 3-mode picker was never shipped — this is
 * the canonical kisi-kisi toggle.
 */
export function updateExamKisiKisi(id: string, enabled: boolean) {
  return updateExam(id, { usesKisiKisi: enabled });
}

export function publishExam(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/exams/${id}/publish`);
}

export function archiveExam(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/exams/${id}/archive`);
}

export function restoreExam(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/exams/${id}/restore`);
}

export function hardDeleteExam(id: string) {
  return del<{ id: string; status: string }>(`/api/v1/exams/${id}`);
}

// --- Exam Sections ---
export interface ExamSection {
  id: string;
  examId: string;
  title: string;
  description: string | null;
  sortOrder: number;
  questionCount: number;
  createdAt: string;
}

export function listExamSections(examId: string) {
  return get<{ data: ExamSection[] }>(`/api/v1/exams/${examId}/sections`);
}

export function createExamSection(examId: string, data: { title: string; description?: string; sortOrder?: number }) {
  return post<{ id: string }>(`/api/v1/exams/${examId}/sections`, data);
}

export function updateExamSection(sectionId: string, data: { title?: string; description?: string; sortOrder?: number }) {
  return patch<{ id: string }>(`/api/v1/exam-sections/${sectionId}`, data);
}

export function deleteExamSection(sectionId: string) {
  return del<{ status: string }>(`/api/v1/exam-sections/${sectionId}`);
}

// --- Questions ---
export type QuestionType = "multiple_choice" | "true_false" | "short_answer" | "essay";
export type ScoringMode = "correct_all" | "correct_one" | "percentage";

export interface QuestionOption {
  id?: string;
  content: string;
  isCorrect: boolean;
  sortOrder?: number;
  pointsWeight?: number | null;
}

export interface QuestionSlotRef {
  id: string;
  position: number;
  cpElementId?: string | null;
  capaianPembelajaran?: string | null;
  elemenCp?: string | null;
  tujuanPembelajaran?: string | null;
  materiPokok?: string | null;
  kelas?: string | null;
  semester?: string | null;
  indikatorSoal?: string | null;
  cognitiveLevel?: string | null;
  difficulty?: string | null;
  questionType?: string | null;
  points: number;
  /** Phase 9.8 — true when the slot's parent blueprint was cloned
   *  from a template. The accordion locks pedagogical fields when
   *  this is set; auto-blueprint slots stay editable. */
  fromTemplate?: boolean;
}

export interface QuestionStimulusRef {
  id: string;
  title: string;
}

export interface QuestionGroupRef {
  id: string;
  stimulusTitle?: string | null;
}

export interface Question {
  id: string;
  examId: string;
  sectionId: string | null;
  groupId?: string | null;
  questionType: QuestionType;
  content: string;
  explanation: string | null;
  correctAnswer?: string | null;
  rubric?: any;
  points: number;
  sortOrder: number;
  scoringMode: ScoringMode;
  wrongPenaltyPct?: number | null;
  shuffleOptionsOverride?: boolean | null;
  correctCount: number;
  blueprintSlotId?: string | null;
  stimulusId?: string | null;
  slot?: QuestionSlotRef | null;
  stimulus?: QuestionStimulusRef | null;
  group?: QuestionGroupRef | null;
  options?: QuestionOption[];
  createdAt: string;
}

export function listQuestions(examId: string) {
  return get<{ data: Question[] }>(`/api/v1/exams/${examId}/questions`);
}

export interface CreateQuestionPayload {
  sectionId?: string;
  groupId?: string;
  stimulusId?: string;
  questionType: QuestionType;
  content: string;
  explanation?: string;
  correctAnswer?: string;
  rubric?: any;
  points?: number;
  sortOrder?: number;
  scoringMode?: ScoringMode;
  wrongPenaltyPct?: number;
  shuffleOptionsOverride?: boolean;
  blueprintSlotId?: string;
  forceLink?: boolean;
  options?: QuestionOption[];
  // Phase 9.8 — inline kisi-kisi metadata. When the exam tracks
  // kisi-kisi the backend either auto-creates a slot or writes the
  // values through to the bound slot in the same transaction.
  cognitiveLevel?: string;
  difficulty?: string;
  cpElementId?: string;
  capaianPembelajaran?: string;
  elemenCp?: string;
  tujuanPembelajaran?: string;
  materiPokok?: string;
  kelas?: string;
  semester?: string;
  indikatorSoal?: string;
}

export function createQuestion(examId: string, data: CreateQuestionPayload) {
  return post<{ id: string }>(`/api/v1/exams/${examId}/questions`, data);
}

/**
 * Slot-first authoring entry point (ADR-0012).
 * The slot's metadata provides the defaults for questionType + points;
 * the link is established server-side in the same transaction as
 * insert.
 */
export interface CreateQuestionFromSlotPayload
  extends Omit<CreateQuestionPayload, "questionType" | "blueprintSlotId"> {
  questionType?: QuestionType;
}

export function createQuestionFromSlot(
  examId: string,
  slotId: string,
  data: CreateQuestionFromSlotPayload,
) {
  return post<{ id: string; slotId: string }>(
    `/api/v1/exams/${examId}/questions/from-slot`,
    { slotId, ...data },
  );
}

// ─── Drag-and-drop move endpoint (ADR-0012) ───

export interface MoveQuestionPayload {
  questionId: string;
  /** Pass empty string to clear section assignment, omit to leave unchanged. */
  sectionId?: string;
  /** Pass empty string to clear group assignment, omit to leave unchanged. */
  groupId?: string;
  sortOrder?: number;
}

export function moveQuestion(examId: string, body: MoveQuestionPayload) {
  return post<{ id: string; status: string }>(
    `/api/v1/exams/${examId}/questions/move`,
    body,
  );
}

// ─── Question groups (stimulus clusters) ───

export interface CreateQuestionGroupPayload {
  /** Section the group lives inside. Optional — backend defaults to
   *  the exam's first section when omitted (Phase 9.8 mandate). */
  sectionId?: string;
  stimulusId?: string;
  name?: string;
  sortOrder?: number;
}

export function createQuestionGroup(
  examId: string,
  data: CreateQuestionGroupPayload,
) {
  return post<{
    id: string;
    sectionId?: string;
    groupType: string;
    displayOrder: number;
  }>(`/api/v1/exams/${examId}/groups`, data);
}

export interface UpdateQuestionGroupPayload {
  /** Move the group between sections. Empty string clears (group
   *  becomes section-less); omit to leave untouched. */
  sectionId?: string;
  stimulusId?: string;
  resyncSnapshot?: boolean;
  sortOrder?: number;
  /** Inline edit of the group's stimulus snapshot. Updates only this
   *  group's local copy; does not touch the master stimuli row, so
   *  other groups referencing the same stimulus are unaffected. */
  titleSnapshot?: string;
  bodySnapshot?: string;
  /** Opt-in: after saving the snapshot, create (or update) a shared
   *  stimulus row in the library and link this group to it. Lets the
   *  user promote a passage to the cross-exam library without leaving
   *  the group editor. */
  saveToLibrary?: boolean;
}

export function updateQuestionGroup(
  groupId: string,
  data: UpdateQuestionGroupPayload,
) {
  return patch<{ id: string; status: string }>(
    `/api/v1/groups/${groupId}`,
    data,
  );
}

export function deleteQuestionGroup(groupId: string) {
  return del<{ id: string; status: string }>(`/api/v1/groups/${groupId}`);
}

// ─── List groups in an exam (ADR-0012 UX rewrite) ───

export interface ExamQuestionGroup {
  id: string;
  sectionId?: string | null;
  stimulusId?: string | null;
  stimulusTitleSnapshot?: string | null;
  stimulusBodySnapshot?: string | null;
  groupType: string;
  displayOrder: number;
  questionCount: number;
  createdAt: string;
}

export function listExamGroups(examId: string) {
  return get<{ data: ExamQuestionGroup[] }>(
    `/api/v1/exams/${examId}/groups`,
  );
}

export function updateQuestion(
  questionId: string,
  data: Partial<CreateQuestionPayload> & { forceLink?: boolean },
) {
  return patch<{ id: string }>(`/api/v1/questions/${questionId}`, data);
}

export function deleteQuestion(questionId: string) {
  return del<{ status: string }>(`/api/v1/questions/${questionId}`);
}

export function createOption(questionId: string, data: { content: string; isCorrect: boolean; sortOrder?: number; pointsWeight?: number }) {
  return post<{ id: string }>(`/api/v1/questions/${questionId}/options`, data);
}

export function updateOption(optionId: string, data: { content?: string; isCorrect?: boolean; sortOrder?: number; pointsWeight?: number }) {
  return patch<{ id: string }>(`/api/v1/options/${optionId}`, data);
}

export function deleteOption(optionId: string) {
  return del<{ status: string }>(`/api/v1/options/${optionId}`);
}

// --- Exam Gate Windows ---
export interface ExamGate {
  id: string;
  examId: string;
  opensAt: string;
  closesAt: string;
  accessCode: string | null;
  isOpen: boolean;
  createdAt: string;
}

export function listExamGates(examId: string) {
  return get<{ data: ExamGate[] }>(`/api/v1/exams/${examId}/gates`);
}

export function createExamGate(examId: string, data: { opensAt: string; closesAt: string; accessCode?: string }) {
  return post<{ id: string }>(`/api/v1/exams/${examId}/gates`, data);
}

export function updateExamGate(gateId: string, data: { opensAt?: string; closesAt?: string; accessCode?: string }) {
  return patch<{ id: string }>(`/api/v1/exam-gates/${gateId}`, data);
}

export function deleteExamGate(gateId: string) {
  return del<{ status: string }>(`/api/v1/exam-gates/${gateId}`);
}

// ==========================================================================
// Phase 9.5 — Collaborators (works for exam/course/blueprint_template)
// ==========================================================================

export type CollaboratorRole = "editor" | "viewer";

export interface Collaborator {
  id: string;
  userId: string;
  displayName: string;
  email: string;
  role: CollaboratorRole | "owner";
  createdAt: string;
}

export type CollabResource = "exams" | "courses" | "blueprint-templates";

export function listCollaborators(resource: CollabResource, resourceId: string) {
  return get<{ owner: Collaborator | null; collaborators: Collaborator[] }>(
    `/api/v1/${resource}/${resourceId}/collaborators`,
  );
}

export function inviteCollaborator(
  resource: CollabResource,
  resourceId: string,
  data: { userId: string; role: CollaboratorRole },
) {
  return post<{ id: string; role: string }>(
    `/api/v1/${resource}/${resourceId}/collaborators`,
    data,
  );
}

export function updateCollaboratorRole(
  resource: CollabResource,
  resourceId: string,
  collaboratorId: string,
  role: CollaboratorRole,
) {
  // Backend mounts PATCH/DELETE under the singular collab resource:
  //   /api/v1/exam-collaborators/{collabId}
  //   /api/v1/course-collaborators/{collabId}
  //   /api/v1/blueprint-template-collaborators/{collabId}
  // resourceId stays in the closure for audit context but is not in
  // the URL.
  void resourceId;
  return patch<{ id: string; role: string }>(
    `/api/v1/${collabResourcePath(resource)}/${collaboratorId}`,
    { role },
  );
}

export function removeCollaborator(
  resource: CollabResource,
  resourceId: string,
  collaboratorId: string,
) {
  void resourceId;
  return del<{ status: string }>(
    `/api/v1/${collabResourcePath(resource)}/${collaboratorId}`,
  );
}

// Maps the plural parent resource to the singular collab table path
// segment used by the backend mux. Keep in sync with
// registerCollaboratorRoutes() in collaborator_handlers.go.
function collabResourcePath(resource: CollabResource): string {
  switch (resource) {
    case "exams":
      return "exam-collaborators";
    case "courses":
      return "course-collaborators";
    case "blueprint-templates":
      return "blueprint-template-collaborators";
  }
}

export function transferOwnership(
  resource: CollabResource,
  resourceId: string,
  newOwnerUserId: string,
) {
  return patch<{ ownerUserId: string }>(
    `/api/v1/${resource}/${resourceId}/transfer-ownership`,
    { newOwnerUserId },
  );
}

// ==========================================================================
// Phase 9.5 — Stimuli library
// ==========================================================================

export type StimulusLifecycle = "exam_scoped" | "shared" | "archived";

export interface Stimulus {
  id: string;
  ownerUserId: string;
  ownerName: string;
  type: string;
  title: string;
  content: string;
  source: string | null;
  lifecycle: StimulusLifecycle;
  parentExamId: string;
  usageCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface StimulusListResponse {
  data: Stimulus[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listStimuli(params?: {
  page?: number;
  search?: string;
  lifecycle?: StimulusLifecycle | "all";
  parentExamId?: string;
}) {
  const q = new URLSearchParams();
  if (params?.page) q.set("page", String(params.page));
  if (params?.search) q.set("search", params.search);
  if (params?.lifecycle) q.set("lifecycle", params.lifecycle);
  if (params?.parentExamId) q.set("parentExamId", params.parentExamId);
  const qs = q.toString();
  return get<StimulusListResponse>(`/api/v1/stimuli${qs ? `?${qs}` : ""}`);
}

export function getStimulus(id: string) {
  return get<Stimulus>(`/api/v1/stimuli/${id}`);
}

export interface CreateStimulusPayload {
  title: string;
  content: string;
  type?: string;
  source?: string;
  lifecycle?: "exam_scoped" | "shared";
  parentExamId?: string;
}

export function createStimulus(data: CreateStimulusPayload) {
  return post<{ id: string; lifecycle: StimulusLifecycle }>("/api/v1/stimuli", data);
}

export function updateStimulus(
  id: string,
  data: Partial<{ title: string; content: string; source: string; type: string }>,
) {
  return patch<{ id: string }>(`/api/v1/stimuli/${id}`, data);
}

export function archiveStimulus(id: string) {
  return patch<{ status: string }>(`/api/v1/stimuli/${id}/archive`);
}

export function promoteStimulus(id: string) {
  return patch<{ id: string; status: string }>(`/api/v1/stimuli/${id}/promote`);
}

// ==========================================================================
// Phase 9.5 — Blueprint templates (library)
// ==========================================================================

export type BlueprintType = "reguler";
export type BlueprintStatus = "draft" | "published" | "archived";
export type CurriculumCode = "merdeka";

export interface BlueprintTemplate {
  id: string;
  ownerUserId: string;
  ownerName: string;
  title: string;
  description: string | null;
  curriculumId: string;
  curriculumCode: string;
  competencyLabel: string;
  subjectCode: string | null;
  gradeOrPhase: string | null;
  blueprintType: BlueprintType;
  totalSlots: number;
  totalPoints: number;
  strictCoverage: boolean;
  status: BlueprintStatus;
  version: number;
  createdAt: string;
  updatedAt: string;
  canAccess: boolean;
}

export interface BlueprintTemplateListResponse {
  data: BlueprintTemplate[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listBlueprintTemplates(params?: {
  page?: number;
  search?: string;
  curriculum?: CurriculumCode;
  type?: BlueprintType;
  status?: BlueprintStatus;
}) {
  const q = new URLSearchParams();
  if (params?.page) q.set("page", String(params.page));
  if (params?.search) q.set("search", params.search);
  if (params?.curriculum) q.set("curriculum", params.curriculum);
  if (params?.type) q.set("type", params.type);
  if (params?.status) q.set("status", params.status);
  const qs = q.toString();
  return get<BlueprintTemplateListResponse>(
    `/api/v1/blueprint-templates${qs ? `?${qs}` : ""}`,
  );
}

export function getBlueprintTemplate(id: string) {
  return get<BlueprintTemplate>(`/api/v1/blueprint-templates/${id}`);
}

export interface CreateBlueprintTemplatePayload {
  title: string;
  description?: string;
  curriculumCode: CurriculumCode;
  subjectCode?: string;
  gradeOrPhase?: string;
  blueprintType?: BlueprintType;
  strictCoverage?: boolean;
}

export function createBlueprintTemplate(data: CreateBlueprintTemplatePayload) {
  return post<{ id: string; status: string }>("/api/v1/blueprint-templates", data);
}

export function updateBlueprintTemplate(
  id: string,
  data: Partial<{
    title: string;
    description: string;
    subjectCode: string;
    gradeOrPhase: string;
    strictCoverage: boolean;
  }>,
) {
  return patch<{ id: string }>(`/api/v1/blueprint-templates/${id}`, data);
}

export function publishBlueprintTemplate(id: string) {
  return patch<{ id: string; status: string }>(
    `/api/v1/blueprint-templates/${id}/publish`,
  );
}

export function unpublishBlueprintTemplate(id: string) {
  return patch<{ id: string; status: string }>(
    `/api/v1/blueprint-templates/${id}/unpublish`,
  );
}

export function archiveBlueprintTemplate(id: string) {
  return patch<{ id: string; status: string }>(
    `/api/v1/blueprint-templates/${id}/archive`,
  );
}

export function restoreBlueprintTemplate(id: string) {
  return patch<{ id: string; status: string }>(
    `/api/v1/blueprint-templates/${id}/restore`,
  );
}

export function hardDeleteBlueprintTemplate(id: string) {
  return del<{ id: string; status: string }>(`/api/v1/blueprint-templates/${id}`);
}

// ─── Slots: dual-table (template + exam blueprint) ───
export interface BlueprintSlot {
  id: string;
  position: number;
  competencyId: string | null;
  cognitiveLevel: string | null;
  difficulty: string | null;
  questionType: string | null;
  points: number;
  stimulusId: string | null;
  cpElementId: string | null;
  capaianPembelajaran: string | null;
  elemenCp: string | null;
  tujuanPembelajaran: string | null;
  materiPokok: string | null;
  kelas: string | null;
  semester: string | null;
  indikatorSoal: string | null;
  questionId?: string | null;
  filled?: boolean;
  createdAt: string;
}

export interface SlotPayload {
  position?: number;
  cognitiveLevel?: string;
  difficulty?: string;
  questionType?: string;
  points?: number;
  stimulusId?: string;
  cpElementId?: string;
  capaianPembelajaran?: string;
  elemenCp?: string;
  tujuanPembelajaran?: string;
  materiPokok?: string;
  kelas?: string;
  semester?: string;
  indikatorSoal?: string;
}

export function listTemplateSlots(templateId: string) {
  return get<{ data: BlueprintSlot[] }>(
    `/api/v1/blueprint-templates/${templateId}/slots`,
  );
}

export function createTemplateSlot(templateId: string, data: SlotPayload) {
  return post<{ id: string; position: number }>(
    `/api/v1/blueprint-templates/${templateId}/slots`,
    data,
  );
}

export function bulkAddTemplateSlots(templateId: string, slots: SlotPayload[]) {
  return post<{ ids: string[]; count: number }>(
    `/api/v1/blueprint-templates/${templateId}/slots/bulk`,
    { slots },
  );
}

export function updateTemplateSlot(slotId: string, data: SlotPayload) {
  return patch<{ id: string }>(`/api/v1/blueprint-template-slots/${slotId}`, data);
}

export function deleteTemplateSlot(slotId: string) {
  return del<{ status: string }>(`/api/v1/blueprint-template-slots/${slotId}`);
}

// ─── Exam blueprints (cloned snapshot) ───
export interface ExamBlueprint {
  id: string;
  examId: string;
  sourceTemplateId: string | null;
  sourceTemplateVersion: number | null;
  createdVia: string;
  title: string;
  description: string | null;
  curriculumCode: string;
  competencyLabel: string;
  blueprintType: BlueprintType;
  totalSlots: number;
  totalPoints: number;
  strictCoverage: boolean;
  status: string;
  filledSlots: number;
  coverage: number;
  createdAt: string;
}

export function getExamBlueprint(examId: string) {
  return get<{ blueprint: ExamBlueprint | null }>(
    `/api/v1/exams/${examId}/blueprint`,
  );
}

export function listExamBlueprintSlots(examId: string) {
  return get<{ data: BlueprintSlot[] }>(
    `/api/v1/exams/${examId}/blueprint/slots`,
  );
}

export function cloneBlueprintToExam(
  examId: string,
  data: { templateId: string; replace?: boolean },
) {
  return post<{
    id: string;
    sourceTemplateId: string;
    sourceTemplateVersion: number;
    createdVia: string;
  }>(`/api/v1/exams/${examId}/blueprint/clone`, data);
}

export function exportExamBlueprintToTemplate(
  examId: string,
  data: {
    title: string;
    description?: string;
    subjectCode?: string;
    gradeOrPhase?: string;
  },
) {
  return post<{ id: string; status: string }>(
    `/api/v1/exams/${examId}/blueprint/export`,
    data,
  );
}

export function createExamBlueprintSlot(examId: string, data: SlotPayload) {
  return post<{ id: string; position: number }>(
    `/api/v1/exams/${examId}/blueprint/slots`,
    data,
  );
}

export function updateExamBlueprintSlot(slotId: string, data: SlotPayload) {
  return patch<{ id: string }>(`/api/v1/exam-blueprint-slots/${slotId}`, data);
}

export function deleteExamBlueprintSlot(slotId: string) {
  return del<{ status: string }>(`/api/v1/exam-blueprint-slots/${slotId}`);
}

export function assignQuestionToSlot(slotId: string, questionId: string | null) {
  return patch<{ slotId: string; questionId: string | null; status: string }>(
    `/api/v1/exam-blueprint-slots/${slotId}/assign-question`,
    { questionId },
  );
}

// ─── Slot-first canvas (ADR-0012) ───

export interface SlotWithQuestion extends BlueprintSlot {
  question: {
    id: string;
    content: string;
    questionType: string;
    points: number;
    sortOrder: number;
    stimulus?: { id: string; title: string } | null;
    group?: { id: string; stimulusTitle?: string | null } | null;
  } | null;
}

export interface SlotsWithQuestionsResponse {
  blueprintType?: string;
  /** Phase 9.8 — populated when the blueprint was cloned from a
   *  template. Frontend uses this to gate edit-vs-readonly on slot
   *  metadata fields rendered inline in the question accordion. */
  sourceTemplateId?: string | null;
  slots: SlotWithQuestion[];
  unlinked: Array<{
    id: string;
    content: string;
    questionType: string;
    points: number;
    sortOrder: number;
    stimulus?: { id: string; title: string } | null;
    group?: { id: string; stimulusTitle?: string | null } | null;
  }>;
  coverage: { filled: number; total: number };
}

export function getSlotsWithQuestions(examId: string) {
  return get<SlotsWithQuestionsResponse>(
    `/api/v1/exams/${examId}/slots-with-questions`,
  );
}

// --- Curriculum CP master data ---
export interface CurriculumCPElement {
  id: string;
  name: string;
  content: string;
  sortOrder: number;
}

export interface CurriculumCPReference {
  id: string;
  curriculumCode: string;
  levelCode: string;
  levelName: string | null;
  subjectCode: string;
  subjectName: string;
  phase: string;
  generalCp: string;
  sourceName: string;
  sourceUrl: string | null;
  status: string;
  elementsCount: number;
  elements?: CurriculumCPElement[];
  createdAt: string;
  updatedAt: string;
}

export interface CurriculumCPListResponse {
  data: CurriculumCPReference[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export function listCurriculumCPReferences(params?: { page?: number; search?: string; level?: string; phase?: string }) {
  const query = new URLSearchParams();
  if (params?.page) query.set("page", String(params.page));
  if (params?.search) query.set("search", params.search);
  if (params?.level) query.set("level", params.level);
  if (params?.phase) query.set("phase", params.phase);
  const qs = query.toString();
  return get<CurriculumCPListResponse>(`/api/v1/curriculum/cp-references${qs ? `?${qs}` : ""}`);
}

export function getCurriculumCPReference(id: string) {
  return get<CurriculumCPReference>(`/api/v1/curriculum/cp-references/${id}`);
}

export function seedCurriculumCPReference(data: { levelCode: string; subjectCode: string; phase: string }) {
  return post<CurriculumCPReference>("/api/v1/curriculum/cp-references/seed", data);
}

export function createCurriculumCPReference(data: {
  levelCode: string;
  levelName?: string;
  subjectCode: string;
  subjectName: string;
  phase: string;
  generalCp: string;
  sourceUrl?: string;
  elements?: Array<{ name: string; content: string }>;
}) {
  return post<CurriculumCPReference>("/api/v1/curriculum/cp-references", data);
}

export function createCurriculumCPElement(referenceId: string, data: { name: string; content: string; sortOrder?: number }) {
  return post<{ id: string }>(`/api/v1/curriculum/cp-references/${referenceId}/elements`, data);
}

export function deleteCurriculumCPElement(id: string) {
  return del<{ status: string }>(`/api/v1/curriculum/cp-elements/${id}`);
}

export function updateCurriculumCPReference(id: string, data: { subjectName?: string; generalCp?: string; status?: string }) {
  return patch<CurriculumCPReference>(`/api/v1/curriculum/cp-references/${id}`, data);
}

export function updateCurriculumCPElement(id: string, data: { name?: string; content?: string; sortOrder?: number }) {
  return patch<{ id: string; status: string }>(`/api/v1/curriculum/cp-elements/${id}`, data);
}
