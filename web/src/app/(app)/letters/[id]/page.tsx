"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import {
  getLetterDetail,
  type LetterApprovalAction,
  type LetterDetail,
  type LetterApprovalStep,
} from "@/lib/api";

const STATUS_LABEL: Record<LetterDetail["status"], string> = {
  draft: "Draft",
  submitted: "Diajukan",
  in_approval: "Menunggu approval",
  revision: "Revisi",
  approved: "Disetujui",
  published: "Terbit",
  cancelled: "Dibatalkan",
  archived: "Arsip",
};

const STATUS_STYLE: Record<LetterDetail["status"], string> = {
  draft: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  submitted: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  in_approval: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  revision: "bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-300",
  approved: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  published: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  cancelled: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
  archived: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
};

const STEP_LABEL: Record<LetterApprovalStep["status"], string> = {
  pending: "Pending",
  waiting: "Menunggu",
  approved: "Disetujui",
  rejected: "Ditolak",
  skipped: "Dilewati",
};

const ACTION_LABEL: Record<LetterApprovalAction["action"], string> = {
  approve: "Setuju",
  reject: "Tolak",
  request_revision: "Minta revisi",
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

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default function LetterDetailPage() {
  const params = useParams<{ id: string }>();
  const [letter, setLetter] = useState<LetterDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getLetterDetail(params.id)
      .then((data) => setLetter(data.letter))
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat detail surat"),
      )
      .finally(() => setLoading(false));
  }, [params.id]);

  const actionsByStepID = useMemo(() => {
    const byStepID = new Map<string, LetterApprovalAction[]>();
    letter?.approval_actions.forEach((action) => {
      const list = byStepID.get(action.step_id) ?? [];
      list.push(action);
      byStepID.set(action.step_id, list);
    });
    return byStepID;
  }, [letter]);

  return (
    <main className="mx-auto grid w-full max-w-7xl flex-1 gap-6 px-6 py-8 lg:grid-cols-[1fr_360px]">
        {loading && <p className="text-sm text-zinc-500">Memuat detail surat...</p>}
        {error && (
          <p
            role="alert"
            className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
          >
            {error}
          </p>
        )}
        {!loading && !error && letter && (
          <>
            <section className="rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <div className="border-b border-zinc-200 px-6 py-5 dark:border-zinc-800">
                <div className="mb-3 flex flex-wrap items-center gap-2">
                  <span
                    className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${STATUS_STYLE[letter.status]}`}
                  >
                    {STATUS_LABEL[letter.status]}
                  </span>
                  <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                    {letter.company_code}
                  </span>
                  <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-800 dark:bg-sky-950 dark:text-sky-300">
                    {letter.letter_type_code}
                  </span>
                </div>
                <h1 className="text-2xl font-semibold text-zinc-950 dark:text-zinc-50">
                  {letter.subject}
                </h1>
                <p className="mt-2 font-mono text-sm text-navy-600 dark:text-sky-400">
                  {letter.letter_number ?? "Nomor belum terbit"}
                </p>
              </div>

              <div className="grid gap-5 px-6 py-5">
                <div className="grid gap-3 rounded-lg border border-zinc-200 bg-zinc-50 p-4 text-sm dark:border-zinc-800 dark:bg-zinc-950/40 md:grid-cols-2">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">
                      Pembuat
                    </p>
                    <p className="mt-1 text-zinc-900 dark:text-zinc-100">
                      {letter.creator_name} · {letter.creator_position_title}
                    </p>
                    {letter.on_behalf_of_title && (
                      <p className="mt-1 text-xs font-semibold text-cyan-700 dark:text-cyan-300">
                        a.n. {letter.on_behalf_of_title}
                      </p>
                    )}
                  </div>
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">
                      Perusahaan
                    </p>
                    <p className="mt-1 text-zinc-900 dark:text-zinc-100">
                      {letter.company_name}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">
                      Dibuat
                    </p>
                    <p className="mt-1 text-zinc-900 dark:text-zinc-100">
                      {formatDate(letter.created_at)}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">
                      Terbit
                    </p>
                    <p className="mt-1 text-zinc-900 dark:text-zinc-100">
                      {formatDate(letter.published_at)}
                    </p>
                  </div>
                </div>

                <div className="grid gap-3 rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900">
                  <h2 className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                    Penerima
                  </h2>
                  <div className="grid gap-3 md:grid-cols-2">
                    {(["to", "cc"] as const).map((type) => (
                      <div key={type}>
                        <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-zinc-500">
                          {type === "to" ? "To" : "CC"}
                        </p>
                        <div className="flex flex-wrap gap-2">
                          {letter.recipients
                            .filter((recipient) => recipient.type === type)
                            .map((recipient) => (
                              <span
                                key={`${recipient.type}-${recipient.target_type}-${recipient.target_id}`}
                                className="rounded-full border border-zinc-300 px-3 py-1 text-xs font-semibold text-zinc-700 dark:border-zinc-700 dark:text-zinc-300"
                              >
                                {recipient.label}
                              </span>
                            ))}
                          {letter.recipients.filter((recipient) => recipient.type === type)
                            .length === 0 && (
                            <span className="text-xs text-zinc-400">Tidak ada</span>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <article className="rounded-lg border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900">
                  <h2 className="mb-4 text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                    Isi Surat
                  </h2>
                  <div
                    className="prose prose-zinc max-w-none text-sm leading-7 dark:prose-invert"
                    dangerouslySetInnerHTML={{ __html: letter.body_html }}
                  />
                </article>
              </div>
            </section>

            <aside className="grid content-start gap-4">
              <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
                <h2 className="mb-3 text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                  Verifikasi
                </h2>
                {letter.final_pdf_url && (
                  <a
                    href={letter.final_pdf_url}
                    target="_blank"
                    rel="noreferrer"
                    className="mb-2 inline-flex rounded-lg border border-zinc-900 bg-zinc-900 px-3 py-2 text-sm font-semibold text-white hover:bg-zinc-800 dark:border-zinc-100 dark:bg-zinc-100 dark:text-zinc-950 dark:hover:bg-zinc-200"
                  >
                    Buka PDF Final
                  </a>
                )}
                {letter.verify_url ? (
                  <a
                    href={letter.verify_url}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex rounded-lg border border-navy-600 px-3 py-2 text-sm font-semibold text-navy-700 hover:bg-navy-50 dark:border-sky-500 dark:text-sky-300 dark:hover:bg-navy-900"
                  >
                    Buka Halaman Verifikasi
                  </a>
                ) : (
                  <p className="text-sm text-zinc-500">Token QR belum terbit.</p>
                )}
              </section>

              <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
                <h2 className="mb-3 text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                  Lampiran
                </h2>
                <div className="grid gap-2">
                  {letter.attachments.length === 0 && (
                    <p className="text-sm text-zinc-500">Tidak ada lampiran.</p>
                  )}
                  {letter.attachments.map((attachment) => (
                    <div
                      key={attachment.id}
                      className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800"
                    >
                      <p className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                        {attachment.file_name}
                      </p>
                      <p className="mt-1 text-xs text-zinc-500">
                        {formatBytes(attachment.size_bytes)} · scan {attachment.scan_status}
                      </p>
                      {attachment.download_url && (
                        <a
                          href={attachment.download_url}
                          target="_blank"
                          rel="noreferrer"
                          className="mt-2 inline-flex rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                        >
                          Buka
                        </a>
                      )}
                    </div>
                  ))}
                </div>
              </section>

              <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
                <h2 className="mb-3 text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                  Timeline Approval
                </h2>
                <div className="grid gap-3">
                  {letter.approval_steps.length === 0 && (
                    <p className="text-sm text-zinc-500">Belum ada approval step.</p>
                  )}
                  {letter.approval_steps.map((step) => (
                    <div key={step.id} className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800">
                      <div className="flex items-center justify-between gap-2">
                        <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                          {step.step_order}. {step.position_title}
                        </p>
                        <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                          {STEP_LABEL[step.status]}
                        </span>
                      </div>
                      <p className="mt-1 text-xs text-zinc-500">
                        Deadline {formatDate(step.sla_deadline)} · keputusan{" "}
                        {formatDate(step.decided_at)}
                      </p>
                      {(actionsByStepID.get(step.id) ?? []).map((action) => (
                        <div
                          key={action.id}
                          className="mt-2 rounded-lg bg-zinc-50 px-3 py-2 text-xs dark:bg-zinc-950/50"
                        >
                          <p className="font-semibold text-zinc-800 dark:text-zinc-200">
                            {ACTION_LABEL[action.action]} oleh {action.actor_name}
                          </p>
                          <p className="mt-1 text-zinc-500">
                            {formatDate(action.created_at)}
                            {action.note ? ` · ${action.note}` : ""}
                          </p>
                        </div>
                      ))}
                    </div>
                  ))}
                </div>
              </section>
            </aside>
          </>
        )}
    </main>
  );
}
