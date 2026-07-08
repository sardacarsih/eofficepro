"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { verifyLetter, type VerifiedLetter } from "@/lib/api";

export default function VerifyLetterPage() {
  const params = useParams<{ token: string }>();
  const [letter, setLetter] = useState<VerifiedLetter | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    verifyLetter(params.token)
      .then(setLetter)
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Token verifikasi tidak valid"),
      )
      .finally(() => setLoading(false));
  }, [params.token]);

  return (
    <main className="flex min-h-screen items-center justify-center bg-[#f5f7fa] px-6 py-10 text-[#172033] dark:bg-zinc-950 dark:text-zinc-50">
      <section className="w-full max-w-xl rounded-lg border border-zinc-200 bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <div className="mb-5 flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-sky-400 to-cyan-500 text-sm font-bold text-navy-950">
            e
          </div>
          <div>
            <h1 className="text-lg font-semibold text-zinc-950 dark:text-zinc-50">
              Verifikasi Surat
            </h1>
            <p className="text-sm text-zinc-500">eOffice Pro</p>
          </div>
        </div>

        {loading && <p className="text-sm text-zinc-500">Memeriksa token...</p>}
        {error && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
            {error}
          </p>
        )}
        {letter && (
          <div className="grid gap-3 text-sm">
            <div className="rounded-lg bg-emerald-50 px-3 py-2 font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
              Token valid
            </div>
            <dl className="grid gap-2">
              <div className="grid grid-cols-[130px_1fr] gap-3">
                <dt className="text-zinc-500">Nomor</dt>
                <dd className="font-semibold">{letter.letter_number ?? "Belum terbit"}</dd>
              </div>
              <div className="grid grid-cols-[130px_1fr] gap-3">
                <dt className="text-zinc-500">Perusahaan</dt>
                <dd>{letter.company_name}</dd>
              </div>
              <div className="grid grid-cols-[130px_1fr] gap-3">
                <dt className="text-zinc-500">Jenis</dt>
                <dd>{letter.letter_type_code} - {letter.letter_type_name}</dd>
              </div>
              <div className="grid grid-cols-[130px_1fr] gap-3">
                <dt className="text-zinc-500">Perihal</dt>
                <dd>{letter.subject}</dd>
              </div>
              <div className="grid grid-cols-[130px_1fr] gap-3">
                <dt className="text-zinc-500">Status</dt>
                <dd>{letter.status}</dd>
              </div>
              <div className="grid grid-cols-[130px_1fr] gap-3">
                <dt className="text-zinc-500">Klasifikasi</dt>
                <dd>{letter.classification}</dd>
              </div>
            </dl>
          </div>
        )}
      </section>
    </main>
  );
}
