"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { User } from "@/lib/api";
import {
  BuildingIcon,
  CheckCircleIcon,
  LayersIcon,
  LayoutDashboardIcon,
  LogOutIcon,
  PenSquareIcon,
  TagIcon,
  UsersIcon,
  XIcon,
  type IconProps,
} from "./icons";

interface NavItem {
  href: string;
  label: string;
  icon: (props: IconProps) => React.ReactElement;
  /** null = tampil untuk semua role (mengikuti aturan navbar existing). */
  roles: string[] | null;
}

const NAV_ITEMS: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboardIcon, roles: null },
  {
    href: "/compose",
    label: "Tulis Surat",
    icon: PenSquareIcon,
    roles: ["admin", "creator", "secretary"],
  },
  {
    href: "/approvals",
    label: "Approval",
    icon: CheckCircleIcon,
    roles: ["admin", "approver"],
  },
  { href: "/organization", label: "Organisasi", icon: BuildingIcon, roles: null },
  { href: "/users", label: "Pengguna", icon: UsersIcon, roles: ["admin"] },
  { href: "/letter-types", label: "Jenis Surat", icon: TagIcon, roles: ["admin"] },
  {
    href: "/letter-templates",
    label: "Template",
    icon: LayersIcon,
    roles: ["admin"],
  },
];

interface SidebarProps {
  me: User | null;
  collapsed: boolean;
  mobileOpen: boolean;
  onCloseMobile: () => void;
  onLogout: () => void;
}

export default function Sidebar({
  me,
  collapsed,
  mobileOpen,
  onCloseMobile,
  onLogout,
}: SidebarProps) {
  const pathname = usePathname();

  const visibleItems = NAV_ITEMS.filter(
    (item) =>
      item.roles === null ||
      (me?.roles.some((role) => item.roles?.includes(role)) ?? false),
  );

  const content = (
    <div className="flex h-full flex-col bg-navy-900 text-navy-100">
      <div
        className={`flex h-16 items-center gap-3 border-b border-navy-800 px-4 ${collapsed ? "lg:justify-center lg:px-2" : ""}`}
      >
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-sky-400 to-cyan-500 text-sm font-bold text-navy-950 shadow-sm">
          e
        </div>
        <div className={`min-w-0 ${collapsed ? "lg:hidden" : ""}`}>
          <p className="truncate text-sm font-semibold text-white">eOffice Pro</p>
          <p className="truncate text-[11px] text-navy-300">FKK Group</p>
        </div>
        <button
          onClick={onCloseMobile}
          className="ml-auto rounded-lg p-1.5 text-navy-300 hover:bg-navy-800 hover:text-white lg:hidden"
          aria-label="Tutup menu"
        >
          <XIcon className="h-5 w-5" />
        </button>
      </div>

      <nav className="flex-1 overflow-y-auto px-3 py-4">
        <p
          className={`mb-2 px-3 text-[11px] font-semibold uppercase tracking-wider text-navy-400 ${collapsed ? "lg:hidden" : ""}`}
        >
          Menu Utama
        </p>
        <ul className="flex flex-col gap-1">
          {visibleItems.map((item) => {
            const active = pathname === item.href;
            const Icon = item.icon;
            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  title={item.label}
                  aria-current={active ? "page" : undefined}
                  className={`group flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition ${
                    collapsed ? "lg:justify-center lg:px-2" : ""
                  } ${
                    active
                      ? "bg-navy-800 font-semibold text-white shadow-sm"
                      : "text-navy-200 hover:bg-navy-800/60 hover:text-white"
                  }`}
                >
                  <Icon
                    className={`h-5 w-5 shrink-0 ${active ? "text-cyan-400" : "text-navy-300 group-hover:text-cyan-300"}`}
                  />
                  <span className={`truncate ${collapsed ? "lg:hidden" : ""}`}>
                    {item.label}
                  </span>
                  {active && (
                    <span
                      className={`ml-auto h-1.5 w-1.5 rounded-full bg-cyan-400 ${collapsed ? "lg:hidden" : ""}`}
                    />
                  )}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>

      <div className="border-t border-navy-800 p-3">
        <button
          onClick={onLogout}
          title="Keluar"
          className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm text-navy-200 transition hover:bg-navy-800/60 hover:text-white ${collapsed ? "lg:justify-center lg:px-2" : ""}`}
        >
          <LogOutIcon className="h-5 w-5 shrink-0 text-navy-300" />
          <span className={collapsed ? "lg:hidden" : ""}>Keluar</span>
        </button>
      </div>
    </div>
  );

  return (
    <>
      {/* Overlay drawer untuk mobile/tablet */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-navy-950/60 backdrop-blur-sm lg:hidden"
          onClick={onCloseMobile}
          aria-hidden
        />
      )}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 transform shadow-xl transition-transform duration-200 lg:hidden ${
          mobileOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {content}
      </aside>

      {/* Sidebar statis untuk desktop */}
      <aside
        className={`sticky top-0 hidden h-screen shrink-0 transition-all duration-200 lg:block ${
          collapsed ? "lg:w-[76px]" : "lg:w-64"
        }`}
      >
        {content}
      </aside>
    </>
  );
}
