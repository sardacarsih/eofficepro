"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  createUser,
  deactivateUser,
  downloadImportTemplate,
  getAccessToken,
  getMe,
  importUsers,
  listUsers,
  logout,
  updateUser,
  type ImportResult,
  type User,
  type UserPayload,
  type UserRow,
} from "@/lib/api";

const ROLE_OPTIONS = [
  { value: "admin", label: "Admin" },
  { value: "creator", label: "Creator" },
  { value: "approver", label: "Approver" },
  { value: "secretary", label: "Secretary" },
  { value: "auditor", label: "Auditor" },
] as const;

const STATUS_LABEL: Record<UserPayload["status"], string> = {
  active: "Aktif",
  inactive: "Nonaktif",
  locked: "Terkunci",
};

const STATUS_STYLE: Record<UserPayload["status"], string> = {
  active: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  inactive: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300",
  locked: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
};

interface UserFormState {
  nik: string;
  email: string;
  full_name: string;
  status: UserPayload["status"];
  roles: string[];
  password: string;
}

function emptyForm(): UserFormState {
  return {
    nik: "",
    email: "",
    full_name: "",
    status: "active",
    roles: ["creator"],
    password: "",
  };
}

function userToForm(user: UserRow): UserFormState {
  return {
    nik: user.nik,
    email: user.email,
    full_name: user.full_name,
    status: user.status as UserPayload["status"],
    roles: user.roles.length > 0 ? user.roles : ["creator"],
    password: "",
  };
}

function compactPayload(form: UserFormState): UserPayload {
  return {
    nik: form.nik.trim(),
    email: form.email.trim(),
    full_name: form.full_name.trim(),
    status: form.status,
    roles: form.roles,
    ...(form.password.trim() ? { password: form.password } : {}),
  };
}

export default function UsersPage() {
  const router = useRouter();
  const fileRef = useRef<HTMLInputElement>(null);
  const [me, setMe] = useState<User | null>(null);
  const [users, setUsers] = useState<UserRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [modalError, setModalError] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [actionUserID, setActionUserID] = useState<string | null>(null);
  const [editing, setEditing] = useState<UserRow | null>(null);
  const [form, setForm] = useState<UserFormState | null>(null);
  const modalOpen = editing !== null || form !== null;

  async function reload() {
    const data = await listUsers();
    setUsers(data.users);
  }

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    Promise.all([getMe(), listUsers()])
      .then(([user, data]) => {
        setMe(user);
        setUsers(data.users);
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat pengguna"),
      )
      .finally(() => setLoading(false));
  }, [router]);

  const activeCount = useMemo(
    () => users.filter((user) => user.status === "active").length,
    [users],
  );

  function openCreate() {
    setEditing(null);
    setForm(emptyForm());
    setModalError(null);
  }

  function openEdit(user: UserRow) {
    setEditing(user);
    setForm(userToForm(user));
    setModalError(null);
  }

  function closeModal() {
    if (busy) return;
    setEditing(null);
    setForm(null);
    setModalError(null);
  }

  function toggleRole(role: string) {
    setForm((current) => {
      if (!current) return current;
      const exists = current.roles.includes(role);
      return {
        ...current,
        roles: exists
          ? current.roles.filter((item) => item !== role)
          : [...current.roles, role],
      };
    });
  }

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setBusy(true);
    setError(null);
    setImportResult(null);
    try {
      setImportResult(await importUsers(file));
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Import gagal");
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setModalError(null);
    try {
      if (form.roles.length === 0) {
        throw new Error("Pilih minimal satu role");
      }
      if (!editing && form.password.trim().length < 10) {
        throw new Error("Password awal minimal 10 karakter");
      }
      const payload = compactPayload(form);
      if (editing) {
        await updateUser(editing.id, payload);
      } else {
        await createUser(payload);
      }
      await reload();
      setEditing(null);
      setForm(null);
    } catch (err) {
      setModalError(err instanceof Error ? err.message : "Gagal menyimpan pengguna");
    } finally {
      setBusy(false);
    }
  }

  async function handleDeactivate(user: UserRow) {
    if (!confirm(`Nonaktifkan ${user.full_name} (${user.nik})?`)) return;
    setActionUserID(user.id);
    setError(null);
    try {
      await deactivateUser(user.id);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menonaktifkan");
    } finally {
      setActionUserID(null);
    }
  }

  async function handleReactivate(user: UserRow) {
    setActionUserID(user.id);
    setError(null);
    try {
      await updateUser(user.id, {
        nik: user.nik,
        email: user.email,
        full_name: user.full_name,
        status: "active",
        roles: user.roles,
      });
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengaktifkan");
    } finally {
      setActionUserID(null);
    }
  }

  async function handleLogout() {
    await logout();
    router.replace("/login");
  }

  return (
    <div className="flex min-h-screen flex-1 flex-col bg-[#f5f7fa] text-[#172033] dark:bg-zinc-950 dark:text-zinc-50">
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
            <Link
              href="/compose"
              className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
            >
              Tulis Surat
            </Link>
            <Link
              href="/organization"
              className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
            >
              Organisasi
            </Link>
            <span className="font-semibold text-emerald-700 dark:text-emerald-400">
              Pengguna
            </span>
            <Link
              href="/letter-types"
              className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
            >
              Jenis Surat
            </Link>
            <Link
              href="/letter-templates"
              className="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
            >
              Template
            </Link>
          </nav>
        </div>
        <div className="flex items-center gap-4 text-sm">
          {me && (
            <span className="hidden text-zinc-600 dark:text-zinc-400 sm:inline">
              {me.full_name}
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

      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
              Pengguna
            </h1>
            <p className="text-sm text-zinc-500">
              {activeCount} aktif dari {users.length} akun terdaftar
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() =>
                downloadImportTemplate().catch(() =>
                  setError("Gagal mengunduh template"),
                )
              }
              className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Unduh Template
            </button>
            <label className="cursor-pointer rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800">
              {busy ? "Memproses..." : "Import Excel"}
              <input
                ref={fileRef}
                type="file"
                accept=".xlsx"
                onChange={handleImport}
                disabled={busy}
                className="hidden"
              />
            </label>
            <button
              onClick={openCreate}
              className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800"
            >
              Tambah Pengguna
            </button>
          </div>
        </div>

        {error && (
          <p
            role="alert"
            className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
          >
            {error}
          </p>
        )}

        {importResult && (
          <div className="mb-4 rounded-lg border border-zinc-200 bg-white px-4 py-3 text-sm dark:border-zinc-800 dark:bg-zinc-900">
            <p className="font-medium text-zinc-900 dark:text-zinc-100">
              Import selesai: {importResult.imported} berhasil,{" "}
              {importResult.failed} gagal.
            </p>
            {importResult.errors.length > 0 && (
              <ul className="mt-2 list-inside list-disc text-red-700 dark:text-red-300">
                {importResult.errors.map((item) => (
                  <li key={item.row}>
                    Baris {item.row}: {item.error}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[900px] text-left text-sm">
              <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
                <tr>
                  <th className="px-4 py-3">NIK</th>
                  <th className="px-4 py-3">Nama</th>
                  <th className="px-4 py-3">Email</th>
                  <th className="px-4 py-3">Role</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-zinc-500">
                      Memuat pengguna...
                    </td>
                  </tr>
                )}
                {!loading && users.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-zinc-500">
                      Belum ada pengguna.
                    </td>
                  </tr>
                )}
                {!loading &&
                  users.map((user) => {
                    const status = user.status as UserPayload["status"];
                    const actionBusy = actionUserID === user.id;

                    return (
                      <tr
                        key={user.id}
                        className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60"
                      >
                        <td className="px-4 py-3 font-mono text-xs text-zinc-600 dark:text-zinc-300">
                          {user.nik}
                        </td>
                        <td className="px-4 py-3 font-medium text-zinc-900 dark:text-zinc-100">
                          {user.full_name}
                          {user.id === me?.id && (
                            <span className="ml-2 rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-700 dark:bg-sky-950 dark:text-sky-300">
                              Anda
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">
                          {user.email}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex flex-wrap gap-1">
                            {user.roles.length > 0 ? (
                              user.roles.map((role) => (
                                <span
                                  key={role}
                                  className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
                                >
                                  {role}
                                </span>
                              ))
                            ) : (
                              <span className="text-zinc-400">-</span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${STATUS_STYLE[status] ?? STATUS_STYLE.inactive}`}
                          >
                            {STATUS_LABEL[status] ?? user.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex justify-end gap-3">
                            <button
                              onClick={() => openEdit(user)}
                              className="text-xs font-semibold text-zinc-600 hover:text-zinc-950 hover:underline dark:text-zinc-300 dark:hover:text-white"
                            >
                              Edit
                            </button>
                            {user.status === "active" ? (
                              <button
                                onClick={() => handleDeactivate(user)}
                                disabled={actionBusy}
                                className="text-xs font-semibold text-red-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400"
                              >
                                {actionBusy ? "Memproses" : "Nonaktifkan"}
                              </button>
                            ) : (
                              <button
                                onClick={() => handleReactivate(user)}
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
        </div>
      </main>

      {modalOpen && form && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="user-form-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <form
            onSubmit={handleSubmit}
            className="max-h-full w-full max-w-2xl overflow-y-auto rounded-xl bg-white shadow-2xl dark:bg-zinc-900"
          >
            <div className="flex items-start justify-between border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <div>
                <h2
                  id="user-form-title"
                  className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  {editing ? "Edit Pengguna" : "Tambah Pengguna"}
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  Kelola identitas, role, status, dan password awal.
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

            <div className="grid gap-4 px-6 py-5 sm:grid-cols-2">
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                NIK
                <input
                  value={form.nik}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, nik: e.target.value } : current,
                    )
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Email
                <input
                  type="email"
                  value={form.email}
                  onChange={(e) =>
                    setForm((current) =>
                      current ? { ...current, email: e.target.value } : current,
                    )
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 sm:col-span-2">
                Nama Lengkap
                <input
                  value={form.full_name}
                  onChange={(e) =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            full_name: e.target.value,
                          }
                        : current,
                    )
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Status
                <select
                  value={form.status}
                  onChange={(e) =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            status: e.target.value as UserPayload["status"],
                          }
                        : current,
                    )
                  }
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="active">Aktif</option>
                  <option value="inactive">Nonaktif</option>
                  <option value="locked">Terkunci</option>
                </select>
              </label>
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                {editing ? "Reset Password" : "Password Awal"}
                <input
                  type="password"
                  value={form.password}
                  onChange={(e) =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            password: e.target.value,
                          }
                        : current,
                    )
                  }
                  required={!editing}
                  minLength={editing ? undefined : 10}
                  placeholder={editing ? "Kosongkan jika tidak diganti" : "Minimal 10 karakter"}
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>

              <fieldset className="sm:col-span-2">
                <legend className="mb-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Role
                </legend>
                <div className="flex flex-wrap gap-2">
                  {ROLE_OPTIONS.map((role) => {
                    const checked = form.roles.includes(role.value);

                    return (
                      <label
                        key={role.value}
                        className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition ${
                          checked
                            ? "border-emerald-600 bg-emerald-50 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200"
                            : "border-zinc-300 bg-white text-zinc-600 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-300 dark:hover:bg-zinc-800"
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleRole(role.value)}
                          className="h-4 w-4 rounded border-zinc-300 text-emerald-700 focus:ring-emerald-600"
                        />
                        {role.label}
                      </label>
                    );
                  })}
                </div>
              </fieldset>

              {modalError && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300 sm:col-span-2"
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
                className="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busy ? "Menyimpan..." : "Simpan"}
              </button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
