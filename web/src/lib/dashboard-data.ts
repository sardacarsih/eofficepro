// Tipe tampilan komponen dashboard. Data diambil dari GET /dashboard/summary
// (lihat getDashboardSummary di lib/api.ts) dan dipetakan di halaman dashboard.

export type DashboardStatIcon = "inbox" | "send" | "approval" | "archive";
export type DashboardStatAccent = "sky" | "cyan" | "amber" | "violet";

export interface DashboardStat {
  id: string;
  label: string;
  value: number;
  note: string;
  icon: DashboardStatIcon;
  accent: DashboardStatAccent;
}

export type DashboardActivityIcon =
  | "surat-masuk"
  | "disposisi"
  | "approval"
  | "surat-keluar";

export interface DashboardActivity {
  id: string;
  title: string;
  time: string;
  icon: DashboardActivityIcon;
}

export interface IncomingMailPoint {
  date: string;
  total: number;
}

export type PendingApprovalStatus = "pending";

export interface PendingApprovalItem {
  id: string;
  document: string;
  requester: string;
  date: string;
  status: PendingApprovalStatus;
}
