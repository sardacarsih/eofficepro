"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { logout } from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import ChangePasswordModal from "@/components/layout/ChangePasswordModal";
import { LockIcon } from "@/components/layout/icons";
import { POSITION_TYPE_LABEL } from "@/lib/position-types";

const ROLE_LABEL: Record<string, string> = {
  admin: "Admin",
  creator: "Creator",
  approver: "Approver",
  secretary: "Secretary",
  auditor: "Auditor",
};

const ASSIGNMENT_LABEL: Record<string, string> = {
  definitive: "Definitif",
  plt: "Plt",
  plh: "Plh",
};

function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
}

// Halaman profil pengguna login: identitas, role, jabatan aktif, dan akses
// cepat ke ubah password. Data diambil dari context (getMe di layout).
export default function ProfilePage() {
  const router = useRouter();
  const me = useCurrentUser();
  const [passwordModalOpen, setPasswordModalOpen] = useState(false);

  async function handleLogout() {
    await logout();
    router.replace("/login");
  }

  if (!me) {
    return (
      <main className="mx-auto w-full max-w-4xl flex-1 px-6 py-8">
        <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
          Memuat profil...
        </p>
      </main>
    );
  }

  const positions = me.positions ?? [];

  return (
    <main className="mx-auto w-full max-w-4xl flex-1 px-6 py-8">
      <section className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
        <div className="flex flex-wrap items-center gap-5 border-b border-zinc-100 px-6 py-6 dark:border-zinc-800">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-gradient-to-br from-navy-700 to-navy-500 text-xl font-bold text-white shadow-sm">
            {initialsOf(me.full_name) || "?"}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-50">
              {me.full_name}
            </h2>
            <p className="truncate text-sm text-zinc-500 dark:text-zinc-400">
              {me.email}
            </p>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {me.roles.map((role) => (
                <span
                  key={role}
                  className="rounded-full bg-navy-50 px-2.5 py-0.5 text-xs font-medium text-navy-700 dark:bg-navy-950 dark:text-navy-300"
                >
                  {ROLE_LABEL[role] ?? role}
                </span>
              ))}
            </div>
          </div>
          <button
            onClick={() => setPasswordModalOpen(true)}
            className="inline-flex items-center gap-2 rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800"
          >
            <LockIcon className="h-4 w-4" />
            Ubah Password
          </button>
        </div>

        <dl className="grid gap-x-8 gap-y-4 px-6 py-6 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-zinc-400 dark:text-zinc-500">
              NIK
            </dt>
            <dd className="mt-1 text-sm text-zinc-900 dark:text-zinc-100">
              {me.nik || "-"}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-zinc-400 dark:text-zinc-500">
              Email
            </dt>
            <dd className="mt-1 text-sm text-zinc-900 dark:text-zinc-100">
              {me.email}
            </dd>
          </div>
        </dl>
      </section>

      <section className="mt-6 overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
        <div className="border-b border-zinc-100 px-6 py-4 dark:border-zinc-800">
          <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
            Jabatan Aktif
          </h3>
        </div>
        {positions.length === 0 ? (
          <p className="px-6 py-8 text-center text-sm text-zinc-500">
            Belum ada jabatan aktif.
          </p>
        ) : (
          <ul className="divide-y divide-zinc-100 dark:divide-zinc-800">
            {positions.map((position) => (
              <li
                key={position.position_id}
                className="flex flex-wrap items-center justify-between gap-3 px-6 py-4"
              >
                <div>
                  <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                    {position.title}
                  </p>
                  <p className="text-xs text-zinc-500 dark:text-zinc-400">
                    {position.org_unit} ·{" "}
                    {POSITION_TYPE_LABEL[position.position_type] ??
                      position.position_type}
                  </p>
                </div>
                <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
                  {ASSIGNMENT_LABEL[position.assignment_type] ??
                    position.assignment_type}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <ChangePasswordModal
        open={passwordModalOpen}
        onClose={() => setPasswordModalOpen(false)}
        onLogout={handleLogout}
      />
    </main>
  );
}
