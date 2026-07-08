"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  getMe,
  getOrgTree,
  logout,
  getAccessToken,
  type OrgUnit,
  type User,
} from "@/lib/api";

const LEVEL_LABEL: Record<string, string> = {
  directorate: "Direktorat",
  biro: "Biro",
  department: "Department",
  section: "Section",
  division: "Division",
  office: "Office",
};

const LEVEL_STYLE: Record<string, string> = {
  directorate:
    "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  biro: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  department:
    "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  section:
    "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-300",
  division: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  office: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
};

function UnitNode({ unit, depth }: { unit: OrgUnit; depth: number }) {
  return (
    <li>
      <div
        className="flex flex-wrap items-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2 dark:border-zinc-800 dark:bg-zinc-900"
        style={{ marginLeft: depth * 24 }}
      >
        <span
          className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${LEVEL_STYLE[unit.unit_level] ?? ""}`}
        >
          {LEVEL_LABEL[unit.unit_level] ?? unit.unit_level}
        </span>
        <span className="font-medium text-zinc-900 dark:text-zinc-100">
          {unit.name}
        </span>
        <span className="font-mono text-xs text-zinc-400">{unit.code}</span>
        {unit.region && (
          <span className="ml-auto text-xs text-zinc-400">{unit.region}</span>
        )}
      </div>
      {unit.children && unit.children.length > 0 && (
        <ul className="mt-2 flex flex-col gap-2">
          {unit.children.map((child) => (
            <UnitNode key={child.id} unit={child} depth={depth + 1} />
          ))}
        </ul>
      )}
    </li>
  );
}

export default function OrganizationPage() {
  const router = useRouter();
  const [me, setMe] = useState<User | null>(null);
  const [tree, setTree] = useState<OrgUnit[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    Promise.all([getMe(), getOrgTree()])
      .then(([user, org]) => {
        setMe(user);
        setTree(org.tree);
        setTotal(org.total);
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat data"),
      )
      .finally(() => setLoading(false));
  }, [router]);

  async function handleLogout() {
    await logout();
    router.replace("/login");
  }

  return (
    <div className="flex min-h-screen flex-1 flex-col bg-zinc-50 dark:bg-zinc-950">
      <header className="flex items-center justify-between border-b border-zinc-200 bg-white px-6 py-3 dark:border-zinc-800 dark:bg-zinc-900">
        <div className="flex items-center gap-5">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-700 text-sm font-bold text-white">
              e
            </div>
            <span className="font-semibold text-zinc-900 dark:text-zinc-50">
              eOffice Pro
            </span>
          </div>
          <nav className="flex flex-wrap gap-4 text-sm">
            {me?.roles.some((role) => ["admin", "creator", "secretary"].includes(role)) && (
              <a
                href="/compose"
                className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
              >
                Tulis Surat
              </a>
            )}
            <span className="font-semibold text-emerald-700 dark:text-emerald-400">
              Organisasi
            </span>
            {me?.roles.includes("admin") && (
              <>
                <a
                  href="/users"
                  className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
                >
                  Pengguna
                </a>
                <a
                  href="/letter-types"
                  className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
                >
                  Jenis Surat
                </a>
                <a
                  href="/letter-templates"
                  className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
                >
                  Template
                </a>
              </>
            )}
          </nav>
        </div>
        <div className="flex items-center gap-4 text-sm">
          {me && (
            <span className="text-zinc-600 dark:text-zinc-400">
              {me.full_name}
              <span className="ml-2 rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                {me.roles.join(", ")}
              </span>
            </span>
          )}
          <button
            onClick={handleLogout}
            className="rounded-lg border border-zinc-300 px-3 py-1.5 text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Keluar
          </button>
        </div>
      </header>

      <main className="mx-auto w-full max-w-4xl flex-1 px-6 py-8">
        <div className="mb-6">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-50">
            Struktur Organisasi
          </h1>
          <p className="text-sm text-zinc-500">
            FKK Group · {total} unit aktif
          </p>
        </div>

        {loading && <p className="text-sm text-zinc-500">Memuat…</p>}
        {error && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
            {error}
          </p>
        )}
        {!loading && !error && tree.length === 0 && (
          <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
            Belum ada unit organisasi. Admin dapat menambahkannya via API
            <code className="mx-1 font-mono">POST /org-units</code>
            (UI kelola menyusul).
          </p>
        )}
        <ul className="flex flex-col gap-2">
          {tree.map((unit) => (
            <UnitNode key={unit.id} unit={unit} depth={0} />
          ))}
        </ul>
      </main>
    </div>
  );
}
