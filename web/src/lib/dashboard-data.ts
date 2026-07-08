// Data dummy dashboard — dipisahkan agar mudah diganti dengan pemanggilan API
// (mis. GET /dashboard/summary) tanpa menyentuh markup komponen.

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

export const DASHBOARD_STATS: DashboardStat[] = [
  {
    id: "surat-masuk",
    label: "Surat Masuk",
    value: 12,
    note: "Belum dibaca",
    icon: "inbox",
    accent: "sky",
  },
  {
    id: "surat-keluar",
    label: "Surat Keluar",
    value: 18,
    note: "Bulan ini",
    icon: "send",
    accent: "cyan",
  },
  {
    id: "approval",
    label: "Approval",
    value: 5,
    note: "Menunggu persetujuan",
    icon: "approval",
    accent: "amber",
  },
  {
    id: "arsip",
    label: "Arsip Dokumen",
    value: 256,
    note: "Total dokumen",
    icon: "archive",
    accent: "violet",
  },
];

export const RECENT_ACTIVITIES: DashboardActivity[] = [
  {
    id: "act-1",
    title: "Surat masuk baru dari BPK RI",
    time: "10 menit yang lalu",
    icon: "surat-masuk",
  },
  {
    id: "act-2",
    title: "Disposisi surat ke Sekretariat",
    time: "35 menit yang lalu",
    icon: "disposisi",
  },
  {
    id: "act-3",
    title: "Approval dokumen anggaran",
    time: "1 jam yang lalu",
    icon: "approval",
  },
  {
    id: "act-4",
    title: "Surat keluar ke Dinas Pendidikan",
    time: "2 jam yang lalu",
    icon: "surat-keluar",
  },
];

// Trend surat masuk 30 hari terakhir (label tanggal statis agar render
// server/klien konsisten; nanti diganti data agregat dari API).
export const INCOMING_MAIL_TREND: IncomingMailPoint[] = [
  { date: "9 Jun", total: 8 },
  { date: "10 Jun", total: 6 },
  { date: "11 Jun", total: 9 },
  { date: "12 Jun", total: 7 },
  { date: "13 Jun", total: 10 },
  { date: "14 Jun", total: 12 },
  { date: "15 Jun", total: 9 },
  { date: "16 Jun", total: 11 },
  { date: "17 Jun", total: 14 },
  { date: "18 Jun", total: 10 },
  { date: "19 Jun", total: 13 },
  { date: "20 Jun", total: 15 },
  { date: "21 Jun", total: 12 },
  { date: "22 Jun", total: 16 },
  { date: "23 Jun", total: 14 },
  { date: "24 Jun", total: 11 },
  { date: "25 Jun", total: 17 },
  { date: "26 Jun", total: 13 },
  { date: "27 Jun", total: 15 },
  { date: "28 Jun", total: 18 },
  { date: "29 Jun", total: 14 },
  { date: "30 Jun", total: 16 },
  { date: "1 Jul", total: 19 },
  { date: "2 Jul", total: 15 },
  { date: "3 Jul", total: 17 },
  { date: "4 Jul", total: 20 },
  { date: "5 Jul", total: 16 },
  { date: "6 Jul", total: 18 },
  { date: "7 Jul", total: 21 },
  { date: "8 Jul", total: 17 },
];

export const PENDING_APPROVALS: PendingApprovalItem[] = [
  {
    id: "apr-1",
    document: "Anggaran Operasional",
    requester: "Budi Santoso",
    date: "Hari ini",
    status: "pending",
  },
  {
    id: "apr-2",
    document: "Memo Internal HRD",
    requester: "Siti Aminah",
    date: "Kemarin",
    status: "pending",
  },
  {
    id: "apr-3",
    document: "Surat Kerja Sama",
    requester: "Andi Wijaya",
    date: "2 hari lalu",
    status: "pending",
  },
];
