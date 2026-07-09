"use client";

import { useEffect, useState } from "react";
import { changePassword } from "@/lib/api";
import { LockIcon, XIcon } from "./icons";

const MIN_PASSWORD_LENGTH = 10;

interface ChangePasswordModalProps {
  open: boolean;
  onClose: () => void;
  // Dipanggil setelah sukses — backend mencabut semua sesi, jadi pemanggil
  // wajib mengarahkan pengguna untuk login ulang.
  onLogout: () => void;
}

// Modal ubah password: verifikasi password lama + konfirmasi password baru.
// Sukses = seluruh sesi dicabut backend, pengguna diminta login ulang.
// Isi modal dirender ulang setiap dibuka sehingga form selalu mulai kosong.
export default function ChangePasswordModal({
  open,
  onClose,
  onLogout,
}: ChangePasswordModalProps) {
  if (!open) return null;
  return <ChangePasswordDialog onClose={onClose} onLogout={onLogout} />;
}

function ChangePasswordDialog({
  onClose,
  onLogout,
}: Omit<ChangePasswordModalProps, "open">) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);

    if (newPassword.length < MIN_PASSWORD_LENGTH) {
      setError(`Password baru minimal ${MIN_PASSWORD_LENGTH} karakter`);
      return;
    }
    if (newPassword !== confirmPassword) {
      setError("Konfirmasi password tidak sama dengan password baru");
      return;
    }

    setSaving(true);
    try {
      const message = await changePassword(currentPassword, newPassword);
      setSuccessMessage(message);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengubah password");
    } finally {
      setSaving(false);
    }
  }

  const inputClass =
    "w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 transition focus:border-navy-400 focus:outline-none focus:ring-2 focus:ring-navy-100 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-100 dark:focus:ring-navy-800";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/50 px-4 py-6"
      role="dialog"
      aria-modal="true"
      aria-label="Ubah password"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-md overflow-hidden rounded-xl bg-white shadow-2xl dark:bg-zinc-900">
        <div className="flex items-center justify-between border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
          <div className="flex items-center gap-2.5">
            <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-navy-50 text-navy-700 dark:bg-navy-950 dark:text-navy-300">
              <LockIcon className="h-4.5 w-4.5" />
            </span>
            <div>
              <h2 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                Ubah Password
              </h2>
              <p className="text-xs text-zinc-500 dark:text-zinc-400">
                Minimal {MIN_PASSWORD_LENGTH} karakter
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 text-zinc-400 transition hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-800 dark:hover:text-zinc-200"
            aria-label="Tutup"
          >
            <XIcon className="h-5 w-5" />
          </button>
        </div>

        {successMessage ? (
          <div className="px-6 py-6">
            <p className="rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
              {successMessage}
            </p>
            <p className="mt-3 text-xs text-zinc-500 dark:text-zinc-400">
              Demi keamanan, semua sesi Anda telah dicabut. Silakan login ulang
              dengan password baru.
            </p>
            <button
              onClick={onLogout}
              className="mt-4 w-full rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800"
            >
              Login Ulang
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4 px-6 py-5">
            {error && (
              <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
                {error}
              </p>
            )}

            <label className="block">
              <span className="mb-1.5 block text-xs font-medium text-zinc-600 dark:text-zinc-300">
                Password Lama
              </span>
              <input
                autoFocus
                type="password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                required
                autoComplete="current-password"
                className={inputClass}
              />
            </label>

            <label className="block">
              <span className="mb-1.5 block text-xs font-medium text-zinc-600 dark:text-zinc-300">
                Password Baru
              </span>
              <input
                type="password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                required
                minLength={MIN_PASSWORD_LENGTH}
                autoComplete="new-password"
                className={inputClass}
              />
            </label>

            <label className="block">
              <span className="mb-1.5 block text-xs font-medium text-zinc-600 dark:text-zinc-300">
                Konfirmasi Password Baru
              </span>
              <input
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                required
                minLength={MIN_PASSWORD_LENGTH}
                autoComplete="new-password"
                className={inputClass}
              />
            </label>

            <div className="flex justify-end gap-2 pt-1">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-600 transition hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={saving}
                className="rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {saving ? "Menyimpan..." : "Simpan Password"}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
