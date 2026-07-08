"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  listApprovalInbox,
  listIncomingLetters,
  listMyLetters,
  type ApprovalInboxItem,
  type DraftLetter,
  type IncomingLetter,
} from "@/lib/api";

type InboxTab = "to" | "cc" | "actions" | "sent";

interface InboxData {
  to: IncomingLetter[];
  cc: IncomingLetter[];
  approvals: ApprovalInboxItem[];
  sent: DraftLetter[];
}

const TAB_LABEL: Record<InboxTab, string> = {
  to: "Surat Masuk",
  cc: "Tembusan",
  actions: "Menunggu Aksi",
  sent: "Terkirim",
};

const PRIORITY_LABEL: Record<DraftLetter["priority"], string> = {
  normal: "Normal",
  urgent: "Urgent",
};

const PRIORITY_STYLE: Record<DraftLetter["priority"], string> = {
  normal: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  urgent: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
};

const CLASSIFICATION_LABEL: Record<DraftLetter["classification"], string> = {
  biasa: "Biasa",
  terbatas: "Terbatas",
  rahasia: "Rahasia",
};

const STATUS_LABEL: Record<DraftLetter["status"], string> = {
  draft: "Draft",
  submitted: "Diajukan",
  in_approval: "Approval",
  revision: "Revisi",
  approved: "Disetujui",
  published: "Terbit",
  cancelled: "Dibatalkan",
  archived: "Arsip",
};

function formatDate(value: string | null): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return `${date.toLocaleDateString("id-ID")} ${date.toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
  })}`;
}

function excerpt(value: string, limit = 220): string {
  const text = value.replace(/\s+/g, " ").trim();
  if (!text) return "Isi surat belum tersedia.";
  return text.length > limit ? `${text.slice(0, limit)}...` : text;
}

function matchesSearch(value: string, query: string): boolean {
  return value.toLowerCase().includes(query.toLowerCase());
}

async function fetchInboxData(): Promise<InboxData> {
  const [toData, ccData, approvalData, sentData] = await Promise.all([
    listIncomingLetters("to"),
    listIncomingLetters("cc"),
    listApprovalInbox(),
    listMyLetters(),
  ]);
  return {
    to: toData.letters,
    cc: ccData.letters,
    approvals: approvalData.approvals,
    sent: sentData.letters,
  };
}

export default function InboxPage() {
  const [activeTab, setActiveTab] = useState<InboxTab>("to");
  const [toLetters, setToLetters] = useState<IncomingLetter[]>([]);
  const [ccLetters, setCcLetters] = useState<IncomingLetter[]>([]);
  const [approvals, setApprovals] = useState<ApprovalInboxItem[]>([]);
  const [sentLetters, setSentLetters] = useState<DraftLetter[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setError(null);
    const data = await fetchInboxData();
    setToLetters(data.to);
    setCcLetters(data.cc);
    setApprovals(data.approvals);
    setSentLetters(data.sent);
  }, []);

  useEffect(() => {
    let active = true;
    fetchInboxData()
      .then((data) => {
        if (!active) return;
        setToLetters(data.to);
        setCcLetters(data.cc);
        setApprovals(data.approvals);
        setSentLetters(data.sent);
      })
      .catch((err) => {
        if (!active) return;
        setError(err instanceof Error ? err.message : "Gagal memuat surat masuk");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const stats = useMemo(
    () => ({
      to: toLetters.filter((item) => !item.is_read).length,
      cc: ccLetters.filter((item) => !item.is_read).length,
      actions: approvals.length,
      sent: sentLetters.length,
    }),
    [approvals.length, ccLetters, sentLetters.length, toLetters],
  );

  const filteredTo = useMemo(
    () =>
      toLetters.filter((item) =>
        matchesSearch(
          `${item.subject} ${item.letter_number ?? ""} ${item.creator_name} ${item.body_plain}`,
          query,
        ),
      ),
    [query, toLetters],
  );

  const filteredCc = useMemo(
    () =>
      ccLetters.filter((item) =>
        matchesSearch(
          `${item.subject} ${item.letter_number ?? ""} ${item.creator_name} ${item.body_plain}`,
          query,
        ),
      ),
    [ccLetters, query],
  );

  const filteredApprovals = useMemo(
    () =>
      approvals.filter((item) =>
        matchesSearch(`${item.subject} ${item.creator_name} ${item.body_plain}`, query),
      ),
    [approvals, query],
  );

  const filteredSent = useMemo(
    () =>
      sentLetters.filter((item) =>
        matchesSearch(
          `${item.subject} ${item.letter_number ?? ""} ${item.body_plain} ${item.status}`,
          query,
        ),
      ),
    [query, sentLetters],
  );

  return (
    <main className="mx-auto w-full max-w-7xl flex-1 px-6 py-8">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
            Surat Masuk
          </h1>
          <p className="text-sm text-zinc-500">
            {stats.to + stats.cc} surat belum dibaca dan {stats.actions} menunggu aksi
          </p>
        </div>
        <button
          onClick={() =>
            reload().catch((err) =>
              setError(err instanceof Error ? err.message : "Gagal memuat ulang"),
            )
          }
          disabled={loading}
          className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
        >
          Muat Ulang
        </button>
      </div>

      <div className="mb-5 grid gap-3 lg:grid-cols-[1fr_280px]">
        <div className="flex flex-wrap gap-2">
          {(["to", "cc", "actions", "sent"] as const).map((tab) => {
            const active = activeTab === tab;
            return (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`rounded-lg border px-3 py-2 text-sm font-semibold transition ${
                  active
                    ? "border-navy-700 bg-navy-900 text-white dark:border-sky-500 dark:bg-sky-500 dark:text-navy-950"
                    : "border-zinc-300 bg-white text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
                }`}
              >
                {TAB_LABEL[tab]}
                <span
                  className={`ml-2 rounded-full px-1.5 py-0.5 text-[11px] ${
                    active
                      ? "bg-white/20 text-white dark:bg-navy-950/15 dark:text-navy-950"
                      : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                  }`}
                >
                  {stats[tab]}
                </span>
              </button>
            );
          })}
        </div>
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Cari surat"
          className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-950 outline-none transition focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
        />
      </div>

      {error && (
        <p
          role="alert"
          className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
        >
          {error}
        </p>
      )}
      {loading && <p className="text-sm text-zinc-500">Memuat surat masuk...</p>}

      {!loading && activeTab === "to" && (
        <IncomingList emptyText="Tidak ada surat masuk." letters={filteredTo} />
      )}
      {!loading && activeTab === "cc" && (
        <IncomingList emptyText="Tidak ada tembusan." letters={filteredCc} />
      )}
      {!loading && activeTab === "actions" && (
        <ApprovalList approvals={filteredApprovals} />
      )}
      {!loading && activeTab === "sent" && <SentList letters={filteredSent} />}
    </main>
  );
}

function IncomingList({
  letters,
  emptyText,
}: {
  letters: IncomingLetter[];
  emptyText: string;
}) {
  if (letters.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
        {emptyText}
      </p>
    );
  }

  return (
    <div className="grid gap-3">
      {letters.map((letter) => (
        <article
          key={`${letter.recipient_type}-${letter.id}`}
          className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
        >
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="mb-2 flex flex-wrap items-center gap-2">
                {!letter.is_read && (
                  <span className="rounded-full bg-cyan-100 px-2 py-0.5 text-[11px] font-semibold text-cyan-800 dark:bg-cyan-950 dark:text-cyan-300">
                    Baru
                  </span>
                )}
                <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                  {letter.company_code}
                </span>
                <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-800 dark:bg-sky-950 dark:text-sky-300">
                  {letter.letter_type_code}
                </span>
                <span
                  className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${PRIORITY_STYLE[letter.priority]}`}
                >
                  {PRIORITY_LABEL[letter.priority]}
                </span>
                <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">
                  {CLASSIFICATION_LABEL[letter.classification]}
                </span>
              </div>
              <h2 className="text-base font-semibold text-zinc-950 dark:text-zinc-50">
                {letter.subject}
              </h2>
              <p className="mt-1 font-mono text-xs text-navy-600 dark:text-sky-400">
                {letter.letter_number ?? "Nomor belum tersedia"}
              </p>
              <p className="mt-1 text-sm text-zinc-500">
                Dari {letter.creator_name} · {letter.creator_position_title}
              </p>
            </div>
            <div className="flex flex-col items-start gap-2 sm:items-end">
              <p className="text-xs text-zinc-500">
                Terbit {formatDate(letter.published_at)}
              </p>
              <Link
                href={`/letters/${letter.id}`}
                className="rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Buka Detail
              </Link>
            </div>
          </div>
          <p className="mt-4 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-3 text-sm leading-6 text-zinc-700 dark:border-zinc-800 dark:bg-zinc-950/40 dark:text-zinc-300">
            {excerpt(letter.body_plain)}
          </p>
          <p className="mt-2 text-xs text-zinc-500">
            {letter.attachment_count > 0
              ? `${letter.attachment_count} lampiran`
              : "Tidak ada lampiran"}
            {letter.is_read ? ` · dibaca ${formatDate(letter.last_read_at)}` : ""}
          </p>
        </article>
      ))}
    </div>
  );
}

function ApprovalList({ approvals }: { approvals: ApprovalInboxItem[] }) {
  if (approvals.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
        Tidak ada surat yang menunggu aksi.
      </p>
    );
  }

  return (
    <div className="grid gap-3">
      {approvals.map((item) => (
        <article
          key={item.step_id}
          className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
        >
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div className="mb-2 flex flex-wrap items-center gap-2">
                <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">
                  Step {item.step_order}
                </span>
                <span
                  className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${PRIORITY_STYLE[item.priority]}`}
                >
                  {PRIORITY_LABEL[item.priority]}
                </span>
              </div>
              <h2 className="text-base font-semibold text-zinc-950 dark:text-zinc-50">
                {item.subject}
              </h2>
              <p className="mt-1 text-sm text-zinc-500">
                Pembuat: {item.creator_name} · {item.creator_position}
              </p>
            </div>
            <Link
              href="/approvals"
              className="rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Proses Approval
            </Link>
          </div>
          <p className="mt-4 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-3 text-sm leading-6 text-zinc-700 dark:border-zinc-800 dark:bg-zinc-950/40 dark:text-zinc-300">
            {excerpt(item.body_plain)}
          </p>
        </article>
      ))}
    </div>
  );
}

function SentList({ letters }: { letters: DraftLetter[] }) {
  if (letters.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
        Belum ada surat terkirim.
      </p>
    );
  }

  return (
    <div className="grid gap-3">
      {letters.map((letter, index) => (
        <article
          key={`${letter.id}-${index}`}
          className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
        >
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div className="mb-2 flex flex-wrap items-center gap-2">
                <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                  {STATUS_LABEL[letter.status]}
                </span>
                <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-800 dark:bg-sky-950 dark:text-sky-300">
                  {letter.letter_type_code}
                </span>
              </div>
              <h2 className="text-base font-semibold text-zinc-950 dark:text-zinc-50">
                {letter.subject}
              </h2>
              <p className="mt-1 font-mono text-xs text-navy-600 dark:text-sky-400">
                {letter.letter_number ?? "Nomor belum terbit"}
              </p>
              <p className="mt-1 text-sm text-zinc-500">
                Diperbarui {formatDate(letter.updated_at)}
              </p>
            </div>
            <Link
              href={`/letters/${letter.id}`}
              className="rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Buka Detail
            </Link>
          </div>
          <p className="mt-4 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-3 text-sm leading-6 text-zinc-700 dark:border-zinc-800 dark:bg-zinc-950/40 dark:text-zinc-300">
            {excerpt(letter.body_plain)}
          </p>
        </article>
      ))}
    </div>
  );
}
