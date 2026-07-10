"use client";

import { useState } from "react";
import Link from "next/link";
import { forgotPassword } from "@/lib/api";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      setMessage(await forgotPassword(email));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Permintaan gagal");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="flex flex-1 items-center justify-center bg-navy-50/50 px-4 dark:bg-zinc-950">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <h1 className="text-xl font-semibold text-zinc-900 dark:text-zinc-50">
            Lupa Password
          </h1>
          <p className="text-sm text-zinc-500">
            Masukkan email terdaftar — kode reset 6 digit akan dikirim.
          </p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="flex flex-col gap-4 rounded-2xl border border-zinc-200 bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
        >
          <label className="flex flex-col gap-1.5 text-sm font-medium text-zinc-700 dark:text-zinc-300">
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
              className="rounded-lg border border-zinc-300 px-3 py-2 text-zinc-900 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/20 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
            />
          </label>

          {message && (
            <p className="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
              {message}
            </p>
          )}
          {error && (
            <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
              {error}
            </p>
          )}

          {message ? (
            <Link
              href={`/reset-password?email=${encodeURIComponent(email)}`}
              className="rounded-lg bg-navy-700 py-2.5 text-center text-sm font-semibold text-white transition hover:bg-navy-800"
            >
              Masukkan Kode Reset
            </Link>
          ) : (
            <button
              type="submit"
              disabled={loading}
              className="rounded-lg bg-navy-700 py-2.5 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:opacity-60"
            >
              {loading ? "Mengirim..." : "Kirim Kode Reset"}
            </button>
          )}

          <Link
            href="/login"
            className="text-center text-sm text-navy-600 hover:underline dark:text-sky-400"
          >
            Kembali ke login
          </Link>
        </form>
      </div>
    </main>
  );
}
