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

function apiError(data: unknown): string | null {
  if (!data || typeof data !== "object" || !("error" in data)) return null;
  const error = (data as { error: unknown }).error;
  return typeof error === "string" ? error : null;
}

async function parseAPIResponse<T>(
  res: Response,
  fallbackMessage: string,
): Promise<T> {
  const text = await res.text();
  const snippet = text.replace(/\s+/g, " ").trim().slice(0, 200);

  if (!text) {
    if (!res.ok) throw new Error(`${fallbackMessage} (${res.status})`);
    return {} as T;
  }

  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    if (!res.ok) {
      throw new Error(
        snippet
          ? `${fallbackMessage} (${res.status}): ${snippet}`
          : `${fallbackMessage} (${res.status})`,
      );
    }
    throw new Error(
      snippet
        ? `Respons API tidak valid (${res.status}): ${snippet}`
        : `Respons API tidak valid (${res.status})`,
    );
  }

  if (!res.ok) throw new Error(apiError(data) ?? `${fallbackMessage} (${res.status})`);
  return data as T;
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
  const data = await parseAPIResponse<{
    access_token: string;
    refresh_token: string;
    user: User;
  }>(res, "Login gagal");
  setTokens(data.access_token, data.refresh_token);
  return data.user;
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
  const data = await parseAPIResponse<{ access_token: string; refresh_token: string }>(
    res,
    "Refresh sesi gagal",
  );
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

  return parseAPIResponse<T>(res, "Permintaan gagal");
}

export const getMe = () => apiFetch<User>("/auth/me");
export const getOrgTree = () =>
  apiFetch<{ tree: OrgUnit[]; total: number }>("/org-units");

export interface Position {
  id: string;
  title: string;
  position_type: string;
  is_approver: boolean;
  is_active: boolean;
  reports_to: string | null;
  reports_to_title: string;
  org_unit_id: string;
  org_unit_name: string;
  org_unit_level: string;
  holder_name: string;
  holder_user_id: string;
  identity_locked: boolean;
}

export interface PositionPayload {
  org_unit_id: string;
  title: string;
  position_type: string;
  reports_to: string | null;
  is_approver: boolean;
}

export interface PositionDeactivationImpact {
  active_assignments: number;
  active_subordinates: number;
  active_delegations: number;
  active_drafts: number;
  active_approvals: number;
  active_dispositions: number;
  can_deactivate: boolean;
}

export const listPositions = (includeInactive = false) =>
  apiFetch<{ positions: Position[] }>(
    `/positions${includeInactive ? "?include_inactive=true" : ""}`,
  );

export const createPosition = (payload: PositionPayload) =>
  apiFetch<{ id: string }>("/positions", {
    method: "POST",
    body: JSON.stringify(payload),
  });

export const updatePosition = (id: string, payload: PositionPayload) =>
  apiFetch<{ id: string }>(`/positions/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });

export const getPositionDeactivationImpact = (id: string) =>
  apiFetch<{ impact: PositionDeactivationImpact }>(
    `/positions/${id}/deactivation-impact`,
  );

export const deactivatePosition = (id: string) =>
  apiFetch<{ id: string }>(`/positions/${id}`, { method: "DELETE" });

export const activatePosition = (id: string) =>
  apiFetch<{ id: string }>(`/positions/${id}/activate`, { method: "POST" });

export interface AssignPositionPayload {
  user_id: string;
  assignment_type?: "definitive" | "plt" | "plh";
}

export const assignPosition = (positionID: string, payload: AssignPositionPayload) =>
  apiFetch<{ id: string }>(`/positions/${positionID}/assign`, {
    method: "POST",
    body: JSON.stringify(payload),
  });

export const endUserPositionAssignment = (assignmentID: string) =>
  apiFetch<{ id: string }>(`/user-positions/${assignmentID}`, {
    method: "DELETE",
  });

export interface Company {
  id: string;
  code: string;
  name: string;
  is_active: boolean;
}

export const listCompanies = () => apiFetch<{ companies: Company[] }>("/companies");

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

// ---- Template surat ----

export interface LetterTemplate {
  id: string;
  letter_type_id: string;
  letter_type_code: string;
  letter_type_name: string;
  company_id: string;
  company_code: string;
  company_name: string;
  version: number;
  layout_config: Record<string, unknown>;
  body_skeleton: string;
  is_active: boolean;
  created_at: string;
}

export interface LetterTemplatePayload {
  letter_type_id: string;
  company_id: string;
  version?: number;
  layout_config: Record<string, unknown>;
  body_skeleton: string;
  is_active?: boolean;
}

export const listLetterTemplates = (includeInactive = false) =>
  apiFetch<{ letter_templates: LetterTemplate[] }>(
    `/letter-templates${includeInactive ? "?include_inactive=true" : ""}`,
  );

export const createLetterTemplate = (payload: LetterTemplatePayload) =>
  apiFetch<{ id: string }>("/letter-templates", {
    method: "POST",
    body: JSON.stringify(payload),
  });

export const updateLetterTemplate = (id: string, payload: LetterTemplatePayload) =>
  apiFetch<{ id: string }>(`/letter-templates/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });

export const activateLetterTemplate = (id: string) =>
  apiFetch<{ id: string }>(`/letter-templates/${id}/activate`, { method: "POST" });

export const deactivateLetterTemplate = (id: string) =>
  apiFetch<{ id: string }>(`/letter-templates/${id}`, { method: "DELETE" });

// ---- Draft surat ----

export interface DraftLetter {
  id: string;
  company_id: string;
  company_code: string;
  company_name: string;
  letter_type_id: string;
  letter_type_code: string;
  letter_type_name: string;
  letter_number: string | null;
  subject: string;
  classification: LetterType["default_classification"];
  priority: "normal" | "urgent";
  status:
    | "draft"
    | "submitted"
    | "in_approval"
    | "revision"
    | "approved"
    | "published"
    | "cancelled"
    | "archived";
  creator_position_id: string;
  creator_position_title: string;
  on_behalf_of_position_id: string | null;
  on_behalf_of_title: string | null;
  version: number;
  body_html: string;
  body_plain: string;
  recipients: DraftRecipient[];
  created_at: string;
  updated_at: string;
}

export interface DraftRecipient {
  type: "to" | "cc";
  target_type: "position" | "org_unit";
  target_id: string;
  label: string;
}

export interface DraftLetterPayload {
  company_id: string;
  letter_type_id: string;
  creator_position_id: string;
  on_behalf_of_position_id?: string | null;
  subject: string;
  classification?: LetterType["default_classification"];
  priority: DraftLetter["priority"];
  body_html: string;
  recipients: Omit<DraftRecipient, "label">[];
}

export const listDraftLetters = () =>
  apiFetch<{ letters: DraftLetter[] }>("/letters/drafts");

export const listMyLetters = () => apiFetch<{ letters: DraftLetter[] }>("/letters/mine");

export const getDraftLetter = (id: string) =>
  apiFetch<{ letter: DraftLetter }>(`/letters/drafts/${id}`);

export const createDraftLetter = (payload: DraftLetterPayload) =>
  apiFetch<{ id: string; version: number }>("/letters/drafts", {
    method: "POST",
    body: JSON.stringify(payload),
  });

export const updateDraftLetter = (id: string, payload: DraftLetterPayload) =>
  apiFetch<{ id: string; version: number }>(`/letters/drafts/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });

export interface LetterAttachment {
  id: string;
  file_name: string;
  mime_type: string;
  size_bytes: number;
  storage_key: string;
  checksum_sha256: string;
  scan_status: "pending" | "clean" | "infected" | "failed";
  download_url?: string;
  created_at: string;
}

export interface DraftPreviewResult {
  storage_key: string;
  preview_url: string;
  expires_in: number;
}

export interface SubmitDraftResult {
  id: string;
  status: "in_approval";
  qr_token: string;
  verify_url: string;
  approval_steps: {
    step_order: number;
    flow_group: number;
    position_id: string;
    position_type: string;
    title: string;
  }[];
}

export const listDraftAttachments = (id: string) =>
  apiFetch<{ attachments: LetterAttachment[] }>(`/letters/drafts/${id}/attachments`);

export async function uploadDraftAttachment(
  id: string,
  file: File,
): Promise<{ id: string }> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${BASE}/letters/drafts/${id}/attachments`, {
    method: "POST",
    headers: { Authorization: `Bearer ${getAccessToken() ?? ""}` },
    body: form,
  });
  return parseAPIResponse<{ id: string }>(res, "Upload lampiran gagal");
}

export const deleteDraftAttachment = (draftID: string, attachmentID: string) =>
  apiFetch<{ id: string }>(`/letters/drafts/${draftID}/attachments/${attachmentID}`, {
    method: "DELETE",
  });

export const previewDraftLetter = (id: string) =>
  apiFetch<DraftPreviewResult>(`/letters/drafts/${id}/preview`, { method: "POST" });

export const submitDraftLetter = (id: string) =>
  apiFetch<SubmitDraftResult>(`/letters/drafts/${id}/submit`, { method: "POST" });

// ---- Inbox surat masuk ----

export interface IncomingLetter {
  id: string;
  recipient_type: "to" | "cc";
  company_code: string;
  letter_type_code: string;
  letter_number: string | null;
  subject: string;
  classification: DraftLetter["classification"];
  priority: DraftLetter["priority"];
  creator_name: string;
  creator_position_title: string;
  body_plain: string;
  attachment_count: number;
  is_read: boolean;
  first_read_at: string | null;
  last_read_at: string | null;
  delivered_at: string | null;
  published_at: string | null;
  updated_at: string;
}

export const listIncomingLetters = (box?: "to" | "cc") =>
  apiFetch<{ letters: IncomingLetter[] }>(
    `/letters/inbox${box ? `?box=${box}` : ""}`,
  );

// ---- Approval surat ----

export interface ApprovalInboxItem {
  step_id: string;
  letter_id: string;
  step_order: number;
  status: "waiting";
  subject: string;
  priority: DraftLetter["priority"];
  classification: DraftLetter["classification"];
  letter_type_code: string;
  company_code: string;
  position_title: string;
  creator_name: string;
  creator_position: string;
  body_plain: string;
  attachment_count: number;
  updated_at: string;
}

export interface ApprovalActionPayload {
  action: "approve" | "reject" | "request_revision";
  note?: string;
  client_action_id?: string;
  device_info?: string;
}

export interface ApprovalActionResult {
  letter_id: string;
  status: DraftLetter["status"];
}

export const listApprovalInbox = () =>
  apiFetch<{ approvals: ApprovalInboxItem[] }>("/approvals/inbox");

export const actApprovalStep = (stepID: string, payload: ApprovalActionPayload) =>
  apiFetch<ApprovalActionResult>(`/approvals/steps/${stepID}/actions`, {
    method: "POST",
    body: JSON.stringify(payload),
  });

// ---- Detail surat ----

export interface LetterApprovalStep {
  id: string;
  step_order: number;
  flow_group: number;
  status: "pending" | "waiting" | "approved" | "rejected" | "skipped";
  position_title: string;
  position_type: string;
  sla_deadline: string | null;
  decided_at: string | null;
}

export interface LetterApprovalAction {
  id: string;
  step_id: string;
  action: "approve" | "reject" | "request_revision";
  actor_name: string;
  note: string | null;
  device_info: string | null;
  created_at: string;
  position_title: string;
}

export interface LetterDetail {
  id: string;
  company_code: string;
  company_name: string;
  letter_type_code: string;
  letter_type_name: string;
  letter_number: string | null;
  subject: string;
  classification: string;
  priority: DraftLetter["priority"];
  status: DraftLetter["status"];
  creator_name: string;
  creator_position_title: string;
  on_behalf_of_title: string | null;
  version: number;
  body_html: string;
  body_plain: string;
  qr_token: string | null;
  verify_url: string | null;
  final_pdf_url: string | null;
  recipients: DraftRecipient[];
  attachments: LetterAttachment[];
  approval_steps: LetterApprovalStep[];
  approval_actions: LetterApprovalAction[];
  created_at: string;
  updated_at: string;
  published_at: string | null;
}

export const getLetterDetail = (id: string) =>
  apiFetch<{ letter: LetterDetail }>(`/letters/view/${id}`);

export interface VerifiedLetter {
  id: string;
  company_code: string;
  company_name: string;
  letter_type_code: string;
  letter_type_name: string;
  letter_number: string | null;
  subject: string;
  classification: string;
  status: string;
  published_at: string | null;
  created_at: string;
}

export async function verifyLetter(token: string): Promise<VerifiedLetter> {
  const res = await fetch(`${BASE}/verify/${encodeURIComponent(token)}`);
  const data = await parseAPIResponse<{ letter: VerifiedLetter }>(
    res,
    "Token verifikasi tidak valid",
  );
  return data.letter;
}

// ---- Reset password (publik) ----

export async function forgotPassword(email: string): Promise<string> {
  const res = await fetch(`${BASE}/auth/forgot-password`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  const data = await parseAPIResponse<{ message: string }>(res, "Permintaan gagal");
  return data.message;
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
  const data = await parseAPIResponse<{ message: string }>(res, "Reset password gagal");
  return data.message;
}

// ---- Pencarian surat ----

export interface LetterSearchResult {
  id: string;
  company_code: string;
  letter_type_code: string;
  letter_number: string | null;
  subject: string;
  status: string;
  classification: string;
  creator_name: string;
  origin: "mine" | "received";
  snippet: string;
  published_at: string | null;
  updated_at: string;
}

export const searchLetters = (query: string) =>
  apiFetch<{ results: LetterSearchResult[]; query: string }>(
    `/letters/search?q=${encodeURIComponent(query)}`,
  );

// ---- Dashboard ----

export interface DashboardSummary {
  stats: {
    inbox_unread: number;
    sent_this_month: number;
    pending_approvals: number;
    archived_total: number;
  };
  incoming_trend: { date: string; total: number }[];
  recent_activities: {
    id: string;
    event_type: string;
    title: string;
    created_at: string;
  }[];
  pending_approvals: {
    step_id: string;
    subject: string;
    creator_name: string;
    updated_at: string;
  }[];
}

export const getDashboardSummary = () =>
  apiFetch<DashboardSummary>("/dashboard/summary");

// ---- Notifikasi in-app ----

export interface AppNotification {
  id: string;
  event_type: "approval_waiting" | "approval_result" | "letter_incoming" | string;
  letter_id: string | null;
  title: string;
  body: string;
  read_at: string | null;
  created_at: string;
}

export const listNotifications = (limit = 15) =>
  apiFetch<{ notifications: AppNotification[]; unread_count: number }>(
    `/notifications?limit=${limit}`,
  );

export const markNotificationRead = (id: string) =>
  apiFetch<{ id: string; updated: boolean }>(`/notifications/${id}/read`, {
    method: "POST",
  });

export const markAllNotificationsRead = () =>
  apiFetch<{ marked_read: number }>("/notifications/read-all", {
    method: "POST",
  });

// ---- Ubah password (pengguna login) ----

export async function changePassword(
  currentPassword: string,
  newPassword: string,
): Promise<string> {
  const data = await apiFetch<{ message: string }>("/auth/change-password", {
    method: "POST",
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
  });
  return data.message;
}

// ---- Manajemen pengguna (admin) ----

export interface UserRow {
  id: string;
  nik: string;
  email: string;
  full_name: string;
  status: string;
  roles: string[];
  positions: UserPositionAssignment[];
}

export interface UserPositionAssignment {
  assignment_id: string;
  position_id: string;
  title: string;
  position_type: string;
  org_unit_name: string;
  assignment_type: "definitive" | "plt" | "plh";
  valid_from: string;
  valid_to: string | null;
}

export interface UserPayload {
  nik: string;
  email: string;
  full_name: string;
  status: "active" | "inactive" | "locked";
  roles: string[];
  password?: string;
  positions: UserPositionPayload[];
}

export interface UserPositionPayload {
  position_id: string;
  assignment_type: "definitive" | "plt" | "plh";
}

export interface DeactivationImpact {
  positions: {
    position_id: string;
    title: string;
    org_unit_name: string;
    assignment_type: UserPositionPayload["assignment_type"];
  }[];
  drafts: {
    letter_id: string;
    subject: string;
    creator_position_id: string;
    creator_position_title: string;
    status: string;
  }[];
  approval_steps: {
    step_id: string;
    letter_id: string;
    subject: string;
    position_id: string;
    position_title: string;
    status: string;
  }[];
  has_impact: boolean;
}

export interface DeactivateUserPayload {
  position_replacements: {
    position_id: string;
    replacement_user_id: string;
    assignment_type: UserPositionPayload["assignment_type"];
  }[];
  draft_transfers: {
    letter_id: string;
    replacement_user_id: string;
    replacement_position_id: string;
  }[];
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

export const getUserDeactivationImpact = (id: string) =>
  apiFetch<{ impact: DeactivationImpact }>(`/users/${id}/deactivation-impact`);

export const deactivateUser = (id: string, payload?: DeactivateUserPayload) =>
  payload
    ? apiFetch<{ id: string }>(`/users/${id}/deactivate`, {
        method: "POST",
        body: JSON.stringify(payload),
      })
    : apiFetch<{ id: string }>(`/users/${id}`, { method: "DELETE" });

export async function importUsers(file: File): Promise<ImportResult> {
  const form = new FormData();
  form.append("file", file);
  // FormData butuh boundary otomatis — jangan set Content-Type manual.
  const res = await fetch(`${BASE}/users/import`, {
    method: "POST",
    headers: { Authorization: `Bearer ${getAccessToken() ?? ""}` },
    body: form,
  });
  return parseAPIResponse<ImportResult>(res, "Import gagal");
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
