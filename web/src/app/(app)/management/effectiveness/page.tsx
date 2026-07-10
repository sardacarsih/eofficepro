"use client";

import { useEffect, useState } from "react";
import { getEffectivenessSummary, type EffectivenessSummary } from "@/lib/api";

function today() { return new Date().toISOString().slice(0, 10); }
function daysAgo(days: number) { const d = new Date(); d.setDate(d.getDate() - days); return d.toISOString().slice(0, 10); }

export default function EffectivenessPage() {
  const [from, setFrom] = useState(daysAgo(30));
  const [to, setTo] = useState(today());
  const [data, setData] = useState<EffectivenessSummary | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { getEffectivenessSummary(from, to).then(setData).catch((e) => setError(e instanceof Error ? e.message : "Gagal memuat KPI")); }, [from, to]);
  const cards = data ? [
    ["Pengguna aktif", `${data.active_users} / ${data.registered_users}`],
    ["Surat dibuat", data.letters_created],
    ["Surat terbit", data.letters_published],
    ["Approval selesai", data.approval_actions],
    ["Approval tertunda", data.pending_approvals],
    ["Lewat SLA", data.overdue_approvals],
    ["Notifikasi dibaca", `${data.read_notifications} / ${data.total_notifications}`],
  ] as const : [];
  return <main className="mx-auto max-w-7xl space-y-8 px-6 py-8">
    <header><p className="text-sm font-medium text-sky-600">Manajemen</p><h1 className="text-3xl font-semibold tracking-tight">Efektivitas eOffice Pro</h1><p className="mt-2 text-sm text-slate-500">Baseline penggunaan dan kinerja proses untuk evaluasi manajemen.</p></header>
    <section className="flex flex-wrap items-end gap-4 rounded-xl border border-slate-200 bg-white p-4">
      <label className="text-sm">Dari<input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className="mt-1 block rounded-md border px-3 py-2" /></label>
      <label className="text-sm">Sampai<input type="date" value={to} onChange={(e) => setTo(e.target.value)} className="mt-1 block rounded-md border px-3 py-2" /></label>
      <button type="button" onClick={() => window.print()} className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white">Cetak / PDF</button>
    </section>
    {error && <p className="rounded-md bg-red-50 p-4 text-sm text-red-700">{error}</p>}
    <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">{cards.map(([label, value]) => <article key={label} className="rounded-xl border border-slate-200 bg-white p-5"><p className="text-sm text-slate-500">{label}</p><p className="mt-2 text-3xl font-semibold text-slate-900">{value}</p></article>)}</section>
    {data && <p className="text-sm text-slate-500">Periode data: {data.from} sampai {data.to}. Gunakan baseline 1–3 bulan pertama sebelum menetapkan target.</p>}
  </main>;
}