"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import {
  createApprovalCategory,
  deactivateApprovalCategory,
  listApprovalCategories,
  listCoordinationScopeRules,
  updateApprovalCategory,
  updateCoordinationScopeRule,
  type ApprovalCategory,
  type ApprovalMatrixFinalLevel,
  type CoordinationScopeRule,
} from "@/lib/api";

const FINAL_LEVELS: ApprovalMatrixFinalLevel[] = [
  "division_head", "sub_dept_head", "dept_head", "gm", "director",
  "vp_director", "president_director",
];

const SCOPE_LABEL: Record<CoordinationScopeRule["scope"], string> = {
  same_unit: "Satu unit",
  cross_department: "Lintas department",
  cross_biro: "Lintas biro",
  cross_directorate: "Lintas direktorat",
  corporate: "Korporat",
};

export default function ApprovalPoliciesPage() {
  const me = useCurrentUser();
  const router = useRouter();
  const [categories, setCategories] = useState<ApprovalCategory[]>([]);
  const [rules, setRules] = useState<CoordinationScopeRule[]>([]);
  const [editing, setEditing] = useState<ApprovalCategory | null>(null);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const [categoryData, ruleData] = await Promise.all([
      listApprovalCategories(), listCoordinationScopeRules(),
    ]);
    setCategories(categoryData.data);
    setRules(ruleData.data);
  }, []);

  useEffect(() => {
    if (me && !me.roles.includes("admin")) router.replace("/dashboard");
  }, [me, router]);

  useEffect(() => {
    void reload().catch((reason: unknown) =>
      setError(reason instanceof Error ? reason.message : "Gagal memuat kebijakan approval"),
    );
  }, [reload]);

  async function saveCategory(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const payload = { code: code.trim().toUpperCase(), name: name.trim() };
      if (editing) await updateApprovalCategory(editing.id, payload);
      else await createApprovalCategory(payload);
      setEditing(null); setCode(""); setName("");
      await reload();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Gagal menyimpan kategori");
    } finally { setBusy(false); }
  }

  async function removeCategory(category: ApprovalCategory) {
    setBusy(true); setError(null);
    try { await deactivateApprovalCategory(category.id); await reload(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Gagal menonaktifkan kategori"); }
    finally { setBusy(false); }
  }

  async function changeRule(scope: CoordinationScopeRule["scope"], level: ApprovalMatrixFinalLevel) {
    setBusy(true); setError(null);
    try { await updateCoordinationScopeRule(scope, level); await reload(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Gagal memperbarui aturan cakupan"); }
    finally { setBusy(false); }
  }

  return (
    <main className="mx-auto w-full max-w-6xl flex-1 space-y-6 px-6 py-8">
      <div>
        <h1 className="text-2xl font-semibold text-zinc-950 dark:text-zinc-50">Kebijakan Approval</h1>
        <p className="mt-1 text-sm text-zinc-500">Kelola kategori Persetujuan dan level akhir Koordinasi.</p>
      </div>
      {error && <p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{error}</p>}

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <h2 className="font-semibold">Kategori Persetujuan</h2>
        <form onSubmit={saveCategory} className="mt-4 grid gap-3 sm:grid-cols-[180px_1fr_auto]">
          <input required maxLength={30} value={code} onChange={(e) => setCode(e.target.value)} placeholder="Kode"
            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 dark:border-zinc-700 dark:bg-zinc-950" />
          <input required maxLength={100} value={name} onChange={(e) => setName(e.target.value)} placeholder="Nama kategori"
            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 dark:border-zinc-700 dark:bg-zinc-950" />
          <button disabled={busy} className="rounded-lg bg-navy-700 px-4 text-sm font-semibold text-white disabled:opacity-50">
            {editing ? "Perbarui" : "Tambah"}
          </button>
        </form>
        <div className="mt-4 divide-y divide-zinc-200 dark:divide-zinc-800">
          {categories.map((category) => (
            <div key={category.id} className="flex items-center gap-3 py-3">
              <span className="rounded bg-zinc-100 px-2 py-1 font-mono text-xs dark:bg-zinc-800">{category.code}</span>
              <span className="flex-1 text-sm">{category.name}</span>
              <button disabled={busy} onClick={() => { setEditing(category); setCode(category.code); setName(category.name); }} className="text-sm font-semibold text-navy-700 dark:text-sky-300">Edit</button>
              <button disabled={busy} onClick={() => void removeCategory(category)} className="text-sm font-semibold text-red-600">Nonaktifkan</button>
            </div>
          ))}
        </div>
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <h2 className="font-semibold">Cakupan Koordinasi</h2>
        <div className="mt-4 grid gap-3">
          {rules.map((rule) => (
            <label key={rule.scope} className="grid items-center gap-2 text-sm sm:grid-cols-[220px_1fr]">
              <span className="font-semibold">{SCOPE_LABEL[rule.scope]}</span>
              <select disabled={busy} value={rule.final_level} onChange={(e) => void changeRule(rule.scope, e.target.value as ApprovalMatrixFinalLevel)}
                className="h-10 rounded-lg border border-zinc-300 bg-white px-3 dark:border-zinc-700 dark:bg-zinc-950">
                {FINAL_LEVELS.map((level) => <option key={level} value={level}>{level.replaceAll("_", " ")}</option>)}
              </select>
            </label>
          ))}
        </div>
      </section>
    </main>
  );
}
