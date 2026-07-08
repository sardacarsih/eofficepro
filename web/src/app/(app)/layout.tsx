"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { getAccessToken, getMe, logout, type User } from "@/lib/api";
import AppShell from "@/components/layout/AppShell";
import { CurrentUserProvider } from "@/components/layout/CurrentUserProvider";

const PAGE_TITLES: { prefix: string; title: string }[] = [
  { prefix: "/dashboard", title: "Dashboard" },
  { prefix: "/inbox", title: "Surat Masuk" },
  { prefix: "/compose", title: "Tulis Surat" },
  { prefix: "/approvals", title: "Approval" },
  { prefix: "/organization", title: "Organisasi" },
  { prefix: "/users", title: "Pengguna" },
  { prefix: "/letter-types", title: "Jenis Surat" },
  { prefix: "/letter-templates", title: "Template Surat" },
  { prefix: "/letters", title: "Detail Surat" },
  { prefix: "/profile", title: "Profil Saya" },
  { prefix: "/search", title: "Pencarian" },
];

// Layout bersama semua halaman ber-autentikasi: guard token, fetch profil
// sekali, lalu render sidebar + header lewat AppShell.
export default function AppLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const router = useRouter();
  const pathname = usePathname();
  const [me, setMe] = useState<User | null>(null);

  useEffect(() => {
    if (!getAccessToken()) {
      router.replace("/login");
      return;
    }
    getMe()
      .then(setMe)
      .catch(() => setMe(null));
  }, [router]);

  async function handleLogout() {
    await logout();
    router.replace("/login");
  }

  const title =
    PAGE_TITLES.find((page) => pathname.startsWith(page.prefix))?.title ??
    "eOffice Pro";

  return (
    <CurrentUserProvider me={me}>
      <AppShell me={me} title={title} onLogout={handleLogout}>
        {children}
      </AppShell>
    </CurrentUserProvider>
  );
}
