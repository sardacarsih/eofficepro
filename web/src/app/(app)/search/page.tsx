"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { searchLetters, type LetterSearchResult } from "@/lib/api";
import { relativeTime } from "@/lib/format";
import { SearchIcon } from "@/components/layout/icons";

const STATUS_LABEL: Record<string, string> = {
  draft: "Draft",
  submitted: "Diajukan",
  in_approval: "Approval",
  revision: "Revisi",
  approved: "Disetujui",
  published: "Terbit",
  cancelled: "Dibatalkan",
  archived: "Arsip",
};

const ORIGIN_LABEL: Record<LetterSearchResult["origin"], string> = {
  mine: "Surat saya",
  received: "Diterima",
};

// Halaman hasil pencarian surat — target navigasi search box topbar.
function SearchResults() {
  const params = useSearchParams();
  const query = (params.get("q") ?? "").trim();
  // Hasil & error disimpan bersama kata kuncinya; kata kunci berbeda dari
  // URL berarti pencarian masih berjalan (tanpa reset state di effect).
  const [fetched, setFetched] = useState<{
    query: string;
    results: LetterSearchResult[];
    error: string | null;
  } | null>(null);

  useEffect(() => {
    if (query.length < 2) return;
    let active = true;
    searchLetters(query)
      .then((data) => {
        if (active) setFetched({ query, results: data.results, error: null });
      })
      .catch((err) => {
        if (active)
          setFetched({
            query,
            results: [],
            error: err instanceof Error ? err.message : "Pencarian gagal",
          });
      });
    return () => {
      active = false;
    };
  }, [query]);

  const current = fetched?.query === query ? fetched : null;
  const results = query.length < 2 ? [] : (current?.results ?? null);
  const error = current?.error ?? null;

  return (
    <main className="mx-auto w-full max-w-4xl flex-1 px-6 py-8">
      <div className="mb-6">
        <h2 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
          Hasil Pencarian
        </h2>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          {query.length < 2
            ? "Ketik minimal 2 karakter di kotak pencarian."
            : `Kata kunci: "${query}"`}
        </p>
      </div>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
          {error}
        </p>
      )}

      {results === null ? (
        <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
          Mencari...
        </p>
      ) : results.length === 0 ? (
        !error &&
        query.length >= 2 && (
          <div className="rounded-lg border border-dashed border-zinc-300 px-4 py-10 text-center dark:border-zinc-700">
            <SearchIcon className="mx-auto h-8 w-8 text-zinc-300 dark:text-zinc-600" />
            <p className="mt-3 text-sm text-zinc-500">
              Tidak ada surat yang cocok dengan kata kunci ini.
            </p>
          </div>
        )
      ) : (
        <div className="grid gap-3">
          {results.map((item) => (
            <Link
              key={`${item.origin}-${item.id}`}
              href={`/letters/${item.id}`}
              className="block rounded-xl border border-zinc-200 bg-white p-4 transition hover:border-navy-300 hover:shadow-sm dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-navy-700"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded-full bg-navy-50 px-2.5 py-0.5 text-[11px] font-semibold text-navy-700 dark:bg-navy-950 dark:text-navy-300">
                  {item.letter_type_code}
                </span>
                <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-[11px] font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
                  {STATUS_LABEL[item.status] ?? item.status}
                </span>
                <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-[11px] font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
                  {ORIGIN_LABEL[item.origin]}
                </span>
                <span className="ml-auto text-xs text-zinc-400">
                  {relativeTime(item.updated_at)}
                </span>
              </div>
              <p className="mt-2 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                {item.subject}
              </p>
              <p className="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">
                {item.letter_number ?? "Tanpa nomor"} · {item.creator_name} ·{" "}
                {item.company_code}
              </p>
              {item.snippet && (
                <p className="mt-2 line-clamp-2 text-xs text-zinc-500 dark:text-zinc-400">
                  {item.snippet}
                </p>
              )}
            </Link>
          ))}
        </div>
      )}
    </main>
  );
}

export default function SearchPage() {
  return (
    <Suspense fallback={null}>
      <SearchResults />
    </Suspense>
  );
}
