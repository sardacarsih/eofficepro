// Klien API eOffice Pro — menyimpan token di localStorage (cukup untuk fase dev;
// evaluasi httpOnly cookie saat hardening E10).

const BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

const ACCESS_KEY = "eoffice_access";
const REFRESH_KEY = "eoffice_refresh";

export interface User {
  id: string;
  nik?: string;
  email: string;
  full_name: string;
  roles: string[];
  positions?: {
    position_id: string;
    title: string;
    position_type: string;
    org_unit: string;
    assignment_type: string;
  }[];
}

export interface OrgUnit {
  id: string;
  parent_id: string | null;
  code: string;
  name: string;
  unit_level: string;
  region: string | null;
  is_active: boolean;
  children?: OrgUnit[];
}

export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACCESS_KEY);
}

function setTokens(access: string, refresh: string) {
  localStorage.setItem(ACCESS_KEY, access);
  localStorage.setItem(REFRESH_KEY, refresh);
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

export async function login(identifier: string, password: string): Promise<User> {
  const res = await fetch(`${BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ identifier, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Login gagal");
  setTokens(data.access_token, data.refresh_token);
  return data.user as User;
}

export async function logout() {
  const refresh = localStorage.getItem(REFRESH_KEY);
  if (refresh) {
    await fetch(`${BASE}/auth/logout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refresh }),
    }).catch(() => {});
  }
  clearTokens();
}

async function tryRefresh(): Promise<boolean> {
  const refresh = localStorage.getItem(REFRESH_KEY);
  if (!refresh) return false;
  const res = await fetch(`${BASE}/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!res.ok) {
    clearTokens();
    return false;
  }
  const data = await res.json();
  setTokens(data.access_token, data.refresh_token);
  return true;
}

// apiFetch: sisipkan bearer token; sekali 401 → coba refresh lalu ulangi.
export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
  retried = false,
): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getAccessToken() ?? ""}`,
      ...init.headers,
    },
  });

  if (res.status === 401 && !retried && (await tryRefresh())) {
    return apiFetch<T>(path, init, true);
  }
  if (res.status === 401) {
    clearTokens();
    if (typeof window !== "undefined") window.location.href = "/login";
    throw new Error("Sesi berakhir, silakan login ulang");
  }

  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? `Permintaan gagal (${res.status})`);
  return data as T;
}

export const getMe = () => apiFetch<User>("/auth/me");
export const getOrgTree = () =>
  apiFetch<{ tree: OrgUnit[]; total: number }>("/org-units");

// ---- Master jenis surat ----

export interface LetterType {
  id: string;
  code: string;
  name: string;
  default_classification: "biasa" | "terbatas" | "rahasia";
  default_sla_hours: number;
  is_active: boolean;
}

export interface LetterTypePayload {
  code: string;
  name: string;
  default_classification: LetterType["default_classification"];
  default_sla_hours: number;
  is_active?: boolean;
}

export const listLetterTypes = (includeInactive = false) =>
  apiFetch<{ letter_types: LetterType[] }>(
    `/letter-types${includeInactive ? "?include_inactive=true" : ""}`,
  );

export const createLetterType = (payload: LetterTypePayload) =>
  apiFetch<{ id: string }>("/letter-types", {
    method: "POST",
    body: JSON.stringify(payload),
  });

export const updateLetterType = (id: string, payload: LetterTypePayload) =>
  apiFetch<{ id: string }>(`/letter-types/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });

export const deactivateLetterType = (id: string) =>
  apiFetch<{ id: string }>(`/letter-types/${id}`, { method: "DELETE" });

// ---- Reset password (publik) ----

export async function forgotPassword(email: string): Promise<string> {
  const res = await fetch(`${BASE}/auth/forgot-password`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Permintaan gagal");
  return data.message as string;
}

export async function resetPassword(
  token: string,
  newPassword: string,
): Promise<string> {
  const res = await fetch(`${BASE}/auth/reset-password`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token, new_password: newPassword }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Reset password gagal");
  return data.message as string;
}

// ---- Manajemen pengguna (admin) ----

export interface UserRow {
  id: string;
  nik: string;
  email: string;
  full_name: string;
  status: string;
  roles: string[];
}

export interface UserPayload {
  nik: string;
  email: string;
  full_name: string;
  status: "active" | "inactive" | "locked";
  roles: string[];
  password?: string;
}

export interface ImportResult {
  imported: number;
  failed: number;
  errors: { row: number; error: string }[];
}

export const listUsers = () => apiFetch<{ users: UserRow[] }>("/users");

export const createUser = (payload: UserPayload) =>
  apiFetch<{ id: string }>("/users", {
    method: "POST",
    body: JSON.stringify(payload),
  });

export const updateUser = (id: string, payload: UserPayload) =>
  apiFetch<{ id: string }>(`/users/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });

export const deactivateUser = (id: string) =>
  apiFetch<{ id: string }>(`/users/${id}`, { method: "DELETE" });

export async function importUsers(file: File): Promise<ImportResult> {
  const form = new FormData();
  form.append("file", file);
  // FormData butuh boundary otomatis — jangan set Content-Type manual.
  const res = await fetch(`${BASE}/users/import`, {
    method: "POST",
    headers: { Authorization: `Bearer ${getAccessToken() ?? ""}` },
    body: form,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Import gagal");
  return data as ImportResult;
}

export async function downloadImportTemplate() {
  const res = await fetch(`${BASE}/users/import/template`, {
    headers: { Authorization: `Bearer ${getAccessToken() ?? ""}` },
  });
  if (!res.ok) throw new Error("Gagal mengunduh template");
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "template-import-pengguna.xlsx";
  a.click();
  URL.revokeObjectURL(url);
}
