"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import type { User } from "@/lib/api";
import Sidebar from "./Sidebar";
import ChangePasswordModal from "./ChangePasswordModal";
import NotificationBell from "./NotificationBell";
import {
  ChevronDownIcon,
  LockIcon,
  LogOutIcon,
  MenuIcon,
  PanelLeftIcon,
  SearchIcon,
  UserIcon,
} from "./icons";

function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
}

interface AppShellProps {
  me: User | null;
  title: string;
  onLogout: () => void;
  children: React.ReactNode;
}

// App shell bersama: sidebar navy + header (judul halaman, search,
// notifikasi, avatar). Semua halaman ber-autentikasi dirender di dalamnya.
export default function AppShell({ me, title, onLogout, children }: AppShellProps) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const [passwordModalOpen, setPasswordModalOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement>(null);

  const displayName = me?.full_name ?? "Admin";

  useEffect(() => {
    if (!userMenuOpen) return;
    function onMouseDown(event: MouseEvent) {
      if (!userMenuRef.current?.contains(event.target as Node)) {
        setUserMenuOpen(false);
      }
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setUserMenuOpen(false);
    }
    window.addEventListener("mousedown", onMouseDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("mousedown", onMouseDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [userMenuOpen]);

  const menuItemClass =
    "flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm text-zinc-700 transition hover:bg-zinc-50 dark:text-zinc-200 dark:hover:bg-zinc-800";

  return (
    <div className="flex min-h-screen flex-1 bg-zinc-50 dark:bg-zinc-950">
      <Sidebar
        me={me}
        collapsed={sidebarCollapsed}
        mobileOpen={mobileNavOpen}
        onCloseMobile={() => setMobileNavOpen(false)}
        onLogout={onLogout}
      />

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-zinc-200 bg-white/90 px-4 backdrop-blur sm:px-6 dark:border-zinc-800 dark:bg-zinc-900/90">
          <button
            onClick={() => setMobileNavOpen(true)}
            className="rounded-lg p-2 text-zinc-500 transition hover:bg-zinc-100 hover:text-zinc-900 lg:hidden dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
            aria-label="Buka menu navigasi"
          >
            <MenuIcon className="h-5 w-5" />
          </button>
          <button
            onClick={() => setSidebarCollapsed((current) => !current)}
            className="hidden rounded-lg p-2 text-zinc-500 transition hover:bg-zinc-100 hover:text-zinc-900 lg:inline-flex dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
            aria-label={sidebarCollapsed ? "Perluas sidebar" : "Ciutkan sidebar"}
          >
            <PanelLeftIcon className="h-5 w-5" />
          </button>

          <h1 className="text-base font-semibold text-zinc-900 sm:text-lg dark:text-zinc-50">
            {title}
          </h1>

          <div className="relative ml-2 hidden max-w-md flex-1 md:block">
            <SearchIcon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
            <input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Cari dokumen, disposisi, memo..."
              aria-label="Cari dokumen, disposisi, memo"
              className="w-full rounded-xl border border-zinc-200 bg-zinc-50 py-2 pl-9 pr-3 text-sm text-zinc-800 placeholder:text-zinc-400 transition focus:border-navy-400 focus:bg-white focus:outline-none focus:ring-2 focus:ring-navy-100 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-200 dark:focus:bg-zinc-900 dark:focus:ring-navy-800"
            />
          </div>

          <div className="ml-auto flex items-center gap-2 sm:gap-3">
            <NotificationBell />

            <div ref={userMenuRef} className="relative">
              <button
                onClick={() => setUserMenuOpen((current) => !current)}
                aria-haspopup="menu"
                aria-expanded={userMenuOpen}
                aria-label="Menu profil"
                className="flex items-center gap-2.5 rounded-xl py-1 pl-1 pr-2 transition hover:bg-zinc-100 sm:pl-1.5 dark:hover:bg-zinc-800"
              >
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gradient-to-br from-navy-700 to-navy-500 text-xs font-bold text-white shadow-sm">
                  {initialsOf(displayName) || "A"}
                </div>
                <div className="hidden text-left sm:block">
                  <p className="text-sm font-semibold leading-tight text-zinc-900 dark:text-zinc-100">
                    {displayName}
                  </p>
                  <p className="text-[11px] leading-tight text-zinc-400 dark:text-zinc-500">
                    {me?.roles.join(", ") || "Administrator"}
                  </p>
                </div>
                <ChevronDownIcon
                  className={`hidden h-4 w-4 text-zinc-400 transition-transform sm:block ${
                    userMenuOpen ? "rotate-180" : ""
                  }`}
                />
              </button>

              {userMenuOpen && (
                <div
                  role="menu"
                  className="absolute right-0 top-full z-40 mt-2 w-60 overflow-hidden rounded-xl border border-zinc-200 bg-white py-1 shadow-xl dark:border-zinc-700 dark:bg-zinc-900"
                >
                  <div className="border-b border-zinc-100 px-4 py-3 dark:border-zinc-800">
                    <p className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                      {displayName}
                    </p>
                    <p className="truncate text-xs text-zinc-500 dark:text-zinc-400">
                      {me?.email ?? ""}
                    </p>
                  </div>
                  <Link
                    href="/profile"
                    role="menuitem"
                    onClick={() => setUserMenuOpen(false)}
                    className={menuItemClass}
                  >
                    <UserIcon className="h-4 w-4 text-zinc-400" />
                    Profil Saya
                  </Link>
                  <button
                    role="menuitem"
                    onClick={() => {
                      setUserMenuOpen(false);
                      setPasswordModalOpen(true);
                    }}
                    className={menuItemClass}
                  >
                    <LockIcon className="h-4 w-4 text-zinc-400" />
                    Ubah Password
                  </button>
                  <div className="my-1 border-t border-zinc-100 dark:border-zinc-800" />
                  <button
                    role="menuitem"
                    onClick={() => {
                      setUserMenuOpen(false);
                      onLogout();
                    }}
                    className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm text-red-600 transition hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/40"
                  >
                    <LogOutIcon className="h-4 w-4" />
                    Keluar
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>

        {children}
      </div>

      <ChangePasswordModal
        open={passwordModalOpen}
        onClose={() => setPasswordModalOpen(false)}
        onLogout={onLogout}
      />
    </div>
  );
}
