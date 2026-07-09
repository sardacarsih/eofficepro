"use client";

import { useState } from "react";
import { XIcon } from "@/components/layout/icons";
import type { Company, CompanyPayload } from "@/lib/api";

interface CompanyFormDialogProps {
  company: Company | null;
  busy: boolean;
  error: string | null;
  onClose: () => void;
  onSave: (payload: CompanyPayload) => Promise<void>;
}

export default function CompanyFormDialog({
  company,
  busy,
  error,
  onClose,
  onSave,
}: CompanyFormDialogProps) {
  const [code, setCode] = useState(company?.code ?? "");
  const [name, setName] = useState(company?.name ?? "");
  const [isActive, setIsActive] = useState(company?.is_active ?? true);
  const [validationError, setValidationError] = useState<string | null>(null);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    const payload: CompanyPayload = {
      code: code.trim().toUpperCase(),
      name: name.trim(),
      is_active: isActive,
    };
    if (!payload.code) {
      setValidationError("Kode perusahaan wajib diisi.");
      return;
    }
    if (payload.code.length > 10) {
      setValidationError("Kode perusahaan maksimal 10 karakter.");
      return;
    }
    if (!payload.name) {
      setValidationError("Nama perusahaan wajib diisi.");
      return;
    }
    if (payload.name.length > 150) {
      setValidationError("Nama perusahaan maksimal 150 karakter.");
      return;
    }
    setValidationError(null);
    await onSave(payload);
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="company-form-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/50 px-4 py-6 backdrop-blur-[2px]"
    >
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-lg overflow-hidden rounded-2xl border border-white/10 bg-white shadow-2xl dark:bg-zinc-900"
      >
        <div className="flex items-start justify-between border-b border-zinc-200 px-6 py-5 dark:border-zinc-800">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wider text-sky-700 dark:text-sky-300">
              Master company
            </p>
            <h2
              id="company-form-title"
              className="mt-1 text-lg font-semibold text-zinc-950 dark:text-zinc-50"
            >
              {company ? "Edit perusahaan" : "Tambah perusahaan"}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={busy}
            aria-label="Tutup dialog"
            className="rounded-lg p-1.5 text-zinc-400 transition hover:bg-zinc-100 hover:text-zinc-700 disabled:opacity-50 dark:hover:bg-zinc-800 dark:hover:text-zinc-200"
          >
            <XIcon className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-4 px-6 py-5">
          {(validationError || error) && (
            <p
              role="alert"
              className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
            >
              {validationError ?? error}
            </p>
          )}

          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-200">
            Kode perusahaan
            <input
              value={code}
              onChange={(event) => setCode(event.target.value.toUpperCase())}
              maxLength={10}
              autoFocus
              placeholder="Contoh: KSK"
              className="mt-1.5 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2.5 font-mono text-sm uppercase text-zinc-950 outline-none transition focus:border-sky-500 focus:ring-2 focus:ring-sky-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
            />
          </label>

          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-200">
            Nama perusahaan
            <input
              value={name}
              onChange={(event) => setName(event.target.value)}
              maxLength={150}
              placeholder="Nama badan usaha lengkap"
              className="mt-1.5 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2.5 text-sm text-zinc-950 outline-none transition focus:border-sky-500 focus:ring-2 focus:ring-sky-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
            />
          </label>

          <label className="flex cursor-pointer items-center justify-between rounded-lg border border-zinc-200 px-3 py-3 dark:border-zinc-700">
            <span>
              <span className="block text-sm font-medium text-zinc-800 dark:text-zinc-100">
                Company aktif
              </span>
              <span className="mt-0.5 block text-xs text-zinc-500">
                Company aktif tersedia saat membuat surat baru.
              </span>
            </span>
            <input
              type="checkbox"
              checked={isActive}
              onChange={(event) => setIsActive(event.target.checked)}
              className="h-4 w-4 rounded border-zinc-300 text-navy-700 focus:ring-sky-500"
            />
          </label>
        </div>

        <div className="flex justify-end gap-3 border-t border-zinc-200 bg-zinc-50 px-6 py-4 dark:border-zinc-800 dark:bg-zinc-900">
          <button
            type="button"
            onClick={onClose}
            disabled={busy}
            className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-white disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
          >
            Batal
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {busy ? "Menyimpan..." : company ? "Simpan perubahan" : "Tambah perusahaan"}
          </button>
        </div>
      </form>
    </div>
  );
}
