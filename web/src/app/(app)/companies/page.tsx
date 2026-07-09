"use client";

import Image from "next/image";
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  createCompany,
  deactivateCompany,
  deleteCompanyLogo,
  listCompanies,
  uploadCompanyLogo,
  updateCompany,
  type Company,
  type CompanyPayload,
} from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import {
  BuildingIcon,
  PlusIcon,
  UploadIcon,
  XIcon,
} from "@/components/layout/icons";
import CompanyFormDialog from "./_components/CompanyFormDialog";

const MAX_LOGO_SIZE = 2 * 1024 * 1024;
const ALLOWED_LOGO_TYPES = new Set(["image/png", "image/jpeg"]);

function LogoPreview({
  company,
  className,
}: {
  company: Company;
  className: string;
}) {
  if (company.logo_url && company.logo) {
    return (
      <Image
        src={company.logo_url}
        alt={`Logo ${company.name}`}
        width={company.logo.width}
        height={company.logo.height}
        unoptimized
        className={`${className} object-contain`}
      />
    );
  }

  return (
    <div
      className={`${className} flex items-center justify-center bg-navy-50 font-semibold text-navy-700 dark:bg-navy-950 dark:text-sky-300`}
      aria-label={`${company.name} belum memiliki logo`}
    >
      {company.code.slice(0, 2)}
    </div>
  );
}

export default function CompaniesPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const previewURLRef = useRef<string | null>(null);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [companyForm, setCompanyForm] = useState<{ company: Company | null } | null>(
    null,
  );
  const [selectedCompany, setSelectedCompany] = useState<Company | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewURL, setPreviewURL] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [formBusy, setFormBusy] = useState(false);
  const [actionID, setActionID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [modalError, setModalError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (me && !me.roles.includes("admin")) {
      router.replace("/organization");
    }
  }, [me, router]);

  useEffect(() => {
    listCompanies(true)
      .then((data) => setCompanies(data.companies))
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat perusahaan"),
      )
      .finally(() => setLoading(false));
  }, []);

  useEffect(
    () => () => {
      if (previewURLRef.current) URL.revokeObjectURL(previewURLRef.current);
    },
    [],
  );

  const activeCount = useMemo(
    () => companies.filter((company) => company.is_active).length,
    [companies],
  );
  const logoCount = useMemo(
    () => companies.filter((company) => company.has_logo).length,
    [companies],
  );

  async function reload() {
    const data = await listCompanies(true);
    setCompanies(data.companies);
  }

  function clearSelectedFile() {
    if (previewURLRef.current) {
      URL.revokeObjectURL(previewURLRef.current);
      previewURLRef.current = null;
    }
    setPreviewURL(null);
    setSelectedFile(null);
  }

  function openUpload(company: Company) {
    clearSelectedFile();
    setCompanyForm(null);
    setSelectedCompany(company);
    setModalError(null);
  }

  function openCreate() {
    setSelectedCompany(null);
    setCompanyForm({ company: null });
    setFormError(null);
  }

  function openEdit(company: Company) {
    setSelectedCompany(null);
    setCompanyForm({ company });
    setFormError(null);
  }

  function closeCompanyForm() {
    if (formBusy) return;
    setCompanyForm(null);
    setFormError(null);
  }

  async function saveCompany(payload: CompanyPayload) {
    if (!companyForm) return;
    setFormBusy(true);
    setFormError(null);
    try {
      if (companyForm.company) {
        await updateCompany(companyForm.company.id, payload);
      } else {
        await createCompany(payload);
      }
      await reload();
      setCompanyForm(null);
    } catch (err) {
      setFormError(
        err instanceof Error ? err.message : "Gagal menyimpan perusahaan",
      );
    } finally {
      setFormBusy(false);
    }
  }

  function closeUpload() {
    if (busy) return;
    clearSelectedFile();
    setSelectedCompany(null);
    setModalError(null);
  }

  function chooseFile(file: File | undefined) {
    clearSelectedFile();
    setModalError(null);
    if (!file) return;
    if (!ALLOWED_LOGO_TYPES.has(file.type)) {
      setModalError("Logo harus berupa PNG atau JPEG.");
      return;
    }
    if (file.size <= 0 || file.size > MAX_LOGO_SIZE) {
      setModalError("Ukuran logo maksimal 2 MB.");
      return;
    }

    const localURL = URL.createObjectURL(file);
    previewURLRef.current = localURL;
    setPreviewURL(localURL);
    setSelectedFile(file);
  }

  async function handleUpload(event: React.FormEvent) {
    event.preventDefault();
    if (!selectedCompany || !selectedFile) return;

    setBusy(true);
    setModalError(null);
    try {
      await uploadCompanyLogo(selectedCompany.id, selectedFile);
      await reload();
      clearSelectedFile();
      setSelectedCompany(null);
    } catch (err) {
      setModalError(
        err instanceof Error ? err.message : "Gagal mengunggah logo perusahaan",
      );
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete(company: Company) {
    if (!confirm(`Hapus logo ${company.name}? Kop surat berikutnya akan kembali ke teks.`)) {
      return;
    }
    setActionID(company.id);
    setError(null);
    try {
      await deleteCompanyLogo(company.id);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menghapus logo perusahaan");
    } finally {
      setActionID(null);
    }
  }

  async function handleDeactivate(company: Company) {
    if (
      !confirm(
        `Nonaktifkan ${company.name}? Company tidak akan tersedia untuk surat baru.`,
      )
    ) {
      return;
    }
    setActionID(company.id);
    setError(null);
    try {
      await deactivateCompany(company.id);
      await reload();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal menonaktifkan perusahaan",
      );
    } finally {
      setActionID(null);
    }
  }

  async function handleReactivate(company: Company) {
    setActionID(company.id);
    setError(null);
    try {
      await updateCompany(company.id, {
        code: company.code,
        name: company.name,
        is_active: true,
      });
      await reload();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Gagal mengaktifkan perusahaan",
      );
    } finally {
      setActionID(null);
    }
  }

  return (
    <>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-sky-700 dark:text-sky-300">
              <span className="h-px w-7 bg-sky-500" />
              Identitas korporat
            </div>
            <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
              Logo Perusahaan
            </h1>
            <p className="mt-1 max-w-2xl text-sm text-zinc-500">
              Kelola logo yang digunakan pada kop preview dan PDF final setiap perusahaan.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <div className="border-r border-zinc-200 px-4 py-3 dark:border-zinc-800">
                <p className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
                  Company aktif
                </p>
                <p className="mt-0.5 text-lg font-semibold text-zinc-950 dark:text-zinc-50">
                  {activeCount}
                </p>
              </div>
              <div className="px-4 py-3">
                <p className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
                  Logo tersedia
                </p>
                <p className="mt-0.5 text-lg font-semibold text-emerald-700 dark:text-emerald-400">
                  {logoCount}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={openCreate}
              className="flex items-center gap-2 rounded-lg bg-navy-700 px-3.5 py-2.5 text-sm font-semibold text-white transition hover:bg-navy-800"
            >
              <PlusIcon className="h-4 w-4" />
              Tambah
            </button>
          </div>
        </div>

        {error && (
          <p
            role="alert"
            className="mb-4 rounded-lg border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300"
          >
            {error}
          </p>
        )}

        <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <div className="border-b border-zinc-200 bg-zinc-50/80 px-5 py-3 dark:border-zinc-800 dark:bg-zinc-900">
            <p className="text-xs text-zinc-500">
              PNG atau JPEG · maksimal 2 MB · minimum 128 × 128 piksel
            </p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead className="border-b border-zinc-200 text-xs uppercase tracking-wide text-zinc-500 dark:border-zinc-800">
                <tr>
                  <th className="px-5 py-3">Logo</th>
                  <th className="px-4 py-3">Perusahaan</th>
                  <th className="px-4 py-3">File</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-5 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={5} className="px-5 py-10 text-center text-zinc-500">
                      Memuat perusahaan...
                    </td>
                  </tr>
                )}
                {!loading && companies.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-5 py-10 text-center text-zinc-500">
                      Belum ada perusahaan.
                    </td>
                  </tr>
                )}
                {!loading &&
                  companies.map((company) => {
                    const actionBusy = actionID === company.id;
                    return (
                      <tr
                        key={company.id}
                        className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/70"
                      >
                        <td className="px-5 py-3">
                          <div className="flex h-14 w-24 items-center justify-center rounded-lg border border-zinc-200 bg-white p-2 dark:border-zinc-700 dark:bg-zinc-950">
                            <LogoPreview company={company} className="h-full w-full rounded" />
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <p className="font-semibold text-zinc-950 dark:text-zinc-50">
                            {company.name}
                          </p>
                          <p className="mt-0.5 font-mono text-xs text-zinc-500">
                            {company.code}
                          </p>
                        </td>
                        <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">
                          {company.logo ? (
                            <>
                              <p className="max-w-52 truncate text-xs font-medium">
                                {company.logo.file_name}
                              </p>
                              <p className="mt-1 text-[11px] text-zinc-400">
                                {company.logo.width} × {company.logo.height} px
                              </p>
                            </>
                          ) : (
                            <span className="text-xs italic text-zinc-400">
                              Kop teks
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                              company.is_active
                                ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                                : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                            }`}
                          >
                            {company.is_active ? "Aktif" : "Nonaktif"}
                          </span>
                        </td>
                        <td className="px-5 py-3 text-right">
                          <div className="flex justify-end gap-3">
                            <button
                              type="button"
                              onClick={() => openUpload(company)}
                              className="text-xs font-semibold text-sky-700 hover:underline dark:text-sky-300"
                            >
                              {company.has_logo ? "Ganti logo" : "Unggah logo"}
                            </button>
                            <button
                              type="button"
                              onClick={() => openEdit(company)}
                              className="text-xs font-semibold text-zinc-600 hover:text-zinc-950 hover:underline dark:text-zinc-300 dark:hover:text-white"
                            >
                              Edit
                            </button>
                            {company.has_logo && (
                              <button
                                type="button"
                                onClick={() => handleDelete(company)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-red-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400"
                              >
                                Hapus logo
                              </button>
                            )}
                            {company.is_active ? (
                              <button
                                type="button"
                                onClick={() => handleDeactivate(company)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-red-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400"
                              >
                                {actionBusy ? "Memproses..." : "Nonaktifkan"}
                              </button>
                            ) : (
                              <button
                                type="button"
                                onClick={() => handleReactivate(company)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-emerald-700 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-emerald-400"
                              >
                                {actionBusy ? "Memproses..." : "Aktifkan"}
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

      {companyForm && (
        <CompanyFormDialog
          key={companyForm.company?.id ?? "new-company"}
          company={companyForm.company}
          busy={formBusy}
          error={formError}
          onClose={closeCompanyForm}
          onSave={saveCompany}
        />
      )}

      {selectedCompany && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="company-logo-dialog-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/50 px-4 py-6 backdrop-blur-[2px]"
        >
          <form
            onSubmit={handleUpload}
            className="w-full max-w-lg overflow-hidden rounded-2xl border border-white/10 bg-white shadow-2xl dark:bg-zinc-900"
          >
            <div className="flex items-start justify-between border-b border-zinc-200 px-6 py-5 dark:border-zinc-800">
              <div>
                <p className="text-xs font-semibold uppercase tracking-wider text-sky-700 dark:text-sky-300">
                  {selectedCompany.code}
                </p>
                <h2
                  id="company-logo-dialog-title"
                  className="mt-1 text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  {selectedCompany.has_logo ? "Ganti logo perusahaan" : "Unggah logo perusahaan"}
                </h2>
              </div>
              <button
                type="button"
                onClick={closeUpload}
                disabled={busy}
                aria-label="Tutup dialog"
                className="rounded-lg p-1.5 text-zinc-400 transition hover:bg-zinc-100 hover:text-zinc-700 disabled:opacity-50 dark:hover:bg-zinc-800 dark:hover:text-zinc-200"
              >
                <XIcon className="h-5 w-5" />
              </button>
            </div>

            <div className="space-y-4 px-6 py-5">
              {modalError && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  {modalError}
                </p>
              )}

              <label
                htmlFor="company-logo-file"
                className="group flex cursor-pointer flex-col items-center rounded-xl border border-dashed border-zinc-300 bg-zinc-50 px-5 py-7 text-center transition hover:border-sky-400 hover:bg-sky-50/50 dark:border-zinc-700 dark:bg-zinc-950 dark:hover:border-sky-700 dark:hover:bg-sky-950/30"
              >
                {previewURL ? (
                  <div className="mb-4 flex h-28 w-52 items-center justify-center rounded-lg border border-zinc-200 bg-white p-3 shadow-sm dark:border-zinc-700">
                    <Image
                      src={previewURL}
                      alt="Pratinjau logo yang dipilih"
                      width={208}
                      height={112}
                      unoptimized
                      className="h-full w-full object-contain"
                    />
                  </div>
                ) : (
                  <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-full bg-sky-100 text-sky-700 transition group-hover:scale-105 dark:bg-sky-950 dark:text-sky-300">
                    <UploadIcon className="h-5 w-5" />
                  </div>
                )}
                <span className="text-sm font-semibold text-zinc-800 dark:text-zinc-100">
                  {selectedFile ? selectedFile.name : "Pilih file logo"}
                </span>
                <span className="mt-1 text-xs text-zinc-500">
                  PNG atau JPEG, maksimal 2 MB
                </span>
                <input
                  id="company-logo-file"
                  type="file"
                  accept="image/png,image/jpeg"
                  className="sr-only"
                  onChange={(event) => chooseFile(event.target.files?.[0])}
                />
              </label>

              <div className="flex gap-3 rounded-lg border border-sky-100 bg-sky-50 px-3 py-3 text-xs text-sky-900 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-200">
                <BuildingIcon className="mt-0.5 h-4 w-4 shrink-0" />
                <p>
                  Logo akan ditempatkan di kiri kop. Nama perusahaan tetap berada
                  di tengah dan rasio gambar tidak akan diubah.
                </p>
              </div>
            </div>

            <div className="flex justify-end gap-3 border-t border-zinc-200 bg-zinc-50 px-6 py-4 dark:border-zinc-800 dark:bg-zinc-900">
              <button
                type="button"
                onClick={closeUpload}
                disabled={busy}
                className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-white disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={!selectedFile || busy}
                className="rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {busy ? "Mengunggah..." : "Simpan logo"}
              </button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}
