"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  actApprovalStep,
  listApprovalInbox,
  type ApprovalActionPayload,
  type ApprovalInboxItem,
  type PageMeta,
} from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import Pagination from "@/components/Pagination";

const PRIORITY_LABEL: Record<ApprovalInboxItem["priority"], string> = {
  normal: "Normal",
  urgent: "Urgent",
};

const PRIORITY_STYLE: Record<ApprovalInboxItem["priority"], string> = {
  normal: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  urgent: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
};

const CLASSIFICATION_LABEL: Record<ApprovalInboxItem["classification"], string> = {
  biasa: "Biasa",
  terbatas: "Terbatas",
  rahasia: "Rahasia",
};

const RESULT_LABEL: Record<string, string> = {
  in_approval: "Approval diteruskan ke step berikutnya.",
  approved: "Surat disetujui.",
  published: "Surat disetujui dan diterbitkan.",
  revision: "Surat dikembalikan untuk revisi.",
  cancelled: "Surat ditolak dan dibatalkan.",
};

function formatRelativeDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return `${date.toLocaleDateString("id-ID")} ${date.toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
  })}`;
}

function makeClientActionID(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function bodyExcerpt(value: string): string {
  const text = value.replace(/\s+/g, " ").trim();
  if (!text) return "Isi surat belum tersedia.";
  return text.length > 420 ? `${text.slice(0, 420)}...` : text;
}

export default function ApprovalsPage() {
  const me = useCurrentUser();
  const [approvals, setApprovals] = useState<ApprovalInboxItem[]>([]);
  const [page, setPage] = useState(1);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [busyStepID, setBusyStepID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  async function reloadApprovals() {
    const data = await listApprovalInbox({ page });
    setApprovals(data.data);
    setMeta(data.meta);
  }

  useEffect(() => {
    queueMicrotask(() => setLoading(true));
    listApprovalInbox({ page })
      .then((data) => {
        setApprovals(data.data);
        setMeta(data.meta);
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat approval"),
      )
      .finally(() => setLoading(false));
  }, [page]);

  const canApprove = useMemo(
    () => me?.capabilities?.can_approve ?? false,
    [me],
  );

  async function handleAction(
    item: ApprovalInboxItem,
    action: ApprovalActionPayload["action"],
  ) {
    if (action === "approve") {
      setError("Approval dengan tanda tangan wajib dilakukan dari Android tablet.");
      return;
    }
    const note = notes[item.step_id]?.trim() ?? "";
    if (!note) {
      setError("Catatan wajib diisi untuk Tolak atau Minta Revisi.");
      return;
    }

    setBusyStepID(item.step_id);
    setError(null);
    setSuccess(null);
    try {
      const result = await actApprovalStep(item.step_id, {
        action,
        note,
        client_action_id: makeClientActionID(),
        device_info: "web",
      });
      setNotes((current) => {
        const next = { ...current };
        delete next[item.step_id];
        return next;
      });
      await reloadApprovals();
      setSuccess(RESULT_LABEL[result.status] ?? "Aksi approval berhasil.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan aksi approval");
    } finally {
      setBusyStepID(null);
    }
  }

  return (
    <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
              Inbox Approval
            </h1>
            <p className="text-sm text-zinc-500">
              {meta?.total ?? approvals.length} surat menunggu keputusan
            </p>
          </div>
          <button
            onClick={() =>
              reloadApprovals().catch((err) =>
                setError(err instanceof Error ? err.message : "Gagal memuat ulang"),
              )
            }
            disabled={loading || busyStepID !== null}
            className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Muat Ulang
          </button>
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
        {!loading && me && !canApprove && (
          <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
            Tidak ada approval aktif untuk jabatan Anda.
          </p>
        )}

        {loading && <p className="text-sm text-zinc-500">Memuat approval...</p>}
        {!loading && canApprove && approvals.length === 0 && (
          <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
            Tidak ada surat yang menunggu approval.
          </p>
        )}

        <div className="grid gap-3">
          {approvals.map((item) => {
            const busy = busyStepID === item.step_id;
            const note = notes[item.step_id] ?? "";

            return (
              <article
                key={item.step_id}
                className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0">
                    <div className="mb-2 flex flex-wrap items-center gap-2">
                      <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                        {item.company_code}
                      </span>
                      <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-800 dark:bg-sky-950 dark:text-sky-300">
                        {item.letter_type_code}
                      </span>
                      <span
                        className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${PRIORITY_STYLE[item.priority]}`}
                      >
                        {PRIORITY_LABEL[item.priority]}
                      </span>
                      <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">
                        {CLASSIFICATION_LABEL[item.classification]}
                      </span>
                    </div>
                    <h2 className="text-base font-semibold text-zinc-950 dark:text-zinc-50">
                      {item.subject}
                    </h2>
                    <p className="mt-1 text-sm text-zinc-500">
                      Step {item.step_order} untuk {item.position_title} · diperbarui{" "}
                      {formatRelativeDate(item.updated_at)}
                    </p>
                    <p className="mt-1 text-sm text-zinc-500">
                      Pembuat: {item.creator_name} · {item.creator_position}
                    </p>
                  </div>
                  <Link
                    href={`/letters/${item.letter_id}`}
                    className="rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                  >
                    Buka Detail
                  </Link>
                </div>

                <div className="mt-4 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-3 dark:border-zinc-800 dark:bg-zinc-950/40">
                  <p className="whitespace-pre-wrap text-sm leading-6 text-zinc-700 dark:text-zinc-300">
                    {bodyExcerpt(item.body_plain)}
                  </p>
                  <p className="mt-2 text-xs text-zinc-500">
                    {item.attachment_count > 0
                      ? `${item.attachment_count} lampiran tercatat`
                      : "Tidak ada lampiran"}
                  </p>
                </div>

                <div className="mt-4 grid gap-3 md:grid-cols-[1fr_auto] md:items-end">
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Catatan
                    <textarea
                      value={note}
                      onChange={(e) =>
                        setNotes((current) => ({
                          ...current,
                          [item.step_id]: e.target.value,
                        }))
                      }
                      rows={3}
                      maxLength={1000}
                      placeholder="Opsional untuk setuju, wajib untuk tolak atau revisi"
                      className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    />
                  </label>
                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={() => void handleAction(item, "request_revision")}
                      disabled={busy || busyStepID !== null}
                      className="rounded-lg border border-amber-300 px-3 py-2 text-sm font-semibold text-amber-800 transition hover:bg-amber-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-amber-900 dark:text-amber-300 dark:hover:bg-amber-950"
                    >
                      Minta Revisi
                    </button>
                    <button
                      onClick={() => void handleAction(item, "reject")}
                      disabled={busy || busyStepID !== null}
                      className="rounded-lg border border-red-200 px-3 py-2 text-sm font-semibold text-red-700 transition hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-900 dark:text-red-300 dark:hover:bg-red-950"
                    >
                      Tolak
                    </button>
                    <button
                      type="button"
                      title="Approval dengan tanda tangan wajib dilakukan dari Android tablet."
                      disabled
                      className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      Setujui di Android
                    </button>
                  </div>
                  <p className="text-xs text-zinc-500 md:col-start-2">
                    Approval tanda tangan dilakukan dari Android tablet.
                  </p>
                </div>
              </article>
            );
          })}
        </div>
        <Pagination
          page={page}
          totalPages={meta?.total_pages ?? 1}
          onPageChange={setPage}
          disabled={loading}
        />
    </main>
  );
}
