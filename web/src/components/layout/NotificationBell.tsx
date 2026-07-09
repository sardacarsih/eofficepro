"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  type AppNotification,
} from "@/lib/api";
import { relativeTime } from "@/lib/format";
import { BellIcon } from "./icons";

const POLL_INTERVAL_MS = 60_000;

// Tujuan klik notifikasi: approver diarahkan ke inbox approval, sisanya ke
// detail surat bila ada.
function notificationHref(item: AppNotification): string | null {
  if (item.event_type === "approval_waiting") return "/approvals";
  if (item.letter_id) return `/letters/${item.letter_id}`;
  return null;
}

// Bell notifikasi topbar (E08-2): badge jumlah belum dibaca + dropdown daftar.
// Polling ringan tiap menit; klik item menandai dibaca lalu navigasi.
export default function NotificationBell() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<AppNotification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);

  const refresh = useCallback(() => {
    listNotifications()
      .then((data) => {
        setItems(data.data);
        setUnreadCount(data.unread_count);
      })
      .catch(() => {
        // Biarkan data lama; polling berikutnya mencoba lagi.
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    refresh();
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") refresh();
    }, POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [refresh]);

  useEffect(() => {
    if (!open) return;
    function onMouseDown(event: MouseEvent) {
      if (!containerRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    window.addEventListener("mousedown", onMouseDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("mousedown", onMouseDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  async function handleItemClick(item: AppNotification) {
    setOpen(false);
    if (!item.read_at) {
      setItems((current) =>
        current.map((existing) =>
          existing.id === item.id
            ? { ...existing, read_at: new Date().toISOString() }
            : existing,
        ),
      );
      setUnreadCount((current) => Math.max(0, current - 1));
      markNotificationRead(item.id).catch(() => {});
    }
    const href = notificationHref(item);
    if (href) router.push(href);
  }

  async function handleMarkAllRead() {
    setItems((current) =>
      current.map((existing) =>
        existing.read_at
          ? existing
          : { ...existing, read_at: new Date().toISOString() },
      ),
    );
    setUnreadCount(0);
    try {
      await markAllNotificationsRead();
    } catch {
      refresh();
    }
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        onClick={() => {
          setOpen((current) => !current);
          if (!open) refresh();
        }}
        aria-haspopup="true"
        aria-expanded={open}
        aria-label={
          unreadCount > 0
            ? `Notifikasi, ${unreadCount} belum dibaca`
            : "Notifikasi"
        }
        className="relative rounded-lg p-2 text-zinc-500 transition hover:bg-zinc-100 hover:text-zinc-900 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
      >
        <BellIcon className="h-5 w-5" />
        {unreadCount > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold leading-none text-white ring-2 ring-white dark:ring-zinc-900">
            {unreadCount > 9 ? "9+" : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="fixed inset-x-3 top-16 z-40 overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-xl sm:absolute sm:inset-x-auto sm:right-0 sm:top-full sm:mt-2 sm:w-[22rem] dark:border-zinc-700 dark:bg-zinc-900">
          <div className="flex items-center justify-between border-b border-zinc-100 px-4 py-3 dark:border-zinc-800">
            <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
              Notifikasi
            </p>
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                className="text-xs font-medium text-navy-700 transition hover:text-navy-900 dark:text-navy-300 dark:hover:text-navy-200"
              >
                Tandai semua dibaca
              </button>
            )}
          </div>

          <div className="max-h-96 overflow-y-auto">
            {loading ? (
              <p className="px-4 py-8 text-center text-sm text-zinc-500">
                Memuat notifikasi...
              </p>
            ) : items.length === 0 ? (
              <p className="px-4 py-8 text-center text-sm text-zinc-500">
                Belum ada notifikasi.
              </p>
            ) : (
              <ul className="divide-y divide-zinc-100 dark:divide-zinc-800">
                {items.map((item) => (
                  <li key={item.id}>
                    <button
                      onClick={() => handleItemClick(item)}
                      className={`flex w-full items-start gap-2.5 px-4 py-3 text-left transition hover:bg-zinc-50 dark:hover:bg-zinc-800 ${
                        item.read_at ? "" : "bg-navy-50/60 dark:bg-navy-950/40"
                      }`}
                    >
                      <span
                        className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${
                          item.read_at
                            ? "bg-transparent"
                            : "bg-navy-600 dark:bg-navy-400"
                        }`}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium text-zinc-900 dark:text-zinc-100">
                          {item.title}
                        </span>
                        <span className="mt-0.5 line-clamp-2 block text-xs text-zinc-500 dark:text-zinc-400">
                          {item.body}
                        </span>
                        <span className="mt-1 block text-[11px] text-zinc-400 dark:text-zinc-500">
                          {relativeTime(item.created_at)}
                        </span>
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
