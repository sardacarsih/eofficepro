"use client";

import ActivityCard from "@/components/dashboard/ActivityCard";
import ApprovalTable from "@/components/dashboard/ApprovalTable";
import StatCard from "@/components/dashboard/StatCard";
import TrendChart from "@/components/dashboard/TrendChart";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import { FileTextIcon } from "@/components/layout/icons";
import {
  DASHBOARD_STATS,
  INCOMING_MAIL_TREND,
  PENDING_APPROVALS,
  RECENT_ACTIVITIES,
} from "@/lib/dashboard-data";

export default function DashboardPage() {
  const me = useCurrentUser();
  const firstName = (me?.full_name ?? "Admin").split(/\s+/)[0] || "Admin";

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

      {/* KPI cards */}
      <section
        aria-label="Ringkasan statistik"
        className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
      >
        {DASHBOARD_STATS.map((stat) => (
          <StatCard key={stat.id} stat={stat} />
        ))}
      </section>

      {/* Grafik + aktivitas */}
      <section className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <TrendChart data={INCOMING_MAIL_TREND} />
        </div>
        <ActivityCard activities={RECENT_ACTIVITIES} />
      </section>

      {/* Pending approval */}
      <ApprovalTable items={PENDING_APPROVALS} />
    </main>
  );
}
