"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  activateLetterTemplate,
  createLetterTemplate,
  deactivateLetterTemplate,
  listAllCompanies,
  listAllLetterTypes,
  listLetterTemplates,
  updateLetterTemplate,
  type Company,
  type LetterTemplate,
  type LetterTemplatePayload,
  type LetterType,
  type PageMeta,
} from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import Pagination from "@/components/Pagination";

const DEFAULT_LAYOUT = {
  page: { size: "A4", margin_mm: { top: 24, right: 22, bottom: 22, left: 22 } },
  letterhead: { logo: true, company_name: true, address: true },
  signature: { align: "right", qr: true },
  placeholders: ["nomor", "tanggal", "perihal", "isi", "penandatangan"],
};

const DEFAULT_BODY = `<p>{{isi_surat}}</p>
<p>Demikian disampaikan, atas perhatian dan kerja samanya diucapkan terima kasih.</p>`;

interface TemplateFormState {
  letter_type_id: string;
  company_id: string;
  version: string;
  layout_config: string;
  body_skeleton: string;
  is_active: boolean;
}

function emptyForm(letterTypes: LetterType[], companies: Company[]): TemplateFormState {
  return {
    letter_type_id: letterTypes[0]?.id ?? "",
    company_id: companies[0]?.id ?? "",
    version: "",
    layout_config: JSON.stringify(DEFAULT_LAYOUT, null, 2),
    body_skeleton: DEFAULT_BODY,
    is_active: true,
  };
}

function templateToForm(template: LetterTemplate): TemplateFormState {
  return {
    letter_type_id: template.letter_type_id,
    company_id: template.company_id,
    version: String(template.version),
    layout_config: JSON.stringify(template.layout_config, null, 2),
    body_skeleton: template.body_skeleton,
    is_active: template.is_active,
  };
}

function parseLayoutConfig(raw: string): Record<string, unknown> {
  const parsed: unknown = JSON.parse(raw);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("Layout config wajib berupa JSON object");
  }
  return parsed as Record<string, unknown>;
}

function compactPayload(form: TemplateFormState): LetterTemplatePayload {
  const version = form.version.trim() ? Number(form.version) : undefined;
  if (version !== undefined && (!Number.isInteger(version) || version < 1)) {
    throw new Error("Versi wajib berupa angka positif");
  }
  if (!form.letter_type_id) throw new Error("Pilih jenis surat");
  if (!form.company_id) throw new Error("Pilih perusahaan");
  if (!form.body_skeleton.trim()) throw new Error("Body skeleton wajib diisi");

  return {
    letter_type_id: form.letter_type_id,
    company_id: form.company_id,
    ...(version === undefined ? {} : { version }),
    layout_config: parseLayoutConfig(form.layout_config),
    body_skeleton: form.body_skeleton.trim(),
    is_active: form.is_active,
  };
}

export default function LetterTemplatesPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const [templates, setTemplates] = useState<LetterTemplate[]>([]);
  const [page, setPage] = useState(1);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [letterTypes, setLetterTypes] = useState<LetterType[]>([]);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [modalError, setModalError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [actionID, setActionID] = useState<string | null>(null);
  const [editing, setEditing] = useState<LetterTemplate | null>(null);
  const [form, setForm] = useState<TemplateFormState | null>(null);
  const modalOpen = editing !== null || form !== null;

  async function reload() {
    const data = await listLetterTemplates({ includeInactive: true, page });
    setTemplates(data.data);
    setMeta(data.meta);
  }

  // Halaman khusus admin — alihkan role lain setelah profil termuat.
  useEffect(() => {
		if (me && !me.capabilities?.is_super_admin && !me.company_roles?.some((role) => role.role_code === "admin")) {
      router.replace("/organization");
    }
  }, [me, router]);

  useEffect(() => {
    queueMicrotask(() => setLoading(true));
    Promise.all([
      listLetterTemplates({ includeInactive: true, page }),
      listAllLetterTypes(false),
      listAllCompanies(),
    ])
      .then(([templateData, typeData, companyData]) => {
        setTemplates(templateData.data);
        setMeta(templateData.meta);
        setLetterTypes(typeData.data);
        setCompanies(companyData.data);
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat template surat"),
      )
      .finally(() => setLoading(false));
  }, [page]);

  const activeCount = useMemo(
    () => templates.filter((template) => template.is_active).length,
    [templates],
  );

  function openCreate() {
    setEditing(null);
    setForm(emptyForm(letterTypes, companies));
    setModalError(null);
  }

  function openEdit(template: LetterTemplate) {
    setEditing(template);
    setForm(templateToForm(template));
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
      if (editing) {
        await updateLetterTemplate(editing.id, payload);
      } else {
        await createLetterTemplate(payload);
      }
      await reload();
      setEditing(null);
      setForm(null);
    } catch (err) {
      setModalError(err instanceof Error ? err.message : "Gagal menyimpan template");
    } finally {
      setBusy(false);
    }
  }

  async function handleActivate(template: LetterTemplate) {
    setActionID(template.id);
    setError(null);
    try {
      await activateLetterTemplate(template.id);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengaktifkan template");
    } finally {
      setActionID(null);
    }
  }

  async function handleDeactivate(template: LetterTemplate) {
    if (!confirm(`Nonaktifkan template ${template.letter_type_code} v${template.version}?`)) {
      return;
    }
    setActionID(template.id);
    setError(null);
    try {
      await deactivateLetterTemplate(template.id);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menonaktifkan template");
    } finally {
      setActionID(null);
    }
  }

  return (
    <>
      <main className="mx-auto w-full max-w-7xl flex-1 px-6 py-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
              Template Surat
            </h1>
            <p className="text-sm text-zinc-500">
              {activeCount} template aktif dari {templates.length} versi
            </p>
          </div>
          <button
            onClick={openCreate}
            disabled={letterTypes.length === 0 || companies.length === 0}
            className="rounded-lg bg-navy-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Tambah Template
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
            <table className="w-full min-w-[980px] text-left text-sm">
              <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
                <tr>
                  <th className="px-4 py-3">Jenis</th>
                  <th className="px-4 py-3">Perusahaan</th>
                  <th className="px-4 py-3">Versi</th>
                  <th className="px-4 py-3">Placeholder</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-zinc-500">
                      Memuat template surat...
                    </td>
                  </tr>
                )}
                {!loading && templates.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-zinc-500">
                      Belum ada template surat.
                    </td>
                  </tr>
                )}
                {!loading &&
                  templates.map((template) => {
                    const placeholders = template.layout_config.placeholders;
                    const placeholderText = Array.isArray(placeholders)
                      ? placeholders.join(", ")
                      : "-";
                    const actionBusy = actionID === template.id;

                    return (
                      <tr
                        key={template.id}
                        className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60"
                      >
                        <td className="px-4 py-3">
                          <div className="font-medium text-zinc-900 dark:text-zinc-100">
                            {template.letter_type_name}
                          </div>
                          <div className="font-mono text-xs text-zinc-400">
                            {template.letter_type_code}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="font-medium text-zinc-900 dark:text-zinc-100">
                            {template.company_name}
                          </div>
                          <div className="font-mono text-xs text-zinc-400">
                            {template.company_code}
                          </div>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs font-semibold text-zinc-700 dark:text-zinc-300">
                          v{template.version}
                        </td>
                        <td className="max-w-sm truncate px-4 py-3 text-zinc-600 dark:text-zinc-400">
                          {placeholderText}
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                              template.is_active
                                ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                                : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                            }`}
                          >
                            {template.is_active ? "Aktif" : "Nonaktif"}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex justify-end gap-3">
                            <button
                              onClick={() => openEdit(template)}
                              className="text-xs font-semibold text-zinc-600 hover:text-zinc-950 hover:underline dark:text-zinc-300 dark:hover:text-white"
                            >
                              Edit
                            </button>
                            {template.is_active ? (
                              <button
                                onClick={() => handleDeactivate(template)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-red-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400"
                              >
                                {actionBusy ? "Memproses" : "Nonaktifkan"}
                              </button>
                            ) : (
                              <button
                                onClick={() => handleActivate(template)}
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
          <div className="px-4">
            <Pagination
              page={page}
              totalPages={meta?.total_pages ?? 1}
              onPageChange={setPage}
              disabled={loading}
            />
          </div>
        </div>
      </main>

      {modalOpen && form && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="template-form-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <form
            onSubmit={handleSubmit}
            className="max-h-full w-full max-w-4xl overflow-y-auto rounded-xl bg-white shadow-2xl dark:bg-zinc-900"
          >
            <div className="flex items-start justify-between border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <div>
                <h2
                  id="template-form-title"
                  className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  {editing ? "Edit Template" : "Tambah Template"}
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  Simpan layout JSON dan skeleton HTML untuk composer surat.
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

            <div className="grid gap-4 px-6 py-5 md:grid-cols-4">
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 md:col-span-2">
                Jenis Surat
                <select
                  value={form.letter_type_id}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, letter_type_id: e.target.value } : current,
                    )
                  }
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  {letterTypes.map((letterType) => (
                    <option key={letterType.id} value={letterType.id}>
                      {letterType.code} - {letterType.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Perusahaan
                <select
                  value={form.company_id}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, company_id: e.target.value } : current,
                    )
                  }
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  {companies.map((company) => (
                    <option key={company.id} value={company.id}>
                      {company.code}
                    </option>
                  ))}
                </select>
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Versi
                <input
                  type="number"
                  min={1}
                  placeholder={editing ? undefined : "Auto"}
                  value={form.version}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, version: e.target.value } : current,
                    )
                  }
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 md:col-span-2">
                Layout Config JSON
                <textarea
                  value={form.layout_config}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, layout_config: e.target.value } : current,
                    )
                  }
                  spellCheck={false}
                  className="min-h-72 resize-y rounded-lg border border-zinc-300 bg-white px-3 py-2 font-mono text-xs font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 md:col-span-2">
                Body Skeleton HTML
                <textarea
                  value={form.body_skeleton}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, body_skeleton: e.target.value } : current,
                    )
                  }
                  className="min-h-72 resize-y rounded-lg border border-zinc-300 bg-white px-3 py-2 font-mono text-xs font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex items-center gap-3 rounded-lg border border-zinc-300 bg-white px-3 py-2.5 text-sm font-semibold text-zinc-800 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-200">
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

              {modalError && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300 md:col-span-4"
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
