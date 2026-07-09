"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  listApprovalInbox,
  listDispositionInbox,
  listIncomingLetters,
  listMyLetters,
  updateDispositionStatus,
  type ApprovalInboxItem,
  type DispositionInboxItem,
  type DispositionStatus,
  type DraftLetter,
  type IncomingLetter,
  type PageMeta,
} from "@/lib/api";
import Pagination from "@/components/Pagination";

type InboxTab = "to" | "cc" | "actions" | "dispositions" | "sent";

const TAB_LABEL: Record<InboxTab, string> = {
  to: "Surat Masuk",
  cc: "Tembusan",
  actions: "Menunggu Aksi",
  dispositions: "Disposisi",
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

const DISPOSITION_STATUS_LABEL: Record<DispositionStatus, string> = {
  open: "Belum diproses",
  in_progress: "Diproses",
  done: "Selesai",
};

const DISPOSITION_STATUS_STYLE: Record<DispositionStatus, string> = {
  open: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  in_progress: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  done: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
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

function formatDateOnly(value: string | null): string {
  if (!value) return "-";
  const [year, month, day] = value.slice(0, 10).split("-");
  if (!year || !month || !day) return "-";
  return `${day}/${month}/${year}`;
}

function isOverdue(value: string | null, status?: DispositionStatus): boolean {
  if (!value || status === "done") return false;
  const due = new Date(`${value.slice(0, 10)}T23:59:59`);
  return !Number.isNaN(due.getTime()) && due.getTime() < Date.now();
}

function excerpt(value: string, limit = 220): string {
  const text = value.replace(/\s+/g, " ").trim();
  if (!text) return "Isi surat belum tersedia.";
  return text.length > limit ? `${text.slice(0, limit)}...` : text;
}

function matchesSearch(value: string, query: string): boolean {
  return value.toLowerCase().includes(query.toLowerCase());
}

export default function InboxPage() {
  const [activeTab, setActiveTab] = useState<InboxTab>("to");

  const [toLetters, setToLetters] = useState<IncomingLetter[]>([]);
  const [toPage, setToPage] = useState(1);
  const [toMeta, setToMeta] = useState<PageMeta | null>(null);
  const [toLoaded, setToLoaded] = useState(false);

  const [ccLetters, setCcLetters] = useState<IncomingLetter[]>([]);
  const [ccPage, setCcPage] = useState(1);
  const [ccMeta, setCcMeta] = useState<PageMeta | null>(null);
  const [ccLoaded, setCcLoaded] = useState(false);

  const [approvals, setApprovals] = useState<ApprovalInboxItem[]>([]);
  const [actionsPage, setActionsPage] = useState(1);
  const [actionsMeta, setActionsMeta] = useState<PageMeta | null>(null);
  const [actionsLoaded, setActionsLoaded] = useState(false);

  const [dispositions, setDispositions] = useState<DispositionInboxItem[]>([]);
  const [dispositionsPage, setDispositionsPage] = useState(1);
  const [dispositionsMeta, setDispositionsMeta] = useState<PageMeta | null>(null);
  const [dispositionsLoaded, setDispositionsLoaded] = useState(false);

  const [sentLetters, setSentLetters] = useState<DraftLetter[]>([]);
  const [sentPage, setSentPage] = useState(1);
  const [sentMeta, setSentMeta] = useState<PageMeta | null>(null);
  const [sentLoaded, setSentLoaded] = useState(false);

  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [busyRecipientID, setBusyRecipientID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const pageByTab: Record<InboxTab, number> = {
    to: toPage,
    cc: ccPage,
    actions: actionsPage,
    dispositions: dispositionsPage,
    sent: sentPage,
  };
  const loadedByTab: Record<InboxTab, boolean> = {
    to: toLoaded,
    cc: ccLoaded,
    actions: actionsLoaded,
    dispositions: dispositionsLoaded,
    sent: sentLoaded,
  };

  const loadTab = useCallback(async (tab: InboxTab, page: number) => {
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case "to": {
          const data = await listIncomingLetters("to", { page });
          setToLetters(data.data);
          setToMeta(data.meta);
          setToPage(page);
          setToLoaded(true);
          break;
        }
        case "cc": {
          const data = await listIncomingLetters("cc", { page });
          setCcLetters(data.data);
          setCcMeta(data.meta);
          setCcPage(page);
          setCcLoaded(true);
          break;
        }
        case "actions": {
          const data = await listApprovalInbox({ page });
          setApprovals(data.data);
          setActionsMeta(data.meta);
          setActionsPage(page);
          setActionsLoaded(true);
          break;
        }
        case "dispositions": {
          const data = await listDispositionInbox({ page });
          setDispositions(data.data);
          setDispositionsMeta(data.meta);
          setDispositionsPage(page);
          setDispositionsLoaded(true);
          break;
        }
        case "sent": {
          const data = await listMyLetters({ page });
          setSentLetters(data.data);
          setSentMeta(data.meta);
          setSentPage(page);
          setSentLoaded(true);
          break;
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat surat masuk");
    } finally {
      setLoading(false);
    }
  }, []);

  const reload = useCallback(
    () => loadTab(activeTab, pageByTab[activeTab]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [loadTab, activeTab],
  );

  useEffect(() => {
    queueMicrotask(() => {
      loadTab("to", 1);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function handleTabChange(tab: InboxTab) {
    setActiveTab(tab);
    if (!loadedByTab[tab]) loadTab(tab, 1);
  }

  const stats = useMemo(
    () => ({
      to: toMeta?.total ?? toLetters.filter((item) => !item.is_read).length,
      cc: ccMeta?.total ?? ccLetters.filter((item) => !item.is_read).length,
      actions: actionsMeta?.total ?? approvals.length,
      dispositions:
        dispositionsMeta?.total ??
        dispositions.filter((item) => item.status !== "done").length,
      sent: sentMeta?.total ?? sentLetters.length,
    }),
    [
      actionsMeta,
      approvals.length,
      ccLetters,
      ccMeta,
      dispositions,
      dispositionsMeta,
      sentLetters.length,
      sentMeta,
      toLetters,
      toMeta,
    ],
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

  const filteredDispositions = useMemo(
    () =>
      dispositions.filter((item) =>
        matchesSearch(
          `${item.letter_subject} ${item.letter_number ?? ""} ${item.from_position_title} ${item.creator_name} ${item.my_position_title} ${item.instruction}`,
          query,
        ),
      ),
    [dispositions, query],
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

  async function handleDispositionStatus(
    item: DispositionInboxItem,
    status: "in_progress" | "done",
    followupNote: string,
  ) {
    const note = followupNote.trim();
    if (status === "done" && !note) {
      setError("Laporan tindak lanjut wajib diisi saat menyelesaikan disposisi.");
      return;
    }

    setBusyRecipientID(item.recipient_id);
    setError(null);
    setSuccess(null);
    try {
      await updateDispositionStatus(item.recipient_id, {
        status,
        followup_note: note,
      });
      await reload();
      setSuccess(
        status === "done"
          ? "Disposisi ditandai selesai."
          : "Disposisi ditandai sedang diproses.",
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memperbarui disposisi");
    } finally {
      setBusyRecipientID(null);
    }
  }

  return (
    <main className="mx-auto w-full max-w-7xl flex-1 px-6 py-8">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
            Surat Masuk
          </h1>
          <p className="text-sm text-zinc-500">
            {stats.to + stats.cc} surat belum dibaca, {stats.actions} approval, dan{" "}
            {stats.dispositions} disposisi aktif
          </p>
        </div>
        <button
          onClick={() =>
            reload().catch((err) =>
              setError(err instanceof Error ? err.message : "Gagal memuat ulang"),
            )
          }
          disabled={loading || busyRecipientID !== null}
          className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
        >
          Muat Ulang
        </button>
      </div>

      <div className="mb-5 grid gap-3 lg:grid-cols-[1fr_280px]">
        <div className="flex flex-wrap gap-2">
          {(["to", "cc", "actions", "dispositions", "sent"] as const).map((tab) => {
            const active = activeTab === tab;
            return (
              <button
                key={tab}
                onClick={() => handleTabChange(tab)}
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
        <div>
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Cari surat atau disposisi"
            className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-950 outline-none transition focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
          />
          <span className="mt-1 block text-[11px] text-zinc-400">
            Pencarian hanya berlaku pada halaman yang sedang dimuat
          </span>
        </div>
      </div>

      {success && (
        <p className="mb-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
          {success}
        </p>
      )}
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
        <>
          <IncomingList emptyText="Tidak ada surat masuk." letters={filteredTo} />
          <Pagination
            page={toPage}
            totalPages={toMeta?.total_pages ?? 1}
            onPageChange={(page) => loadTab("to", page)}
            disabled={loading}
          />
        </>
      )}
      {!loading && activeTab === "cc" && (
        <>
          <IncomingList emptyText="Tidak ada tembusan." letters={filteredCc} />
          <Pagination
            page={ccPage}
            totalPages={ccMeta?.total_pages ?? 1}
            onPageChange={(page) => loadTab("cc", page)}
            disabled={loading}
          />
        </>
      )}
      {!loading && activeTab === "actions" && (
        <>
          <ApprovalList approvals={filteredApprovals} />
          <Pagination
            page={actionsPage}
            totalPages={actionsMeta?.total_pages ?? 1}
            onPageChange={(page) => loadTab("actions", page)}
            disabled={loading}
          />
        </>
      )}
      {!loading && activeTab === "dispositions" && (
        <>
          <DispositionList
            busyRecipientID={busyRecipientID}
            dispositions={filteredDispositions}
            onStatusChange={handleDispositionStatus}
          />
          <Pagination
            page={dispositionsPage}
            totalPages={dispositionsMeta?.total_pages ?? 1}
            onPageChange={(page) => loadTab("dispositions", page)}
            disabled={loading}
          />
        </>
      )}
      {!loading && activeTab === "sent" && (
        <>
          <SentList letters={filteredSent} />
          <Pagination
            page={sentPage}
            totalPages={sentMeta?.total_pages ?? 1}
            onPageChange={(page) => loadTab("sent", page)}
            disabled={loading}
          />
        </>
      )}
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

function DispositionList({
  dispositions,
  busyRecipientID,
  onStatusChange,
}: {
  dispositions: DispositionInboxItem[];
  busyRecipientID: string | null;
  onStatusChange: (
    item: DispositionInboxItem,
    status: "in_progress" | "done",
    followupNote: string,
  ) => Promise<void>;
}) {
  const [notes, setNotes] = useState<Record<string, string>>({});

  if (dispositions.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
        Tidak ada disposisi untuk jabatan aktif Anda.
      </p>
    );
  }

  return (
    <div className="grid gap-3">
      {dispositions.map((item) => {
        const note = notes[item.recipient_id] ?? item.followup_note ?? "";
        const busy = busyRecipientID === item.recipient_id;
        const disabled = busyRecipientID !== null || item.status === "done";
        const overdue = isOverdue(item.due_date, item.status);

        return (
          <article
            key={item.recipient_id}
            className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
          >
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="min-w-0">
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <span
                    className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${DISPOSITION_STATUS_STYLE[item.status]}`}
                  >
                    {DISPOSITION_STATUS_LABEL[item.status]}
                  </span>
                  {overdue && (
                    <span className="rounded-full bg-red-100 px-2 py-0.5 text-[11px] font-semibold text-red-800 dark:bg-red-950 dark:text-red-300">
                      Lewat tenggat
                    </span>
                  )}
                  <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                    Untuk {item.my_position_title}
                  </span>
                </div>
                <h2 className="text-base font-semibold text-zinc-950 dark:text-zinc-50">
                  {item.letter_subject}
                </h2>
                <p className="mt-1 font-mono text-xs text-navy-600 dark:text-sky-400">
                  {item.letter_number ?? "Nomor belum tersedia"}
                </p>
                <p className="mt-1 text-sm text-zinc-500">
                  Dari {item.creator_name} · {item.from_position_title}
                </p>
              </div>
              <div className="flex flex-col items-start gap-2 sm:items-end">
                <p className="text-xs text-zinc-500">
                  Tenggat {formatDateOnly(item.due_date)}
                </p>
                <Link
                  href={`/letters/${item.letter_id}`}
                  className="rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                >
                  Buka Detail
                </Link>
              </div>
            </div>

            <p className="mt-4 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-3 text-sm leading-6 text-zinc-700 dark:border-zinc-800 dark:bg-zinc-950/40 dark:text-zinc-300">
              {excerpt(item.instruction, 520)}
            </p>

            <div className="mt-4 grid gap-3 md:grid-cols-[1fr_auto] md:items-end">
              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Laporan tindak lanjut
                <textarea
                  value={note}
                  onChange={(event) =>
                    setNotes((current) => ({
                      ...current,
                      [item.recipient_id]: event.target.value,
                    }))
                  }
                  disabled={item.status === "done"}
                  rows={3}
                  maxLength={1200}
                  placeholder="Wajib saat menandai selesai"
                  className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-900"
                />
              </label>
              <div className="flex flex-wrap gap-2 md:justify-end">
                {item.status === "open" && (
                  <button
                    onClick={() => void onStatusChange(item, "in_progress", note)}
                    disabled={disabled}
                    className="rounded-lg border border-sky-300 px-3 py-2 text-sm font-semibold text-sky-800 transition hover:bg-sky-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-sky-800 dark:text-sky-300 dark:hover:bg-sky-950"
                  >
                    {busy ? "Memproses..." : "Mulai Proses"}
                  </button>
                )}
                <button
                  onClick={() => void onStatusChange(item, "done", note)}
                  disabled={disabled}
                  className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {busy ? "Menyimpan..." : item.status === "done" ? "Selesai" : "Tandai Selesai"}
                </button>
              </div>
            </div>
            {item.completed_at && (
              <p className="mt-2 text-xs text-zinc-500">
                Selesai {formatDate(item.completed_at)}
              </p>
            )}
          </article>
        );
      })}
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
