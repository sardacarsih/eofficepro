"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import {
  ApiError,
  createDisposition,
  createLetterComment,
	  downloadAuthenticatedFile,
  getLetterDetail,
  listLetterComments,
  listLetterDispositions,
  listAllPositions,
  type DispositionItem,
  type DispositionRecipient,
  type DispositionStatus,
  type LetterApprovalAction,
  type LetterComment,
  type LetterDetail,
  type LetterApprovalStep,
  type PageMeta,
  type Position,
} from "@/lib/api";
import Pagination from "@/components/Pagination";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";

const COMMENT_MAX_LENGTH = 2000;

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

interface DispositionFormState {
  from_position_id: string;
  instruction: string;
  due_date: string;
  parent_disposition_id: string;
  recipient_position_ids: string[];
}

type DispositionAccessState = "hidden" | "loading" | "allowed" | "error";

const EMPTY_DISPOSITION_FORM: DispositionFormState = {
  from_position_id: "",
  instruction: "",
  due_date: "",
  parent_disposition_id: "",
  recipient_position_ids: [],
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

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function dispositionRecipientSummary(recipients: DispositionRecipient[]): string {
  if (recipients.length === 0) return "Belum ada penerima";
  const done = recipients.filter((recipient) => recipient.status === "done").length;
  return `${done}/${recipients.length} selesai`;
}

function matchesPosition(position: Position, query: string): boolean {
  const value = [
    position.title,
    position.org_unit_name,
    position.position_type,
    position.holder_name,
  ]
    .join(" ")
    .toLowerCase();
  return value.includes(query.toLowerCase());
}

export default function LetterDetailPage() {
  const params = useParams<{ id: string }>();
  const me = useCurrentUser();
  const [letter, setLetter] = useState<LetterDetail | null>(null);
  const [dispositions, setDispositions] = useState<DispositionItem[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [dispositionForm, setDispositionForm] =
    useState<DispositionFormState>(EMPTY_DISPOSITION_FORM);
  const [positionQuery, setPositionQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [savingDisposition, setSavingDisposition] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dispositionAccess, setDispositionAccess] =
    useState<DispositionAccessState>("hidden");
  const [dispositionLoadError, setDispositionLoadError] = useState<string | null>(
    null,
  );
  const [dispositionError, setDispositionError] = useState<string | null>(null);
  const [dispositionSuccess, setDispositionSuccess] = useState<string | null>(null);
  const [comments, setComments] = useState<LetterComment[]>([]);
  const [commentsMeta, setCommentsMeta] = useState<PageMeta | null>(null);
  const [commentsLoading, setCommentsLoading] = useState(true);
  const [commentsError, setCommentsError] = useState<string | null>(null);
  const [commentBody, setCommentBody] = useState("");
  const [postingComment, setPostingComment] = useState(false);
  const [commentSubmitError, setCommentSubmitError] = useState<string | null>(null);

  const myPositions = useMemo(() => me?.positions ?? [], [me]);
  const selectedFromPositionID =
    dispositionForm.from_position_id || myPositions[0]?.position_id || "";
  const heldPositionIDs = useMemo(
    () => new Set(myPositions.map((position) => position.position_id)),
    [myPositions],
  );

  const loadDispositions = useCallback(async () => {
    const data = await listLetterDispositions(params.id, { pageSize: 100 });
    setDispositions(data.data);
  }, [params.id]);

  const loadComments = useCallback(
    async (page: number) => {
      setCommentsLoading(true);
      setCommentsError(null);
      try {
        const data = await listLetterComments(params.id, { page });
        setComments(data.data);
        setCommentsMeta(data.meta);
      } catch (err) {
        setCommentsError(
          err instanceof Error ? err.message : "Gagal memuat komentar",
        );
      } finally {
        setCommentsLoading(false);
      }
    },
    [params.id],
  );

  useEffect(() => {
    let active = true;

    async function loadPublishedDispositionData() {
      setDispositionAccess("loading");

      try {
        const dispositionData = await listLetterDispositions(params.id, {
          pageSize: 100,
        });
        if (!active) return;
        setDispositions(dispositionData.data);
        setDispositionAccess("allowed");
      } catch (err) {
        if (!active) return;
        if (err instanceof ApiError && err.status === 403) {
          setDispositionAccess("hidden");
          return;
        }
        setDispositionLoadError(
          err instanceof Error ? err.message : "Gagal memuat disposisi",
        );
        setDispositionAccess("error");
        return;
      }

      try {
        const positionData = await listAllPositions();
        if (!active) return;
        setPositions(positionData.data);
      } catch (err) {
        if (!active) return;
        setDispositionLoadError(
          err instanceof Error ? err.message : "Gagal memuat daftar jabatan",
        );
      }
    }

    getLetterDetail(params.id)
      .then((letterData) => {
        if (!active) return;
        setLetter(letterData.letter);
        void loadComments(1);
        if (letterData.letter.status === "published") {
          void loadPublishedDispositionData();
        }
      })
      .catch((err) => {
        if (!active) return;
        setError(err instanceof Error ? err.message : "Gagal memuat detail surat");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [params.id, loadComments]);

  const actionsByStepID = useMemo(() => {
    const byStepID = new Map<string, LetterApprovalAction[]>();
    letter?.approval_actions.forEach((action) => {
      const list = byStepID.get(action.step_id) ?? [];
      list.push(action);
      byStepID.set(action.step_id, list);
    });
    return byStepID;
  }, [letter]);

  const parentDispositionOptions = useMemo(
    () =>
      dispositions.filter((disposition) =>
        disposition.recipients.some((recipient) =>
          heldPositionIDs.has(recipient.position_id),
        ),
      ),
    [dispositions, heldPositionIDs],
  );

  const selectedRecipientIDs = useMemo(
    () => new Set(dispositionForm.recipient_position_ids),
    [dispositionForm.recipient_position_ids],
  );

  const recipientOptions = useMemo(
    () =>
      positions
        .filter(
          (position) =>
            position.is_active &&
            position.id !== selectedFromPositionID &&
            !selectedRecipientIDs.has(position.id) &&
            matchesPosition(position, positionQuery),
        )
        .slice(0, 24),
    [positionQuery, positions, selectedFromPositionID, selectedRecipientIDs],
  );

  const selectedRecipients = useMemo(
    () =>
      dispositionForm.recipient_position_ids
        .map((id) => positions.find((position) => position.id === id))
        .filter((position): position is Position => Boolean(position)),
    [dispositionForm.recipient_position_ids, positions],
  );

  async function handleCreateDisposition(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const instruction = dispositionForm.instruction.trim();
    if (!selectedFromPositionID) {
      setDispositionError("Pilih jabatan pengirim disposisi.");
      return;
    }
    if (!instruction) {
      setDispositionError("Instruksi disposisi wajib diisi.");
      return;
    }
    if (dispositionForm.recipient_position_ids.length === 0) {
      setDispositionError("Pilih minimal satu penerima disposisi.");
      return;
    }

    setSavingDisposition(true);
    setDispositionError(null);
    setDispositionSuccess(null);
    try {
      await createDisposition(params.id, {
        from_position_id: selectedFromPositionID,
        instruction,
        due_date: dispositionForm.due_date || null,
        parent_disposition_id: dispositionForm.parent_disposition_id || null,
        recipient_position_ids: dispositionForm.recipient_position_ids,
      });
      await loadDispositions();
      setDispositionForm((current) => ({
        ...EMPTY_DISPOSITION_FORM,
        from_position_id: current.from_position_id || selectedFromPositionID,
      }));
      setPositionQuery("");
      setDispositionSuccess("Disposisi berhasil dikirim.");
    } catch (err) {
      setDispositionError(
        err instanceof Error ? err.message : "Gagal menyimpan disposisi",
      );
    } finally {
      setSavingDisposition(false);
    }
  }

  async function handleCreateComment(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = commentBody.trim();
    if (!body) {
      setCommentSubmitError("Komentar wajib diisi.");
      return;
    }
    // Hitung per code point agar konsisten dengan validasi rune di server.
    if (Array.from(body).length > COMMENT_MAX_LENGTH) {
      setCommentSubmitError(
        `Komentar maksimal ${COMMENT_MAX_LENGTH} karakter.`,
      );
      return;
    }

    setPostingComment(true);
    setCommentSubmitError(null);
    try {
      await createLetterComment(params.id, body);
      setCommentBody("");
      // Komentar terurut kronologis: komentar baru ada di halaman terakhir.
      const total = (commentsMeta?.total ?? comments.length) + 1;
      const pageSize = commentsMeta?.page_size ?? 20;
      await loadComments(Math.max(1, Math.ceil(total / pageSize)));
    } catch (err) {
      setCommentSubmitError(
        err instanceof Error ? err.message : "Gagal mengirim komentar",
      );
    } finally {
      setPostingComment(false);
    }
  }

  return (
    <main className="mx-auto grid w-full max-w-7xl flex-1 gap-6 px-6 py-8 lg:grid-cols-[1fr_380px]">
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
                {(letter.approval_category_name || letter.coordination_scope) && (
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">Kebijakan Approval</p>
                    <p className="mt-1 text-zinc-900 dark:text-zinc-100">
                      {letter.approval_category_name ?? letter.coordination_scope?.replaceAll("_", " ")} · {letter.resolved_final_level?.replaceAll("_", " ")}
                    </p>
                  </div>
                )}
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
			  {letter.status === "published" && (
				<button
				  type="button"
				  onClick={() => void downloadAuthenticatedFile(
					`/letters/view/${letter.id}/final-pdf`,
					`${letter.letter_number ?? "surat"}.pdf`,
				  )}
                  className="mb-2 inline-flex rounded-lg border border-zinc-900 bg-zinc-900 px-3 py-2 text-sm font-semibold text-white hover:bg-zinc-800 dark:border-zinc-100 dark:bg-zinc-100 dark:text-zinc-950 dark:hover:bg-zinc-200"
				>
				  Buka PDF Final
				</button>
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
					{attachment.scan_status === "clean" && (
					  <button
						type="button"
						onClick={() => void downloadAuthenticatedFile(
						  `/letters/view/${letter.id}/attachments/${attachment.id}/download`,
						  attachment.file_name,
						)}
                        className="mt-2 inline-flex rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
					  >
						Buka
					  </button>
                    )}
                  </div>
                ))}
              </div>
            </section>

            {letter.status === "published" &&
              dispositionAccess !== "hidden" && (
              <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <div className="mb-3 flex items-center justify-between gap-3">
                <h2 className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                  Disposisi
                </h2>
                <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                  {dispositions.length}
                </span>
              </div>

              {dispositionAccess === "loading" && (
                <p className="text-sm text-zinc-500">Memuat disposisi...</p>
              )}

              {dispositionLoadError && (
                <p
                  role="alert"
                  className="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  {dispositionLoadError}
                </p>
              )}

              {dispositionAccess === "allowed" && dispositionSuccess && (
                <p className="mb-3 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
                  {dispositionSuccess}
                </p>
              )}
              {dispositionAccess === "allowed" && dispositionError && (
                <p
                  role="alert"
                  className="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  {dispositionError}
                </p>
              )}

              {dispositionAccess === "allowed" && (
                <>
                <div className="mb-4 grid gap-3">
                {dispositions.length === 0 && (
                  <p className="rounded-lg border border-dashed border-zinc-300 px-3 py-5 text-center text-sm text-zinc-500 dark:border-zinc-700">
                    Belum ada disposisi.
                  </p>
                )}
                {dispositions.map((disposition) => (
                  <div
                    key={disposition.id}
                    className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                          {disposition.from_position_title}
                        </p>
                        <p className="mt-1 text-xs text-zinc-500">
                          {formatDate(disposition.created_at)} · tenggat{" "}
                          {formatDateOnly(disposition.due_date)}
                        </p>
                      </div>
                      <span className="shrink-0 rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                        {dispositionRecipientSummary(disposition.recipients)}
                      </span>
                    </div>
                    {disposition.parent_disposition_id && (
                      <p className="mt-2 text-xs font-semibold text-cyan-700 dark:text-cyan-300">
                        Disposisi lanjutan
                      </p>
                    )}
                    <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-zinc-700 dark:text-zinc-300">
                      {disposition.instruction}
                    </p>
                    <div className="mt-3 grid gap-2">
                      {disposition.recipients.map((recipient) => (
                        <div
                          key={recipient.id}
                          className="rounded-lg bg-zinc-50 px-3 py-2 text-xs dark:bg-zinc-950/50"
                        >
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <p className="font-semibold text-zinc-800 dark:text-zinc-200">
                              {recipient.position_title}
                            </p>
                            <span
                              className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${DISPOSITION_STATUS_STYLE[recipient.status]}`}
                            >
                              {DISPOSITION_STATUS_LABEL[recipient.status]}
                            </span>
                          </div>
                          <p className="mt-1 text-zinc-500">
                            {recipient.holder_name || "Belum ada pemegang aktif"}
                            {recipient.completed_at
                              ? ` · selesai ${formatDate(recipient.completed_at)}`
                              : ""}
                          </p>
                          {recipient.followup_note && (
                            <p className="mt-2 whitespace-pre-wrap text-zinc-600 dark:text-zinc-400">
                              {recipient.followup_note}
                            </p>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
                </div>

                <form onSubmit={handleCreateDisposition} className="grid gap-3">
                <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Jabatan pengirim
                  <select
                    value={selectedFromPositionID}
                    onChange={(event) =>
                      setDispositionForm((current) => ({
                        ...current,
                        from_position_id: event.target.value,
                        recipient_position_ids: current.recipient_position_ids.filter(
                          (id) => id !== event.target.value,
                        ),
                      }))
                    }
                    disabled={savingDisposition || myPositions.length === 0}
                    className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-900"
                  >
                    {myPositions.length === 0 && (
                      <option value="">Tidak ada jabatan aktif</option>
                    )}
                    {myPositions.map((position) => (
                      <option key={position.position_id} value={position.position_id}>
                        {position.title} · {position.org_unit}
                      </option>
                    ))}
                  </select>
                </label>

                {parentDispositionOptions.length > 0 && (
                  <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Disposisi induk
                    <select
                      value={dispositionForm.parent_disposition_id}
                      onChange={(event) =>
                        setDispositionForm((current) => ({
                          ...current,
                          parent_disposition_id: event.target.value,
                        }))
                      }
                      disabled={savingDisposition}
                      className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-900"
                    >
                      <option value="">Disposisi baru dari surat</option>
                      {parentDispositionOptions.map((disposition) => (
                        <option key={disposition.id} value={disposition.id}>
                          Dari {disposition.from_position_title} ·{" "}
                          {formatDateOnly(disposition.due_date)}
                        </option>
                      ))}
                    </select>
                  </label>
                )}

                <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Instruksi
                  <textarea
                    value={dispositionForm.instruction}
                    onChange={(event) =>
                      setDispositionForm((current) => ({
                        ...current,
                        instruction: event.target.value,
                      }))
                    }
                    disabled={savingDisposition}
                    rows={4}
                    maxLength={1500}
                    placeholder="Tuliskan instruksi tindak lanjut"
                    className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-900"
                  />
                </label>

                <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Tenggat
                  <input
                    type="date"
                    value={dispositionForm.due_date}
                    onChange={(event) =>
                      setDispositionForm((current) => ({
                        ...current,
                        due_date: event.target.value,
                      }))
                    }
                    disabled={savingDisposition}
                    className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-900"
                  />
                </label>

                <div className="grid gap-2">
                  <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Penerima
                    <input
                      value={positionQuery}
                      onChange={(event) => setPositionQuery(event.target.value)}
                      disabled={savingDisposition}
                      placeholder="Cari jabatan atau unit"
                      className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-900"
                    />
                  </label>
                  {selectedRecipients.length > 0 && (
                    <div className="flex flex-wrap gap-2">
                      {selectedRecipients.map((position) => (
                        <button
                          key={position.id}
                          type="button"
                          onClick={() =>
                            setDispositionForm((current) => ({
                              ...current,
                              recipient_position_ids:
                                current.recipient_position_ids.filter(
                                  (id) => id !== position.id,
                                ),
                            }))
                          }
                          disabled={savingDisposition}
                          className="rounded-full border border-navy-200 bg-navy-50 px-3 py-1 text-left text-xs font-semibold text-navy-800 hover:bg-navy-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-sky-900 dark:bg-navy-950 dark:text-sky-300"
                        >
                          {position.title}
                        </button>
                      ))}
                    </div>
                  )}
                  <div className="max-h-56 overflow-y-auto rounded-lg border border-zinc-200 dark:border-zinc-800">
                    {recipientOptions.length === 0 ? (
                      <p className="px-3 py-4 text-sm text-zinc-500">
                        Tidak ada jabatan yang sesuai.
                      </p>
                    ) : (
                      recipientOptions.map((position) => (
                        <button
                          key={position.id}
                          type="button"
                          onClick={() =>
                            setDispositionForm((current) => ({
                              ...current,
                              recipient_position_ids: [
                                ...current.recipient_position_ids,
                                position.id,
                              ],
                            }))
                          }
                          disabled={savingDisposition}
                          className="block w-full border-b border-zinc-200 px-3 py-2 text-left text-sm last:border-b-0 hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-800 dark:hover:bg-zinc-950/60"
                        >
                          <span className="font-semibold text-zinc-900 dark:text-zinc-100">
                            {position.title}
                          </span>
                          <span className="mt-0.5 block text-xs text-zinc-500">
                            {position.org_unit_name}
                            {position.holder_name ? ` · ${position.holder_name}` : ""}
                          </span>
                        </button>
                      ))
                    )}
                  </div>
                </div>

                <button
                  type="submit"
                  disabled={savingDisposition || myPositions.length === 0}
                  className="rounded-lg bg-navy-900 px-3 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-sky-500 dark:text-navy-950 dark:hover:bg-sky-400"
                >
                  {savingDisposition ? "Mengirim..." : "Kirim Disposisi"}
                </button>
                </form>
                </>
              )}
              </section>
            )}

            <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <h2 className="mb-3 text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                Timeline Approval
              </h2>
              <div className="grid gap-3">
                {letter.approval_steps.length === 0 && (
                  <p className="text-sm text-zinc-500">Belum ada approval step.</p>
                )}
                {letter.approval_steps.map((step) => (
                  <div
                    key={step.id}
                    className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                        Putaran {step.approval_cycle} · {step.step_order}.{" "}
                        {step.position_title}
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

            <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
              <div className="mb-3 flex items-center justify-between gap-3">
                <h2 className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                  Komentar
                </h2>
                {commentsMeta && (
                  <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                    {commentsMeta.total}
                  </span>
                )}
              </div>

              {commentsLoading && (
                <p className="text-sm text-zinc-500">Memuat komentar...</p>
              )}

              {!commentsLoading && commentsError && (
                <div
                  role="alert"
                  className="mb-3 grid gap-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  <p>{commentsError}</p>
                  <button
                    type="button"
                    onClick={() => void loadComments(commentsMeta?.page ?? 1)}
                    className="justify-self-start rounded-lg border border-red-300 px-3 py-1.5 text-xs font-semibold text-red-700 hover:bg-red-100 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-900"
                  >
                    Coba lagi
                  </button>
                </div>
              )}

              {!commentsLoading && !commentsError && (
                <div className="grid gap-3">
                  {comments.length === 0 && (
                    <p className="rounded-lg border border-dashed border-zinc-300 px-3 py-5 text-center text-sm text-zinc-500 dark:border-zinc-700">
                      Belum ada komentar.
                    </p>
                  )}
                  {comments.map((comment) => (
                    <div
                      key={comment.id}
                      className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800"
                    >
                      <div className="flex flex-wrap items-baseline justify-between gap-2">
                        <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                          {comment.user_name}
                        </p>
                        <span className="text-xs text-zinc-500">
                          {formatDate(comment.created_at)}
                        </span>
                      </div>
                      {comment.position_title && (
                        <p className="mt-0.5 text-xs text-zinc-500">
                          {comment.position_title}
                        </p>
                      )}
                      <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-zinc-700 dark:text-zinc-300">
                        {comment.body}
                      </p>
                    </div>
                  ))}
                  <Pagination
                    page={commentsMeta?.page ?? 1}
                    totalPages={commentsMeta?.total_pages ?? 1}
                    onPageChange={(page) => void loadComments(page)}
                    disabled={commentsLoading || postingComment}
                  />
                </div>
              )}

              <form onSubmit={handleCreateComment} className="mt-4 grid gap-2">
                {commentSubmitError && (
                  <p
                    role="alert"
                    className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                  >
                    {commentSubmitError}
                  </p>
                )}
                <label className="grid gap-1 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Tambah komentar
                  <textarea
                    value={commentBody}
                    onChange={(event) => setCommentBody(event.target.value)}
                    disabled={postingComment}
                    rows={3}
                    maxLength={COMMENT_MAX_LENGTH}
                    placeholder="Tulis komentar internal"
                    className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-900"
                  />
                </label>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-xs text-zinc-500">
                    {Array.from(commentBody).length}/{COMMENT_MAX_LENGTH}
                  </span>
                  <button
                    type="submit"
                    disabled={postingComment}
                    className="rounded-lg bg-navy-900 px-3 py-2 text-sm font-semibold text-white transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-sky-500 dark:text-navy-950 dark:hover:bg-sky-400"
                  >
                    {postingComment ? "Mengirim..." : "Kirim Komentar"}
                  </button>
                </div>
              </form>
            </section>
          </aside>
        </>
      )}
    </main>
  );
}
