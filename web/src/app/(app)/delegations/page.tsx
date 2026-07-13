"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  createDelegation,
  listDelegateOptions,
  listDelegations,
  revokeDelegation,
  type DelegateOption,
  type Delegation,
  type DelegationStatus,
} from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import { PlusIcon, XIcon } from "@/components/layout/icons";

const REASON_MAX_LENGTH = 255;

// Status dihitung server dari waktu query — klien hanya menampilkan.
const STATUS_LABEL: Record<DelegationStatus, string> = {
  scheduled: "Terjadwal",
  active: "Aktif",
  expired: "Kedaluwarsa",
  revoked: "Dicabut",
};

const STATUS_STYLE: Record<DelegationStatus, string> = {
  scheduled: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  active: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  expired: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300",
  revoked: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
};

interface DelegationFormState {
  delegator_position_id: string;
  delegate_user_id: string;
  reason: string;
  valid_from: string;
  valid_to: string;
}

const EMPTY_FORM: DelegationFormState = {
  delegator_position_id: "",
  delegate_user_id: "",
  reason: "",
  valid_from: "",
  valid_to: "",
};

function formatDate(value: string | null): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return `${date.toLocaleDateString("id-ID")} ${date.toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
  })}`;
}

// datetime-local (waktu lokal) -> RFC3339 sesuai kontrak API.
function toRFC3339(local: string): string | null {
  if (!local) return null;
  const date = new Date(local);
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString();
}

export default function DelegationsPage() {
  const me = useCurrentUser();
  const [delegations, setDelegations] = useState<Delegation[]>([]);
  const [includePast, setIncludePast] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [form, setForm] = useState<DelegationFormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [delegateOptions, setDelegateOptions] = useState<DelegateOption[]>([]);
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [optionsError, setOptionsError] = useState<string | null>(null);

  const [revokeTarget, setRevokeTarget] = useState<Delegation | null>(null);
  const [revoking, setRevoking] = useState(false);
  const [revokeError, setRevokeError] = useState<string | null>(null);

  const myPositions = useMemo(() => me?.positions ?? [], [me]);

  const reload = useCallback(async () => {
    const data = await listDelegations(undefined, includePast);
    setDelegations(data.data);
  }, [includePast]);

  useEffect(() => {
    queueMicrotask(() => setLoading(true));
    listDelegations(undefined, includePast)
      .then((data) => setDelegations(data.data))
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat delegasi"),
      )
      .finally(() => setLoading(false));
  }, [includePast]);

  function openCreate() {
    setForm({
      ...EMPTY_FORM,
      delegator_position_id: myPositions[0]?.position_id ?? "",
    });
    setFormError(null);
    setSuccess(null);
  }

  function closeCreate() {
    if (saving) return;
    setForm(null);
    setFormError(null);
    setDelegateOptions([]);
    setOptionsError(null);
  }

  // Muat kandidat delegate setiap kali posisi delegator berubah.
  const selectedPositionID = form?.delegator_position_id ?? "";
  useEffect(() => {
    if (!selectedPositionID) {
      queueMicrotask(() => setDelegateOptions([]));
      return;
    }
    let active = true;
    queueMicrotask(() => {
      setOptionsLoading(true);
      setOptionsError(null);
      setDelegateOptions([]);
    });
    listDelegateOptions(selectedPositionID)
      .then((data) => {
        if (active) setDelegateOptions(data.data);
      })
      .catch((err) => {
        if (active)
          setOptionsError(
            err instanceof Error ? err.message : "Gagal memuat kandidat delegate",
          );
      })
      .finally(() => {
        if (active) setOptionsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [selectedPositionID]);

  async function handleCreate(event: React.FormEvent) {
    event.preventDefault();
    if (!form) return;

    const reason = form.reason.trim();
    if (!form.delegator_position_id) {
      setFormError("Pilih posisi yang didelegasikan.");
      return;
    }
    if (!form.delegate_user_id) {
      setFormError("Pilih penerima delegasi.");
      return;
    }
    if (!reason) {
      setFormError("Alasan delegasi wajib diisi.");
      return;
    }
    if (Array.from(reason).length > REASON_MAX_LENGTH) {
      setFormError(`Alasan maksimal ${REASON_MAX_LENGTH} karakter.`);
      return;
    }
    const validFrom = toRFC3339(form.valid_from);
    const validTo = toRFC3339(form.valid_to);
    if (!validFrom || !validTo) {
      setFormError("Tanggal mulai dan berakhir wajib diisi.");
      return;
    }
    if (validFrom >= validTo) {
      setFormError("Tanggal mulai harus sebelum tanggal berakhir.");
      return;
    }

    setSaving(true);
    setFormError(null);
    try {
      await createDelegation({
        delegator_position_id: form.delegator_position_id,
        delegate_user_id: form.delegate_user_id,
        reason,
        valid_from: validFrom,
        valid_to: validTo,
      });
      await reload();
      setForm(null);
      setDelegateOptions([]);
      setSuccess("Delegasi berhasil dibuat.");
    } catch (err) {
      // Termasuk 409 overlap — tampilkan pesan dari server apa adanya.
      setFormError(err instanceof Error ? err.message : "Gagal membuat delegasi");
    } finally {
      setSaving(false);
    }
  }

  async function confirmRevoke() {
    if (!revokeTarget) return;
    setRevoking(true);
    setRevokeError(null);
    try {
      await revokeDelegation(revokeTarget.id);
      await reload();
      setRevokeTarget(null);
      setSuccess("Delegasi berhasil dicabut.");
    } catch (err) {
      setRevokeError(err instanceof Error ? err.message : "Gagal mencabut delegasi");
    } finally {
      setRevoking(false);
    }
  }

  return (
    <>
      <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-5 px-4 py-6 sm:px-6">
        <header className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
              Delegasi Wewenang
            </h1>
            <p className="mt-1 text-sm text-zinc-500">
              Delegasikan wewenang approval posisi Anda untuk rentang waktu
              terbatas. Aksi delegate tercatat sebagai a.n. posisi asal.
            </p>
          </div>
          <button
            type="button"
            onClick={openCreate}
            disabled={myPositions.length === 0}
            className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-navy-700 px-4 text-sm font-semibold text-white shadow-sm transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <PlusIcon className="h-4 w-4" />
            Buat Delegasi
          </button>
        </header>

        <label className="flex items-center gap-2 text-sm text-zinc-700 dark:text-zinc-300">
          <input
            type="checkbox"
            checked={includePast}
            onChange={(event) => setIncludePast(event.target.checked)}
            className="h-4 w-4 rounded border-zinc-300 text-navy-700 focus:ring-navy-600"
          />
          Tampilkan riwayat (kedaluwarsa dan dicabut)
        </label>

        {success && (
          <p className="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
            {success}
          </p>
        )}
        {error && (
          <p
            role="alert"
            className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
          >
            {error}
          </p>
        )}

        {loading && <p className="text-sm text-zinc-500">Memuat delegasi...</p>}
        {!loading && !error && delegations.length === 0 && (
          <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
            {includePast
              ? "Belum ada delegasi yang tercatat."
              : "Tidak ada delegasi aktif atau terjadwal."}
          </p>
        )}

        <div className="grid gap-3">
          {delegations.map((delegation) => (
            <article
              key={delegation.id}
              className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="mb-2 flex flex-wrap items-center gap-2">
                    <span
                      className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${STATUS_STYLE[delegation.status]}`}
                    >
                      {STATUS_LABEL[delegation.status]}
                    </span>
                  </div>
                  <h2 className="text-base font-semibold text-zinc-950 dark:text-zinc-50">
                    {delegation.delegator_position_title}
                  </h2>
                  <p className="mt-1 text-sm text-zinc-700 dark:text-zinc-300">
                    Delegate: {delegation.delegate_name}
                  </p>
                  <p className="mt-1 text-sm text-zinc-500">
                    {formatDate(delegation.valid_from)} —{" "}
                    {formatDate(delegation.valid_to)}
                  </p>
                  <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-zinc-600 dark:text-zinc-400">
                    {delegation.reason}
                  </p>
                  <p className="mt-2 text-xs text-zinc-500">
                    Dibuat oleh {delegation.created_by_name} ·{" "}
                    {formatDate(delegation.created_at)}
                    {delegation.revoked_at
                      ? ` · dicabut ${formatDate(delegation.revoked_at)}`
                      : ""}
                  </p>
                </div>
                {(delegation.status === "scheduled" ||
                  delegation.status === "active") && (
                  <button
                    type="button"
                    onClick={() => {
                      setRevokeTarget(delegation);
                      setRevokeError(null);
                      setSuccess(null);
                    }}
                    className="rounded-lg border border-red-200 px-3 py-1.5 text-xs font-semibold text-red-700 transition hover:bg-red-50 dark:border-red-900 dark:text-red-300 dark:hover:bg-red-950"
                  >
                    Cabut
                  </button>
                )}
              </div>
            </article>
          ))}
        </div>
      </main>

      {form && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="delegation-form-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <form
            onSubmit={handleCreate}
            className="flex max-h-full w-full max-w-lg flex-col overflow-hidden rounded-xl bg-white shadow-2xl dark:bg-zinc-900"
          >
            <header className="flex items-start justify-between border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
              <h2
                id="delegation-form-title"
                className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
              >
                Buat Delegasi
              </h2>
              <button
                type="button"
                onClick={closeCreate}
                aria-label="Tutup"
                className="rounded-lg border border-zinc-200 p-2 text-zinc-500 hover:bg-zinc-100 dark:border-zinc-800 dark:hover:bg-zinc-800"
              >
                <XIcon className="h-4 w-4" />
              </button>
            </header>

            <div className="grid gap-4 overflow-y-auto px-5 py-5">
              <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Posisi yang didelegasikan
                <select
                  value={form.delegator_position_id}
                  onChange={(event) =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            delegator_position_id: event.target.value,
                            delegate_user_id: "",
                          }
                        : current,
                    )
                  }
                  disabled={saving}
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="">Pilih posisi</option>
                  {myPositions.map((position) => (
                    <option key={position.position_id} value={position.position_id}>
                      [{position.company_code}] {position.title} · {position.org_unit}
                    </option>
                  ))}
                </select>
              </label>

              <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Penerima delegasi
                <select
                  value={form.delegate_user_id}
                  onChange={(event) =>
                    setForm((current) =>
                      current
                        ? { ...current, delegate_user_id: event.target.value }
                        : current,
                    )
                  }
                  disabled={
                    saving || !form.delegator_position_id || optionsLoading
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="">
                    {!form.delegator_position_id
                      ? "Pilih posisi dahulu"
                      : optionsLoading
                        ? "Memuat kandidat..."
                        : delegateOptions.length === 0
                          ? "Tidak ada kandidat tersedia"
                          : "Pilih penerima delegasi"}
                  </option>
                  {delegateOptions.map((option) => (
                    <option key={option.user_id} value={option.user_id}>
                      {option.full_name}
                      {option.position_titles.length > 0
                        ? ` · ${option.position_titles.join(", ")}`
                        : ""}
                    </option>
                  ))}
                </select>
                {optionsError && (
                  <span className="text-xs font-normal text-red-700 dark:text-red-300">
                    {optionsError}
                  </span>
                )}
              </label>

              <div className="grid gap-4 sm:grid-cols-2">
                <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Mulai
                  <input
                    type="datetime-local"
                    value={form.valid_from}
                    onChange={(event) =>
                      setForm((current) =>
                        current
                          ? { ...current, valid_from: event.target.value }
                          : current,
                      )
                    }
                    disabled={saving}
                    required
                    className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                  />
                </label>
                <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Berakhir
                  <input
                    type="datetime-local"
                    value={form.valid_to}
                    onChange={(event) =>
                      setForm((current) =>
                        current
                          ? { ...current, valid_to: event.target.value }
                          : current,
                      )
                    }
                    disabled={saving}
                    required
                    className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                  />
                </label>
              </div>

              <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Alasan
                <textarea
                  value={form.reason}
                  onChange={(event) =>
                    setForm((current) =>
                      current ? { ...current, reason: event.target.value } : current,
                    )
                  }
                  disabled={saving}
                  required
                  rows={3}
                  maxLength={REASON_MAX_LENGTH}
                  placeholder="Contoh: cuti tahunan, dinas luar kota"
                  className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
                <span className="text-xs font-normal text-zinc-500">
                  {Array.from(form.reason).length}/{REASON_MAX_LENGTH}
                </span>
              </label>

              {formError && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  {formError}
                </p>
              )}
            </div>

            <footer className="flex shrink-0 flex-col-reverse gap-2 border-t border-zinc-200 px-5 py-4 dark:border-zinc-800 sm:flex-row sm:justify-end">
              <button
                type="button"
                onClick={closeCreate}
                disabled={saving}
                className="h-10 rounded-lg border border-zinc-300 px-4 text-sm font-semibold text-zinc-700 hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={saving}
                className="h-10 rounded-lg bg-navy-700 px-4 text-sm font-semibold text-white shadow-sm hover:bg-navy-800 disabled:opacity-50"
              >
                {saving ? "Menyimpan..." : "Buat Delegasi"}
              </button>
            </footer>
          </form>
        </div>
      )}

      {revokeTarget && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="delegation-revoke-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <div className="w-full max-w-md rounded-xl bg-white shadow-2xl dark:bg-zinc-900">
            <header className="flex items-start justify-between border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
              <div>
                <h2
                  id="delegation-revoke-title"
                  className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  Cabut Delegasi
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  {revokeTarget.delegator_position_title} →{" "}
                  {revokeTarget.delegate_name}
                </p>
              </div>
              <button
                type="button"
                onClick={() => {
                  if (!revoking) setRevokeTarget(null);
                }}
                aria-label="Tutup"
                className="rounded-lg border border-zinc-200 p-2 text-zinc-500 hover:bg-zinc-100 dark:border-zinc-800 dark:hover:bg-zinc-800"
              >
                <XIcon className="h-4 w-4" />
              </button>
            </header>

            <div className="px-5 py-5">
              <p className="text-sm text-zinc-600 dark:text-zinc-300">
                Pencabutan berlaku seketika: akses delegate ke inbox approval
                posisi ini langsung hilang. Lanjutkan?
              </p>
              {revokeError && (
                <p
                  role="alert"
                  className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  {revokeError}
                </p>
              )}
            </div>

            <footer className="flex justify-end gap-2 border-t border-zinc-200 px-5 py-4 dark:border-zinc-800">
              <button
                type="button"
                onClick={() => setRevokeTarget(null)}
                disabled={revoking}
                className="h-10 rounded-lg border border-zinc-300 px-4 text-sm font-semibold text-zinc-700 hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={confirmRevoke}
                disabled={revoking}
                className="h-10 rounded-lg bg-red-700 px-4 text-sm font-semibold text-white hover:bg-red-800 disabled:opacity-50"
              >
                {revoking ? "Mencabut..." : "Cabut Delegasi"}
              </button>
            </footer>
          </div>
        </div>
      )}
    </>
  );
}
