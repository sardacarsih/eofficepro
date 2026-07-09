"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  createApprovalMatrix,
  deactivateApprovalMatrix,
  listApprovalMatrices,
  listLetterTypes,
  updateApprovalMatrix,
  type ApprovalMatrix,
  type ApprovalMatrixFinalLevel,
  type ApprovalMatrixPayload,
  type ApprovalMatrixPositionLevel,
  type LetterType,
} from "@/lib/api";
import { POSITION_TYPE_LABEL } from "@/lib/position-types";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";

const ORIGINATOR_LEVELS: ApprovalMatrixPositionLevel[] = [
  "secretary",
  "staff",
  "assistant",
  "division_head",
  "sub_dept_head",
  "dept_head",
  "gm",
  "director",
  "vp_director",
  "president_director",
  "auditor",
];

const FINAL_LEVELS: ApprovalMatrixFinalLevel[] = [
  "division_head",
  "sub_dept_head",
  "dept_head",
  "gm",
  "director",
  "vp_director",
  "president_director",
];

const LEVEL_BADGE_STYLE: Partial<Record<ApprovalMatrixFinalLevel, string>> = {
  director: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  vp_director: "bg-indigo-100 text-indigo-800 dark:bg-indigo-950 dark:text-indigo-300",
  president_director:
    "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
};

interface MatrixFormState {
  letter_type_id: string;
  originator_level: "" | ApprovalMatrixPositionLevel;
  final_level: ApprovalMatrixFinalLevel;
  is_active: boolean;
}

function emptyForm(letterTypes: LetterType[]): MatrixFormState {
  return {
    letter_type_id: letterTypes.find((item) => item.is_active)?.id ?? "",
    originator_level: "",
    final_level: "director",
    is_active: true,
  };
}

function matrixToForm(matrix: ApprovalMatrix): MatrixFormState {
  return {
    letter_type_id: matrix.letter_type_id,
    originator_level: matrix.originator_level ?? "",
    final_level: matrix.final_level,
    is_active: matrix.is_active,
  };
}

function compactPayload(form: MatrixFormState): ApprovalMatrixPayload {
  return {
    letter_type_id: form.letter_type_id,
    originator_level: form.originator_level || null,
    final_level: form.final_level,
    flow_mode: "serial",
    is_active: form.is_active,
  };
}

function levelLabel(level: string | null): string {
  if (!level) return "Semua Level";
  return POSITION_TYPE_LABEL[level] ?? level;
}

function ruleSummary(matrix: ApprovalMatrix): string {
  const finalLabel = levelLabel(matrix.final_level);
  if (!matrix.originator_level) {
    return `Semua Level -> ${finalLabel} -> Terbit`;
  }
  if (matrix.originator_level === "secretary" && matrix.final_level === "director") {
    return `Secretary -> Director -> Terbit`;
  }
  if (
    matrix.originator_level === "secretary" &&
    matrix.final_level === "president_director"
  ) {
    return `Secretary -> Director -> President Director -> Terbit`;
  }
  if (matrix.originator_level === "secretary" && matrix.final_level === "vp_director") {
    return `Secretary -> Director -> Vice President Director -> Terbit`;
  }
  return `${levelLabel(matrix.originator_level)} -> ${finalLabel} -> Terbit`;
}

export default function ApprovalMatricesPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const [matrices, setMatrices] = useState<ApprovalMatrix[]>([]);
  const [letterTypes, setLetterTypes] = useState<LetterType[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [modalError, setModalError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [actionID, setActionID] = useState<string | null>(null);
  const [editing, setEditing] = useState<ApprovalMatrix | null>(null);
  const [form, setForm] = useState<MatrixFormState | null>(null);
  const modalOpen = editing !== null || form !== null;

  async function reload() {
    const [matrixData, typeData] = await Promise.all([
      listApprovalMatrices(true),
      listLetterTypes(true),
    ]);
    setMatrices(matrixData.approval_matrices);
    setLetterTypes(typeData.letter_types);
  }

  useEffect(() => {
    if (me && !me.roles.includes("admin")) {
      router.replace("/organization");
    }
  }, [me, router]);

  useEffect(() => {
    Promise.all([listApprovalMatrices(true), listLetterTypes(true)])
      .then(([matrixData, typeData]) => {
        setMatrices(matrixData.approval_matrices);
        setLetterTypes(typeData.letter_types);
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat matrix approval"),
      )
      .finally(() => setLoading(false));
  }, []);

  const activeCount = useMemo(
    () => matrices.filter((item) => item.is_active).length,
    [matrices],
  );

  const selectableLetterTypes = useMemo(() => {
    if (!editing) return letterTypes.filter((item) => item.is_active);
    return letterTypes.filter((item) => item.is_active || item.id === editing.letter_type_id);
  }, [editing, letterTypes]);

  function openCreate() {
    setEditing(null);
    setForm(emptyForm(letterTypes));
    setModalError(null);
  }

  function openEdit(matrix: ApprovalMatrix) {
    setEditing(matrix);
    setForm(matrixToForm(matrix));
    setModalError(null);
  }

  function closeModal() {
    if (busy) return;
    setEditing(null);
    setForm(null);
    setModalError(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setModalError(null);
    try {
      const payload = compactPayload(form);
      if (!payload.letter_type_id) {
        throw new Error("Jenis surat wajib dipilih");
      }
      if (editing) {
        await updateApprovalMatrix(editing.id, payload);
      } else {
        await createApprovalMatrix(payload);
      }
      await reload();
      setEditing(null);
      setForm(null);
    } catch (err) {
      setModalError(
        err instanceof Error ? err.message : "Gagal menyimpan matrix approval",
      );
    } finally {
      setBusy(false);
    }
  }

  async function handleDeactivate(matrix: ApprovalMatrix) {
    if (
      !confirm(
        `Nonaktifkan matrix ${matrix.letter_type_code} untuk ${levelLabel(matrix.originator_level)}?`,
      )
    ) {
      return;
    }
    setActionID(matrix.id);
    setError(null);
    try {
      await deactivateApprovalMatrix(matrix.id);
      await reload();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal menonaktifkan matrix approval",
      );
    } finally {
      setActionID(null);
    }
  }

  return (
    <>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
              Matrix Approval
            </h1>
            <p className="text-sm text-zinc-500">
              {activeCount} aktif dari {matrices.length} aturan approval
            </p>
          </div>
          <button
            onClick={openCreate}
            disabled={letterTypes.filter((item) => item.is_active).length === 0}
            className="rounded-lg bg-navy-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Tambah Matrix
          </button>
        </div>

        {error && (
          <p
            role="alert"
            className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
          >
            {error}
          </p>
        )}

        <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[900px] text-left text-sm">
              <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
                <tr>
                  <th className="px-4 py-3">Jenis Surat</th>
                  <th className="px-4 py-3">Originator Level</th>
                  <th className="px-4 py-3">Final Approval</th>
                  <th className="px-4 py-3">Alur</th>
                  <th className="px-4 py-3">Mode</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={7} className="px-4 py-8 text-center text-zinc-500">
                      Memuat matrix approval...
                    </td>
                  </tr>
                )}
                {!loading && matrices.length === 0 && (
                  <tr>
                    <td colSpan={7} className="px-4 py-8 text-center text-zinc-500">
                      Belum ada matrix approval.
                    </td>
                  </tr>
                )}
                {!loading &&
                  matrices.map((matrix) => {
                    const actionBusy = actionID === matrix.id;
                    return (
                      <tr
                        key={matrix.id}
                        className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60"
                      >
                        <td className="px-4 py-3">
                          <p className="font-mono text-xs font-semibold text-zinc-700 dark:text-zinc-300">
                            {matrix.letter_type_code}
                          </p>
                          <p className="mt-0.5 font-medium text-zinc-900 dark:text-zinc-100">
                            {matrix.letter_type_name}
                          </p>
                        </td>
                        <td className="px-4 py-3 text-zinc-700 dark:text-zinc-300">
                          {levelLabel(matrix.originator_level)}
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                              LEVEL_BADGE_STYLE[matrix.final_level] ??
                              "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
                            }`}
                          >
                            {levelLabel(matrix.final_level)}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-xs font-semibold text-zinc-500">
                          {ruleSummary(matrix)}
                        </td>
                        <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">
                          Serial
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                              matrix.is_active
                                ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                                : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                            }`}
                          >
                            {matrix.is_active ? "Aktif" : "Nonaktif"}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex justify-end gap-3">
                            <button
                              onClick={() => openEdit(matrix)}
                              className="text-xs font-semibold text-zinc-600 hover:text-zinc-950 hover:underline dark:text-zinc-300 dark:hover:text-white"
                            >
                              Edit
                            </button>
                            {matrix.is_active && (
                              <button
                                onClick={() => handleDeactivate(matrix)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-red-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400"
                              >
                                {actionBusy ? "Memproses" : "Nonaktifkan"}
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
              </tbody>
            </table>
          </div>
        </div>
      </main>

      {modalOpen && form && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="approval-matrix-form-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <form
            onSubmit={handleSubmit}
            className="max-h-full w-full max-w-2xl overflow-y-auto rounded-xl bg-white shadow-2xl dark:bg-zinc-900"
          >
            <div className="flex items-start justify-between border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <div>
                <h2
                  id="approval-matrix-form-title"
                  className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  {editing ? "Edit Matrix Approval" : "Tambah Matrix Approval"}
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  Tentukan level approval terakhir sebelum surat terbit.
                </p>
              </div>
              <button
                type="button"
                onClick={closeModal}
                className="rounded-lg px-2 py-1 text-xl leading-none text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:hover:bg-zinc-800 dark:hover:text-white"
                aria-label="Tutup"
              >
                x
              </button>
            </div>

            <div className="grid gap-4 px-6 py-5 sm:grid-cols-2">
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Jenis Surat
                <select
                  value={form.letter_type_id}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, letter_type_id: e.target.value } : current,
                    )
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  {selectableLetterTypes.length === 0 && (
                    <option value="">Tidak ada jenis surat aktif</option>
                  )}
                  {selectableLetterTypes.map((letterType) => (
                    <option key={letterType.id} value={letterType.id}>
                      {letterType.code} - {letterType.name}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Originator Level
                <select
                  value={form.originator_level}
                  onChange={(e) =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            originator_level: e.target
                              .value as MatrixFormState["originator_level"],
                          }
                        : current,
                    )
                  }
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="">Semua Level</option>
                  {ORIGINATOR_LEVELS.map((level) => (
                    <option key={level} value={level}>
                      {levelLabel(level)}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Final Approval
                <select
                  value={form.final_level}
                  onChange={(e) =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            final_level: e.target.value as ApprovalMatrixFinalLevel,
                          }
                        : current,
                    )
                  }
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  {FINAL_LEVELS.map((level) => (
                    <option key={level} value={level}>
                      {levelLabel(level)}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Mode
                <input
                  value="Serial"
                  disabled
                  className="h-10 rounded-lg border border-zinc-300 bg-zinc-100 px-3 text-sm font-normal text-zinc-500 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-400"
                />
              </label>

              <label className="flex items-center gap-3 self-end rounded-lg border border-zinc-300 bg-white px-3 py-2.5 text-sm font-semibold text-zinc-800 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-200">
                <input
                  type="checkbox"
                  checked={form.is_active}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, is_active: e.target.checked } : current,
                    )
                  }
                  className="h-4 w-4 rounded border-zinc-300 text-navy-700 focus:ring-navy-600"
                />
                Aktif
              </label>

              <div className="rounded-lg border border-cyan-200 bg-cyan-50 px-3 py-2 text-sm text-cyan-900 dark:border-cyan-900 dark:bg-cyan-950/40 dark:text-cyan-100">
                {levelLabel(form.originator_level || null)} -&gt;{" "}
                {levelLabel(form.final_level)} -&gt; Terbit
              </div>

              {modalError && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300 sm:col-span-2"
                >
                  {modalError}
                </p>
              )}
            </div>

            <div className="flex justify-end gap-2 border-t border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <button
                type="button"
                onClick={closeModal}
                disabled={busy}
                className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={busy}
                className="rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busy ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}
