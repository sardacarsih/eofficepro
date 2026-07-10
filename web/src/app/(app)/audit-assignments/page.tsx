"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  createAuditAssignment,
  deleteAuditAssignment,
  getAuditAssignmentOptions,
  listAuditAssignments,
  updateAuditAssignment,
  type AuditAssignment,
  type AuditAssignmentOptions,
  type AuditAssignmentPayload,
} from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";

const CLASSIFICATION_LABEL = {
  biasa: "Biasa",
  terbatas: "Terbatas",
  rahasia: "Rahasia",
} as const;

const CLASSIFICATION_STYLE = {
  biasa: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  terbatas: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  rahasia: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
} as const;

type FormState = AuditAssignmentPayload;

function today() {
  return new Date().toISOString().slice(0, 10);
}

function emptyForm(options: AuditAssignmentOptions): FormState {
  return {
    user_id: options.auditors[0]?.id ?? "",
    org_unit_id: options.org_units[0]?.org_unit_id ?? "",
    max_classification: "terbatas",
    can_export: false,
    valid_from: today(),
    valid_to: null,
  };
}

function assignmentToForm(assignment: AuditAssignment): FormState {
  return {
    user_id: assignment.user_id,
    org_unit_id: assignment.org_unit_id,
    max_classification: assignment.max_classification,
    can_export: assignment.can_export,
    valid_from: assignment.valid_from,
    valid_to: assignment.valid_to,
  };
}

export default function AuditAssignmentsPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const [assignments, setAssignments] = useState<AuditAssignment[]>([]);
  const [options, setOptions] = useState<AuditAssignmentOptions>({
    auditors: [],
    org_units: [],
  });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [deletingID, setDeletingID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [editing, setEditing] = useState<AuditAssignment | null>(null);
  const [form, setForm] = useState<FormState | null>(null);

  useEffect(() => {
    if (me && !me.roles.includes("admin")) router.replace("/organization");
  }, [me, router]);

  async function reload() {
    const [assignmentData, optionData] = await Promise.all([
      listAuditAssignments(),
      getAuditAssignmentOptions(),
    ]);
    setAssignments(assignmentData.data);
    setOptions(optionData);
  }

  useEffect(() => {
    let cancelled = false;
    Promise.all([listAuditAssignments(), getAuditAssignmentOptions()])
      .then(([assignmentData, optionData]) => {
        if (cancelled) return;
        setAssignments(assignmentData.data);
        setOptions(optionData);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Gagal memuat scope audit");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(emptyForm(options));
    setFormError(null);
  }

  function openEdit(assignment: AuditAssignment) {
    setEditing(assignment);
    setForm(assignmentToForm(assignment));
    setFormError(null);
  }

  function closeDialog() {
    if (busy) return;
    setEditing(null);
    setForm(null);
    setFormError(null);
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!form) return;
    const payload: AuditAssignmentPayload = {
      ...form,
      valid_to: form.valid_to?.trim() || null,
    };
    if (!payload.user_id || !payload.org_unit_id) {
      setFormError("Pilih auditor dan unit organisasi.");
      return;
    }
    if (payload.valid_to && payload.valid_to <= payload.valid_from) {
      setFormError("Tanggal selesai harus setelah tanggal mulai.");
      return;
    }

    setBusy(true);
    setFormError(null);
    try {
      if (editing) {
        await updateAuditAssignment(editing.id, payload);
      } else {
        await createAuditAssignment(payload);
      }
      await reload();
      setEditing(null);
      setForm(null);
      setFormError(null);
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal menyimpan scope audit");
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete(assignment: AuditAssignment) {
    if (!confirm(`Hapus scope audit untuk ${assignment.user_name}?`)) return;
    setDeletingID(assignment.id);
    setError(null);
    try {
      await deleteAuditAssignment(assignment.id);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menghapus scope audit");
    } finally {
      setDeletingID(null);
    }
  }

  const canCreate = options.auditors.length > 0 && options.org_units.length > 0;

  return (
    <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
            Scope Audit
          </h1>
          <p className="text-sm text-zinc-500">
            Batasi unit, periode, klasifikasi, dan ekspor untuk setiap auditor.
          </p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          disabled={!canCreate || loading}
          className="rounded-lg bg-navy-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Tambah Scope
        </button>
      </div>

      {!loading && !canCreate && (
        <p className="mb-4 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200">
          Tambahkan role auditor pada pengguna aktif dan pastikan unit organisasi tersedia terlebih dahulu.
        </p>
      )}
      {error && (
        <p role="alert" className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
          {error}
        </p>
      )}

      <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[820px] text-left text-sm">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
              <tr>
                <th className="px-4 py-3">Auditor</th>
                <th className="px-4 py-3">Unit Scope</th>
                <th className="px-4 py-3">Klasifikasi Maks.</th>
                <th className="px-4 py-3">Ekspor</th>
                <th className="px-4 py-3">Periode</th>
                <th className="px-4 py-3 text-right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr><td colSpan={6} className="px-4 py-8 text-center text-zinc-500">Memuat scope audit...</td></tr>
              )}
              {!loading && assignments.length === 0 && (
                <tr><td colSpan={6} className="px-4 py-8 text-center text-zinc-500">Belum ada scope audit.</td></tr>
              )}
              {!loading && assignments.map((assignment) => (
                <tr key={assignment.id} className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60">
                  <td className="px-4 py-3"><p className="font-medium text-zinc-900 dark:text-zinc-100">{assignment.user_name}</p><p className="text-xs text-zinc-500">{assignment.user_email}</p></td>
                  <td className="px-4 py-3"><p className="font-medium text-zinc-800 dark:text-zinc-200">{assignment.org_unit_name}</p><p className="font-mono text-xs text-zinc-500">{assignment.org_unit_code}</p></td>
                  <td className="px-4 py-3"><span className={`rounded-full px-2 py-1 text-xs font-medium ${CLASSIFICATION_STYLE[assignment.max_classification]}`}>{CLASSIFICATION_LABEL[assignment.max_classification]}</span></td>
                  <td className="px-4 py-3">{assignment.can_export ? "Diizinkan" : "Tidak diizinkan"}</td>
                  <td className="px-4 py-3 text-zinc-600 dark:text-zinc-300">{assignment.valid_from} — {assignment.valid_to ?? "tanpa batas"}</td>
                  <td className="px-4 py-3 text-right"><div className="inline-flex gap-2"><button type="button" onClick={() => openEdit(assignment)} disabled={deletingID !== null} className="rounded-md border border-zinc-300 px-2.5 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800">Ubah</button><button type="button" onClick={() => void handleDelete(assignment)} disabled={deletingID !== null} className="rounded-md border border-red-200 px-2.5 py-1.5 text-xs font-semibold text-red-700 hover:bg-red-50 disabled:opacity-60 dark:border-red-900 dark:text-red-300 dark:hover:bg-red-950">{deletingID === assignment.id ? "Menghapus..." : "Hapus"}</button></div></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {form && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/50 p-4" role="dialog" aria-modal="true" aria-labelledby="audit-scope-title">
          <form onSubmit={handleSubmit} className="w-full max-w-xl rounded-xl bg-white p-6 shadow-xl dark:bg-zinc-900">
            <div className="mb-5 flex items-start justify-between gap-4"><div><h2 id="audit-scope-title" className="text-lg font-semibold text-zinc-950 dark:text-zinc-50">{editing ? "Ubah Scope Audit" : "Tambah Scope Audit"}</h2><p className="mt-1 text-sm text-zinc-500">Scope berlaku juga pada seluruh sub-unit di bawah unit yang dipilih.</p></div><button type="button" onClick={closeDialog} disabled={busy} className="text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100">Tutup</button></div>
            {formError && <p role="alert" className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{formError}</p>}
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="sm:col-span-2 text-sm font-medium text-zinc-700 dark:text-zinc-300">Auditor<select value={form.user_id} onChange={(event) => setForm({ ...form, user_id: event.target.value })} className="mt-1 block w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"><option value="">Pilih auditor</option>{options.auditors.map((auditor) => <option key={auditor.id} value={auditor.id}>{auditor.full_name} — {auditor.email}</option>)}</select></label>
              <label className="sm:col-span-2 text-sm font-medium text-zinc-700 dark:text-zinc-300">Unit organisasi<select value={form.org_unit_id} onChange={(event) => setForm({ ...form, org_unit_id: event.target.value })} className="mt-1 block w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950"><option value="">Pilih unit</option>{options.org_units.map((unit) => <option key={unit.org_unit_id} value={unit.org_unit_id}>{unit.name} ({unit.code})</option>)}</select></label>
              <label className="text-sm font-medium text-zinc-700 dark:text-zinc-300">Klasifikasi maksimum<select value={form.max_classification} onChange={(event) => setForm({ ...form, max_classification: event.target.value as FormState["max_classification"] })} className="mt-1 block w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950">{Object.entries(CLASSIFICATION_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
              <label className="flex items-center gap-2 self-end rounded-lg border border-zinc-200 px-3 py-2 text-sm font-medium text-zinc-700 dark:border-zinc-700 dark:text-zinc-300"><input type="checkbox" checked={form.can_export} onChange={(event) => setForm({ ...form, can_export: event.target.checked })} />Izinkan ekspor</label>
              <label className="text-sm font-medium text-zinc-700 dark:text-zinc-300">Berlaku mulai<input type="date" required value={form.valid_from} onChange={(event) => setForm({ ...form, valid_from: event.target.value })} className="mt-1 block w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950" /></label>
              <label className="text-sm font-medium text-zinc-700 dark:text-zinc-300">Berakhir (opsional)<input type="date" value={form.valid_to ?? ""} onChange={(event) => setForm({ ...form, valid_to: event.target.value || null })} className="mt-1 block w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-950" /></label>
            </div>
            <div className="mt-6 flex justify-end gap-3"><button type="button" onClick={closeDialog} disabled={busy} className="rounded-lg border border-zinc-300 px-3 py-2 text-sm font-semibold text-zinc-700 dark:border-zinc-700 dark:text-zinc-300">Batal</button><button type="submit" disabled={busy} className="rounded-lg bg-navy-700 px-3 py-2 text-sm font-semibold text-white hover:bg-navy-800 disabled:opacity-60">{busy ? "Menyimpan..." : "Simpan Scope"}</button></div>
          </form>
        </div>
      )}
    </main>
  );
}
