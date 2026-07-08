"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  createLetterType,
  deactivateLetterType,
  listLetterTypes,
  updateLetterType,
  type LetterType,
  type LetterTypePayload,
} from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";

const CLASSIFICATION_LABEL: Record<LetterType["default_classification"], string> = {
  biasa: "Biasa",
  terbatas: "Terbatas",
  rahasia: "Rahasia",
};

const CLASSIFICATION_STYLE: Record<LetterType["default_classification"], string> = {
  biasa: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  terbatas: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  rahasia: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
};

interface LetterTypeFormState {
  code: string;
  name: string;
  default_classification: LetterType["default_classification"];
  default_sla_hours: string;
  is_active: boolean;
}

function emptyForm(): LetterTypeFormState {
  return {
    code: "",
    name: "",
    default_classification: "biasa",
    default_sla_hours: "24",
    is_active: true,
  };
}

function letterTypeToForm(letterType: LetterType): LetterTypeFormState {
  return {
    code: letterType.code,
    name: letterType.name,
    default_classification: letterType.default_classification,
    default_sla_hours: String(letterType.default_sla_hours),
    is_active: letterType.is_active,
  };
}

function compactPayload(form: LetterTypeFormState): LetterTypePayload {
  return {
    code: form.code.trim().toUpperCase(),
    name: form.name.trim(),
    default_classification: form.default_classification,
    default_sla_hours: Number(form.default_sla_hours),
    is_active: form.is_active,
  };
}

export default function LetterTypesPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const [letterTypes, setLetterTypes] = useState<LetterType[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [modalError, setModalError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [actionID, setActionID] = useState<string | null>(null);
  const [editing, setEditing] = useState<LetterType | null>(null);
  const [form, setForm] = useState<LetterTypeFormState | null>(null);
  const modalOpen = editing !== null || form !== null;

  async function reload() {
    const data = await listLetterTypes(true);
    setLetterTypes(data.letter_types);
  }

  // Halaman khusus admin — alihkan role lain setelah profil termuat.
  useEffect(() => {
    if (me && !me.roles.includes("admin")) {
      router.replace("/organization");
    }
  }, [me, router]);

  useEffect(() => {
    listLetterTypes(true)
      .then((data) => setLetterTypes(data.letter_types))
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat jenis surat"),
      )
      .finally(() => setLoading(false));
  }, []);

  const activeCount = useMemo(
    () => letterTypes.filter((item) => item.is_active).length,
    [letterTypes],
  );

  function openCreate() {
    setEditing(null);
    setForm(emptyForm());
    setModalError(null);
  }

  function openEdit(letterType: LetterType) {
    setEditing(letterType);
    setForm(letterTypeToForm(letterType));
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
      if (payload.code.length > 5) {
        throw new Error("Kode maksimal 5 karakter");
      }
      if (!Number.isInteger(payload.default_sla_hours) || payload.default_sla_hours < 1) {
        throw new Error("SLA wajib berupa jam positif");
      }
      if (editing) {
        await updateLetterType(editing.id, payload);
      } else {
        await createLetterType(payload);
      }
      await reload();
      setEditing(null);
      setForm(null);
    } catch (err) {
      setModalError(err instanceof Error ? err.message : "Gagal menyimpan jenis surat");
    } finally {
      setBusy(false);
    }
  }

  async function handleDeactivate(letterType: LetterType) {
    if (!confirm(`Nonaktifkan jenis surat ${letterType.code} - ${letterType.name}?`)) return;
    setActionID(letterType.id);
    setError(null);
    try {
      await deactivateLetterType(letterType.id);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menonaktifkan jenis surat");
    } finally {
      setActionID(null);
    }
  }

  async function handleReactivate(letterType: LetterType) {
    setActionID(letterType.id);
    setError(null);
    try {
      await updateLetterType(letterType.id, {
        code: letterType.code,
        name: letterType.name,
        default_classification: letterType.default_classification,
        default_sla_hours: letterType.default_sla_hours,
        is_active: true,
      });
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengaktifkan jenis surat");
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
              Jenis Surat
            </h1>
            <p className="text-sm text-zinc-500">
              {activeCount} aktif dari {letterTypes.length} master surat
            </p>
          </div>
          <button
            onClick={openCreate}
            className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800"
          >
            Tambah Jenis Surat
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
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
                <tr>
                  <th className="px-4 py-3">Kode</th>
                  <th className="px-4 py-3">Nama</th>
                  <th className="px-4 py-3">Klasifikasi Default</th>
                  <th className="px-4 py-3">SLA</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-zinc-500">
                      Memuat jenis surat...
                    </td>
                  </tr>
                )}
                {!loading && letterTypes.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-zinc-500">
                      Belum ada jenis surat.
                    </td>
                  </tr>
                )}
                {!loading &&
                  letterTypes.map((letterType) => {
                    const actionBusy = actionID === letterType.id;

                    return (
                      <tr
                        key={letterType.id}
                        className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60"
                      >
                        <td className="px-4 py-3 font-mono text-xs font-semibold text-zinc-700 dark:text-zinc-300">
                          {letterType.code}
                        </td>
                        <td className="px-4 py-3 font-medium text-zinc-900 dark:text-zinc-100">
                          {letterType.name}
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${CLASSIFICATION_STYLE[letterType.default_classification]}`}
                          >
                            {CLASSIFICATION_LABEL[letterType.default_classification]}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">
                          {letterType.default_sla_hours} jam
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                              letterType.is_active
                                ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                                : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                            }`}
                          >
                            {letterType.is_active ? "Aktif" : "Nonaktif"}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex justify-end gap-3">
                            <button
                              onClick={() => openEdit(letterType)}
                              className="text-xs font-semibold text-zinc-600 hover:text-zinc-950 hover:underline dark:text-zinc-300 dark:hover:text-white"
                            >
                              Edit
                            </button>
                            {letterType.is_active ? (
                              <button
                                onClick={() => handleDeactivate(letterType)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-red-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400"
                              >
                                {actionBusy ? "Memproses" : "Nonaktifkan"}
                              </button>
                            ) : (
                              <button
                                onClick={() => handleReactivate(letterType)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-emerald-700 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-emerald-400"
                              >
                                {actionBusy ? "Memproses" : "Aktifkan"}
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
          aria-labelledby="letter-type-form-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <form
            onSubmit={handleSubmit}
            className="max-h-full w-full max-w-xl overflow-y-auto rounded-xl bg-white shadow-2xl dark:bg-zinc-900"
          >
            <div className="flex items-start justify-between border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <div>
                <h2
                  id="letter-type-form-title"
                  className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  {editing ? "Edit Jenis Surat" : "Tambah Jenis Surat"}
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  Atur kode, nama, klasifikasi, dan SLA default.
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
                Kode
                <input
                  value={form.code}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, code: e.target.value.toUpperCase() } : current,
                    )
                  }
                  required
                  maxLength={5}
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal uppercase text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                SLA Default (jam)
                <input
                  type="number"
                  min={1}
                  max={720}
                  value={form.default_sla_hours}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, default_sla_hours: e.target.value } : current,
                    )
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 sm:col-span-2">
                Nama
                <input
                  value={form.name}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, name: e.target.value } : current,
                    )
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Klasifikasi Default
                <select
                  value={form.default_classification}
                  onChange={(e) =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            default_classification: e.target
                              .value as LetterType["default_classification"],
                          }
                        : current,
                    )
                  }
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="biasa">Biasa</option>
                  <option value="terbatas">Terbatas</option>
                  <option value="rahasia">Rahasia</option>
                </select>
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
                  className="h-4 w-4 rounded border-zinc-300 text-emerald-700 focus:ring-emerald-600"
                />
                Aktif
              </label>

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
                className="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
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
