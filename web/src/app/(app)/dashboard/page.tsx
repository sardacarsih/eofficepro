"use client";

import { useEffect, useMemo, useState } from "react";
import ActivityCard from "@/components/dashboard/ActivityCard";
import ApprovalTable from "@/components/dashboard/ApprovalTable";
import StatCard from "@/components/dashboard/StatCard";
import TrendChart from "@/components/dashboard/TrendChart";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import { FileTextIcon } from "@/components/layout/icons";
import { getDashboardSummary, type DashboardSummary } from "@/lib/api";
import { relativeTime } from "@/lib/format";
import type {
  DashboardActivity,
  DashboardActivityIcon,
  DashboardStat,
  IncomingMailPoint,
  PendingApprovalItem,
} from "@/lib/dashboard-data";

const ACTIVITY_ICON_BY_EVENT: Record<string, DashboardActivityIcon> = {
  letter_incoming: "surat-masuk",
  approval_waiting: "approval",
  approval_result: "surat-keluar",
  disposition_assigned: "disposisi",
  disposition_updated: "disposisi",
  sla_reminder: "approval",
  sla_escalation: "approval",
};

function trendLabel(isoDate: string): string {
  const date = new Date(`${isoDate}T00:00:00`);
  if (Number.isNaN(date.getTime())) return isoDate;
  return date.toLocaleDateString("id-ID", { day: "numeric", month: "short" });
}

export default function DashboardPage() {
  const me = useCurrentUser();
  const firstName = (me?.full_name ?? "Admin").split(/\s+/)[0] || "Admin";
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    getDashboardSummary()
      .then((data) => {
        if (active) setSummary(data);
      })
      .catch((err) => {
        if (active)
          setError(
            err instanceof Error ? err.message : "Gagal memuat dashboard",
          );
      });
    return () => {
      active = false;
    };
  }, []);

  const stats = useMemo<DashboardStat[]>(() => {
    const s = summary?.stats;
    return [
      {
        id: "surat-masuk",
        label: "Surat Masuk",
        value: s?.inbox_unread ?? 0,
        note: "Belum dibaca",
        icon: "inbox",
        accent: "sky",
      },
      {
        id: "surat-keluar",
        label: "Surat Keluar",
        value: s?.sent_this_month ?? 0,
        note: "Terbit bulan ini",
        icon: "send",
        accent: "cyan",
      },
      {
        id: "approval",
        label: "Approval",
        value: s?.pending_approvals ?? 0,
        note: "Menunggu persetujuan",
        icon: "approval",
        accent: "amber",
      },
      {
        id: "arsip",
        label: "Arsip Dokumen",
        value: s?.archived_total ?? 0,
        note: "Total surat terbit",
        icon: "archive",
        accent: "violet",
      },
    ];
  }, [summary]);

  const trend = useMemo<IncomingMailPoint[]>(
    () =>
      (summary?.incoming_trend ?? []).map((point) => ({
        date: trendLabel(point.date),
        total: point.total,
      })),
    [summary],
  );

  const activities = useMemo<DashboardActivity[]>(
    () =>
      (summary?.recent_activities ?? []).map((activity) => ({
        id: activity.id,
        title: activity.title,
        time: relativeTime(activity.created_at),
        icon: ACTIVITY_ICON_BY_EVENT[activity.event_type] ?? "surat-masuk",
      })),
    [summary],
  );

  const pendingApprovals = useMemo<PendingApprovalItem[]>(
    () =>
      (summary?.pending_approvals ?? []).map((item) => ({
        id: item.step_id,
        document: item.subject,
        requester: item.creator_name,
        date: relativeTime(item.updated_at),
        status: "pending" as const,
      })),
    [summary],
  );

  return (
    <main className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 px-4 py-6 sm:px-6">
      {/* Welcome banner */}
      <section className="relative overflow-hidden rounded-2xl bg-gradient-to-r from-navy-900 via-navy-800 to-navy-700 px-6 py-7 shadow-sm sm:px-8">
        <div className="pointer-events-none absolute -right-10 -top-16 h-56 w-56 rounded-full bg-cyan-400/10 blur-2xl" />
        <div className="pointer-events-none absolute -bottom-20 right-32 h-48 w-48 rounded-full bg-sky-500/10 blur-2xl" />
        <div className="relative flex items-center justify-between gap-6">
          <div>
            <h2 className="text-xl font-bold text-white sm:text-2xl">
              Selamat datang, {firstName}
            </h2>
            <p className="mt-1.5 max-w-xl text-sm text-navy-200">
              Kelola surat, disposisi, dan dokumen dengan lebih efisien.
            </p>
          </div>
          <div className="hidden shrink-0 sm:block" aria-hidden>
            <div className="flex h-20 w-20 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-cyan-300 backdrop-blur-sm">
              <FileTextIcon className="h-10 w-10" />
            </div>
          </div>
        </div>
      </section>

      {error && (
        <p className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
          {error}
        </p>
      )}

      {/* KPI cards */}
      <section
        aria-label="Ringkasan statistik"
        className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
      >
        {stats.map((stat) => (
          <StatCard key={stat.id} stat={stat} />
        ))}
      </section>

      {/* Grafik + aktivitas */}
      <section className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <TrendChart data={trend} />
        </div>
        <ActivityCard activities={activities} />
      </section>

      {/* Pending approval */}
      <ApprovalTable items={pendingApprovals} />
    </main>
  );
}
