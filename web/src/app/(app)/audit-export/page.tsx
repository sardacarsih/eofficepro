"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { downloadAuditLetterExport } from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";

function dateValue(date: Date) {
  return date.toISOString().slice(0, 10);
}

export default function AuditExportPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const [to, setTo] = useState(() => dateValue(new Date()));
  const [from, setFrom] = useState(() => {
    const date = new Date();
    date.setDate(date.getDate() - 30);
    return dateValue(date);
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (me && !me.capabilities?.can_export_audit) router.replace("/dashboard");
  }, [me, router]);

  async function handleExport() {
    if (from > to) {
      setError("Tanggal mulai tidak boleh melebihi tanggal selesai.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await downloadAuditLetterExport(from, to);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ekspor audit gagal");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto w-full max-w-3xl flex-1 px-6 py-8">
      <div className="rounded-xl border border-zinc-200 bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <p className="text-sm font-medium text-sky-700 dark:text-sky-300">Audit</p>
        <h1 className="mt-1 text-xl font-semibold text-zinc-950 dark:text-zinc-50">
          Ekspor Surat Audit
        </h1>
        <p className="mt-2 text-sm text-zinc-500 dark:text-zinc-400">
          CSV hanya memuat metadata surat terbit dalam unit, periode, dan klasifikasi
          yang diizinkan oleh scope audit Anda. Isi surat serta lampiran tidak diekspor.
        </p>

        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          <label className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
            Dari
            <input type="date" value={from} onChange={(event) => setFrom(event.target.value)} className="mt-1 block w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950" />
          </label>
          <label className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
            Sampai
            <input type="date" value={to} onChange={(event) => setTo(event.target.value)} className="mt-1 block w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950" />
          </label>
        </div>
        <p className="mt-3 text-xs text-zinc-500">Maksimum periode ekspor adalah 366 hari dan maksimum 10.000 baris.</p>

        {error && <p role="alert" className="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{error}</p>}

        <button type="button" onClick={() => void handleExport()} disabled={busy} className="mt-6 rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60">
          {busy ? "Menyiapkan CSV..." : "Unduh CSV"}
        </button>
      </div>
    </main>
  );
}
