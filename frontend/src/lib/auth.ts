import { get, post, type ApiResponse } from "./api-client";

export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  isPlatformAdmin: boolean;
}

export interface AuthSession {
  user: AuthUser;
  effectiveTenantId: string | null;
  roles: string[];
  permissions: string[];
}

export interface LoginResponse {
  user: AuthUser;
  csrfToken: string;
}

export async function login(
  email: string,
  password: string
): Promise<ApiResponse<LoginResponse>> {
  return post<LoginResponse>("/api/v1/auth/login", { email, password });
}

export async function logout(): Promise<ApiResponse<{ status: string }>> {
  return post<{ status: string }>("/api/v1/auth/logout");
}

export async function getMe(): Promise<ApiResponse<AuthSession>> {
  return get<AuthSession>("/api/v1/auth/me");
}
