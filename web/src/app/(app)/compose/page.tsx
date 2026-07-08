"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import RichTextEditor from "@/components/RichTextEditor";
import {
  createDraftLetter,
  deleteDraftAttachment,
  getOrgTree,
  listCompanies,
  listDraftAttachments,
  listDraftLetters,
  listLetterTemplates,
  listLetterTypes,
  listMyLetters,
  listPositions,
  previewDraftLetter,
  submitDraftLetter,
  updateDraftLetter,
  uploadDraftAttachment,
  type Company,
  type DraftRecipient,
  type DraftLetter,
  type DraftLetterPayload,
  type LetterAttachment,
  type LetterTemplate,
  type LetterType,
  type OrgUnit,
  type Position,
  type User,
} from "@/lib/api";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";

type SaveState = "idle" | "saving" | "saved" | "error";

interface ComposerForm {
  id: string | null;
  company_id: string;
  letter_type_id: string;
  template_id: string;
  creator_position_id: string;
  subject: string;
  classification: LetterType["default_classification"];
  priority: DraftLetterPayload["priority"];
  body_html: string;
  recipients: DraftRecipient[];
  version: number;
}

type RecipientType = DraftRecipient["type"];
type RecipientTargetType = DraftRecipient["target_type"];

function emptyForm(
  companies: Company[],
  letterTypes: LetterType[],
  user: User | null,
): ComposerForm {
  const firstType = letterTypes[0];
  return {
    id: null,
    company_id: companies[0]?.id ?? "",
    letter_type_id: firstType?.id ?? "",
    template_id: "",
    creator_position_id: user?.positions?.[0]?.position_id ?? "",
    subject: "",
    classification: firstType?.default_classification ?? "biasa",
    priority: "normal",
    body_html: "",
    recipients: [],
    version: 0,
  };
}

function draftToForm(draft: DraftLetter): ComposerForm {
  return {
    id: draft.id,
    company_id: draft.company_id,
    letter_type_id: draft.letter_type_id,
    template_id: "",
    creator_position_id: draft.creator_position_id,
    subject: draft.subject,
    classification: draft.classification,
    priority: draft.priority,
    body_html: draft.body_html,
    recipients: draft.recipients,
    version: draft.version,
  };
}

function validForSave(form: ComposerForm): boolean {
  return Boolean(
    form.company_id &&
      form.letter_type_id &&
      form.creator_position_id &&
      form.subject.trim() &&
      form.recipients.some((recipient) => recipient.type === "to") &&
      form.body_html.replace(/<[^>]*>/g, "").trim(),
  );
}

function compactPayload(form: ComposerForm): DraftLetterPayload {
  if (!validForSave(form)) {
    throw new Error("Perusahaan, jenis, jabatan, perihal, dan isi surat wajib diisi");
  }

  return {
    company_id: form.company_id,
    letter_type_id: form.letter_type_id,
    creator_position_id: form.creator_position_id,
    subject: form.subject.trim(),
    classification: form.classification,
    priority: form.priority,
    body_html: form.body_html.trim(),
    recipients: form.recipients.map((recipient) => ({
      type: recipient.type,
      target_type: recipient.target_type,
      target_id: recipient.target_id,
    })),
  };
}

function flattenOrgUnits(units: OrgUnit[]): OrgUnit[] {
  return units.flatMap((unit) => [unit, ...flattenOrgUnits(unit.children ?? [])]);
}

function bodySkeletonForComposer(bodySkeleton: string): string {
  return bodySkeleton
    .replace(/<p>\s*Yth\.\s*\{\{tujuan\}\}\s*<\/p>\s*/i, "")
    .replace(/\{\{tujuan\}\}/gi, "")
    .trim();
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

const LETTER_STATUS_LABEL: Record<DraftLetter["status"], string> = {
  draft: "Draft",
  submitted: "Diajukan",
  in_approval: "Menunggu approval",
  revision: "Revisi",
  approved: "Disetujui",
  published: "Terbit",
  cancelled: "Dibatalkan",
  archived: "Arsip",
};

const LETTER_STATUS_STYLE: Record<DraftLetter["status"], string> = {
  draft: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  submitted: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  in_approval: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  revision: "bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-300",
  approved: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  published: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  cancelled: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
  archived: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
};

const MANAGER_OR_ABOVE_POSITION_TYPES = new Set([
  "dept_head",
  "gm",
  "director",
  "vp_director",
  "president_director",
]);

function isManagerOrAbove(positionType: string): boolean {
  return MANAGER_OR_ABOVE_POSITION_TYPES.has(positionType);
}

function directorateIDForOrgUnit(
  orgUnitID: string,
  orgUnitByID: Map<string, OrgUnit>,
): string | null {
  let current = orgUnitByID.get(orgUnitID);
  const visited = new Set<string>();

  while (current && !visited.has(current.id)) {
    if (current.unit_level === "directorate") return current.id;
    visited.add(current.id);
    current = current.parent_id ? orgUnitByID.get(current.parent_id) : undefined;
  }

  return null;
}

function positionDirectorateID(
  position: Position | undefined,
  orgUnitByID: Map<string, OrgUnit>,
): string | null {
  return position ? directorateIDForOrgUnit(position.org_unit_id, orgUnitByID) : null;
}

function recipientPolicyMessage(
  creatorPosition: Position | undefined,
  recipient: DraftRecipient,
  positions: Position[],
  orgUnitByID: Map<string, OrgUnit>,
): string | null {
  const creatorDirectorateID = positionDirectorateID(creatorPosition, orgUnitByID);
  const targetDirectorateID =
    recipient.target_type === "position"
      ? positionDirectorateID(
          positions.find((position) => position.id === recipient.target_id),
          orgUnitByID,
        )
      : directorateIDForOrgUnit(recipient.target_id, orgUnitByID);

  if (!creatorDirectorateID || !targetDirectorateID) return null;
  if (creatorDirectorateID === targetDirectorateID) return null;
  if (!creatorPosition || !isManagerOrAbove(creatorPosition.position_type)) {
    return "Surat lintas direktorat hanya dapat dibuat oleh level manager ke atas.";
  }
  if (recipient.target_type === "org_unit") {
    return "Penerima Unit lintas direktorat tidak diizinkan; pilih Jabatan tujuan.";
  }
  return null;
}

export default function ComposePage() {
  const me = useCurrentUser();
  const [companies, setCompanies] = useState<Company[]>([]);
  const [letterTypes, setLetterTypes] = useState<LetterType[]>([]);
  const [templates, setTemplates] = useState<LetterTemplate[]>([]);
  const [recipientPositions, setRecipientPositions] = useState<Position[]>([]);
  const [recipientOrgUnits, setRecipientOrgUnits] = useState<OrgUnit[]>([]);
  const [drafts, setDrafts] = useState<DraftLetter[]>([]);
  const [myLetters, setMyLetters] = useState<DraftLetter[]>([]);
  const [form, setForm] = useState<ComposerForm | null>(null);
  const [attachments, setAttachments] = useState<LetterAttachment[]>([]);
  const [recipientType, setRecipientType] = useState<RecipientType>("to");
  const [recipientTargetType, setRecipientTargetType] =
    useState<RecipientTargetType>("position");
  const [recipientTargetID, setRecipientTargetID] = useState("");
  const [loading, setLoading] = useState(true);
  const [dirty, setDirty] = useState(false);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [uploadingAttachment, setUploadingAttachment] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reloadDrafts = useCallback(async () => {
    const data = await listDraftLetters();
    setDrafts(data.letters);
  }, []);

  const reloadMyLetters = useCallback(async () => {
    const data = await listMyLetters();
    setMyLetters(data.letters);
  }, []);

  const reloadAttachments = useCallback(async (draftID: string) => {
    const data = await listDraftAttachments(draftID);
    setAttachments(data.attachments);
  }, []);

  // Tunggu profil dari layout (app) agar posisi pembuat surat langsung terisi.
  useEffect(() => {
    if (!me) return;

    Promise.all([
      listCompanies(),
      listPositions(),
      getOrgTree(),
      listLetterTypes(false),
      listLetterTemplates(false),
      listDraftLetters(),
      listMyLetters(),
    ])
      .then(
        ([
          companyData,
          positionData,
          orgData,
          typeData,
          templateData,
          draftData,
          myLetterData,
        ]) => {
        setCompanies(companyData.companies);
        setRecipientPositions(positionData.positions);
        setRecipientOrgUnits(flattenOrgUnits(orgData.tree));
        setLetterTypes(typeData.letter_types);
        setTemplates(templateData.letter_templates);
        setDrafts(draftData.letters);
        setMyLetters(myLetterData.letters);
        setForm(emptyForm(companyData.companies, typeData.letter_types, me));
        setRecipientTargetID(positionData.positions[0]?.id ?? "");
        },
      )
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat composer"),
      )
      .finally(() => setLoading(false));
  }, [me]);

  const matchingTemplates = useMemo(() => {
    if (!form) return [];
    return templates.filter(
      (template) =>
        template.company_id === form.company_id &&
        template.letter_type_id === form.letter_type_id,
    );
  }, [form, templates]);

  const orgUnitByID = useMemo(() => {
    const byID = new Map<string, OrgUnit>();
    recipientOrgUnits.forEach((unit) => byID.set(unit.id, unit));
    return byID;
  }, [recipientOrgUnits]);

  const selectedCreatorPosition = useMemo(
    () => recipientPositions.find((position) => position.id === form?.creator_position_id),
    [form?.creator_position_id, recipientPositions],
  );

  const creatorDirectorateID = useMemo(
    () => positionDirectorateID(selectedCreatorPosition, orgUnitByID),
    [orgUnitByID, selectedCreatorPosition],
  );

  const creatorCanSendCrossDirectorate = Boolean(
    selectedCreatorPosition && isManagerOrAbove(selectedCreatorPosition.position_type),
  );

  const filteredRecipientPositions = useMemo(() => {
    if (!creatorDirectorateID || creatorCanSendCrossDirectorate) return recipientPositions;
    return recipientPositions.filter(
      (position) => positionDirectorateID(position, orgUnitByID) === creatorDirectorateID,
    );
  }, [creatorCanSendCrossDirectorate, creatorDirectorateID, orgUnitByID, recipientPositions]);

  const filteredRecipientOrgUnits = useMemo(() => {
    if (!creatorDirectorateID) return recipientOrgUnits;
    return recipientOrgUnits.filter(
      (unit) => directorateIDForOrgUnit(unit.id, orgUnitByID) === creatorDirectorateID,
    );
  }, [creatorDirectorateID, orgUnitByID, recipientOrgUnits]);

  const recipientPolicyErrors = useMemo(() => {
    if (!form) return [];
    return form.recipients
      .map((recipient) => ({
        key: `${recipient.type}-${recipient.target_type}-${recipient.target_id}`,
        label: recipient.label,
        message: recipientPolicyMessage(
          selectedCreatorPosition,
          recipient,
          recipientPositions,
          orgUnitByID,
        ),
      }))
      .filter((item): item is { key: string; label: string; message: string } =>
        Boolean(item.message),
      );
  }, [form, orgUnitByID, recipientPositions, selectedCreatorPosition]);

  const recipientOptions =
    recipientTargetType === "position" ? filteredRecipientPositions : filteredRecipientOrgUnits;
  const selectedRecipientTargetID = recipientOptions.some(
    (option) => option.id === recipientTargetID,
  )
    ? recipientTargetID
    : recipientOptions[0]?.id ?? "";

  const canSaveDraft = Boolean(
    form && validForSave(form) && recipientPolicyErrors.length === 0,
  );
  const canUploadAttachment = canSaveDraft && !uploadingAttachment;

  const activeError = error ?? recipientPolicyErrors[0]?.message ?? null;
  const submittedLetters = myLetters
    .filter((letter) => letter.status !== "draft" && letter.status !== "revision")
    .slice(0, 8);

  function updateForm(patch: Partial<ComposerForm>) {
    setForm((current) => (current ? { ...current, ...patch } : current));
    setDirty(true);
    setSaveState("idle");
    setSuccess(null);
  }

  function selectLetterType(letterTypeID: string) {
    const letterType = letterTypes.find((item) => item.id === letterTypeID);
    updateForm({
      letter_type_id: letterTypeID,
      template_id: "",
      classification: letterType?.default_classification ?? "biasa",
    });
  }

  function applyTemplate(templateID: string) {
    const template = templates.find((item) => item.id === templateID);
    if (!template) {
      updateForm({ template_id: templateID });
      return;
    }
    updateForm({
      template_id: templateID,
      body_html: bodySkeletonForComposer(template.body_skeleton),
    });
  }

  function changeRecipientTargetType(targetType: RecipientTargetType) {
    setRecipientTargetType(targetType);
  }

  function addRecipient() {
    if (!form || !selectedRecipientTargetID) return;

    const label =
      recipientTargetType === "position"
        ? (() => {
            const position = recipientPositions.find(
              (item) => item.id === selectedRecipientTargetID,
            );
            return position ? `${position.title} - ${position.org_unit_name}` : "";
          })()
        : recipientOrgUnits.find((unit) => unit.id === selectedRecipientTargetID)?.name ?? "";
    if (!label) return;

    const exists = form.recipients.some(
      (recipient) =>
        recipient.type === recipientType &&
        recipient.target_type === recipientTargetType &&
        recipient.target_id === selectedRecipientTargetID,
    );
    if (exists) return;

    updateForm({
      recipients: [
        ...form.recipients,
        {
          type: recipientType,
          target_type: recipientTargetType,
          target_id: selectedRecipientTargetID,
          label,
        },
      ],
    });
  }

  function removeRecipient(recipient: DraftRecipient) {
    if (!form) return;
    updateForm({
      recipients: form.recipients.filter(
        (item) =>
          !(
            item.type === recipient.type &&
            item.target_type === recipient.target_type &&
            item.target_id === recipient.target_id
          ),
      ),
    });
  }

  function openDraft(draft: DraftLetter) {
    setForm(draftToForm(draft));
    setAttachments([]);
    setDirty(false);
    setSaveState("idle");
    setLastSavedAt(new Date(draft.updated_at).toLocaleTimeString("id-ID"));
    setSuccess(null);
    setError(null);
    void reloadAttachments(draft.id).catch((err) =>
      setError(err instanceof Error ? err.message : "Gagal memuat lampiran"),
    );
  }

  function newDraft() {
    if (!canCompose || creatorPositions.length === 0) {
      setSuccess(null);
      setError(
        !canCompose
          ? "Akun ini belum memiliki role pembuat surat."
          : "Akun ini belum ditempatkan ke jabatan aktif.",
      );
      return;
    }

    setForm(emptyForm(companies, letterTypes, me));
    setAttachments([]);
    setDirty(false);
    setSaveState("idle");
    setLastSavedAt(null);
    setSuccess("Draft baru siap diisi. Draft akan dibuat saat data wajib lengkap dan disimpan.");
    setError(null);
  }

  const saveDraft = useCallback(async (mode: "manual" | "auto"): Promise<string | null> => {
    if (!form) return null;
    if (recipientPolicyErrors.length > 0) {
      setSaveState("error");
      if (mode === "manual") setError(recipientPolicyErrors[0].message);
      return null;
    }
    setSaveState("saving");
    setError(null);
    setSuccess(null);
    try {
      const payload = compactPayload(form);
      const result = form.id
        ? await updateDraftLetter(form.id, payload)
        : await createDraftLetter(payload);

      setForm((current) =>
        current
          ? { ...current, id: result.id, version: result.version }
          : current,
      );
      setDirty(false);
      setSaveState("saved");
      setLastSavedAt(new Date().toLocaleTimeString("id-ID"));
      await Promise.all([reloadDrafts(), reloadMyLetters()]);
      return result.id;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Gagal menyimpan draft";
      setSaveState("error");
      if (mode === "manual") setError(message);
      return null;
    }
  }, [form, recipientPolicyErrors, reloadDrafts, reloadMyLetters]);

  useEffect(() => {
    if (!dirty || !form || !validForSave(form) || recipientPolicyErrors.length > 0) return;
    const timer = window.setTimeout(() => {
      void saveDraft("auto");
    }, 30000);
    return () => window.clearTimeout(timer);
  }, [dirty, form, recipientPolicyErrors.length, saveDraft]);

  async function ensureDraftSaved(): Promise<string | null> {
    if (!form) return null;
    if (form.id && !dirty) return form.id;
    return saveDraft("manual");
  }

  async function handleAttachmentSelected(file: File | null) {
    if (!file) return;
    setUploadingAttachment(true);
    setError(null);
    setSuccess(null);
    try {
      const draftID = await ensureDraftSaved();
      if (!draftID) return;
      await uploadDraftAttachment(draftID, file);
      await reloadAttachments(draftID);
      setSuccess("Lampiran berhasil diunggah.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengunggah lampiran");
    } finally {
      setUploadingAttachment(false);
    }
  }

  async function handleDeleteAttachment(attachmentID: string) {
    if (!form?.id) return;
    setError(null);
    setSuccess(null);
    try {
      await deleteDraftAttachment(form.id, attachmentID);
      await reloadAttachments(form.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menghapus lampiran");
    }
  }

  async function handlePreview() {
    setPreviewing(true);
    setError(null);
    setSuccess(null);
    try {
      const draftID = await ensureDraftSaved();
      if (!draftID) return;
      const result = await previewDraftLetter(draftID);
      window.open(result.preview_url, "_blank", "noopener,noreferrer");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal membuat preview PDF");
    } finally {
      setPreviewing(false);
    }
  }

  async function handleSubmit() {
    setSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      const draftID = await ensureDraftSaved();
      if (!draftID) return;
      const result = await submitDraftLetter(draftID);
      await Promise.all([reloadDrafts(), reloadMyLetters()]);
      setAttachments([]);
      setForm(emptyForm(companies, letterTypes, me));
      setDirty(false);
      setSaveState("idle");
      setLastSavedAt(null);
      setSuccess(
        `Surat diajukan ke approval (${result.approval_steps.length} step). URL verifikasi: ${result.verify_url}`,
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengajukan surat");
    } finally {
      setSubmitting(false);
    }
  }

  const canCompose =
    me?.roles.some((role) => ["admin", "creator", "secretary"].includes(role)) ?? false;
  const creatorPositions = me?.positions ?? [];
  const canStartNewDraft = !loading && canCompose && creatorPositions.length > 0;

  return (
    <main className="mx-auto grid w-full max-w-7xl flex-1 gap-6 px-6 py-8 lg:grid-cols-[320px_1fr]">
        <aside className="rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <div className="flex items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
            <div>
              <h2 className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                Draft Saya
              </h2>
              <p className="text-xs text-zinc-500">{drafts.length} draft aktif</p>
            </div>
            <button
              onClick={newDraft}
              disabled={!canStartNewDraft}
              title={
                canStartNewDraft
                  ? "Mulai draft baru"
                  : "Akun harus memiliki role pembuat surat dan jabatan aktif"
              }
              className="rounded-lg bg-emerald-700 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-500 dark:disabled:bg-zinc-800 dark:disabled:text-zinc-500"
            >
              Baru
            </button>
          </div>
          <div className="max-h-[calc(100vh-180px)] overflow-y-auto p-2">
            {loading && <p className="px-2 py-4 text-sm text-zinc-500">Memuat...</p>}
            {!loading && drafts.length === 0 && (
              <p className="px-2 py-4 text-sm text-zinc-500">Belum ada draft.</p>
            )}
            {drafts.map((draft) => (
              <button
                key={draft.id}
                onClick={() => openDraft(draft)}
                className={`mb-2 w-full rounded-lg border px-3 py-2 text-left transition ${
                  form?.id === draft.id
                    ? "border-emerald-300 bg-emerald-50 dark:border-emerald-900 dark:bg-emerald-950/40"
                    : "border-zinc-200 bg-white hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:bg-zinc-800"
                }`}
              >
                <div className="line-clamp-2 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                  {draft.subject}
                </div>
                <div className="mt-1 flex items-center justify-between gap-2 text-xs text-zinc-500">
                  <span>{draft.letter_type_code} v{draft.version}</span>
                  <span>{new Date(draft.updated_at).toLocaleDateString("id-ID")}</span>
                </div>
              </button>
            ))}
          </div>
          <div className="border-t border-zinc-200 p-4 dark:border-zinc-800">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <h2 className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                  Pengajuan Saya
                </h2>
                <p className="text-xs text-zinc-500">
                  {submittedLetters.length} surat terakhir
                </p>
              </div>
            </div>
            <div className="grid gap-2">
              {loading && <p className="text-sm text-zinc-500">Memuat...</p>}
              {!loading && submittedLetters.length === 0 && (
                <p className="rounded-lg border border-dashed border-zinc-300 px-3 py-3 text-xs text-zinc-500 dark:border-zinc-700">
                  Belum ada surat yang diajukan.
                </p>
              )}
              {submittedLetters.map((letter) => (
                <Link
                  key={letter.id}
                  href={`/letters/${letter.id}`}
                  className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 transition hover:bg-white dark:border-zinc-800 dark:bg-zinc-950/40 dark:hover:bg-zinc-900"
                >
                  <div className="line-clamp-2 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                    {letter.subject}
                  </div>
                  {letter.letter_number && (
                    <div className="mt-1 font-mono text-xs text-emerald-700 dark:text-emerald-300">
                      {letter.letter_number}
                    </div>
                  )}
                  <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-zinc-500">
                    <span>
                      {letter.letter_type_code} v{letter.version}
                    </span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                        LETTER_STATUS_STYLE[letter.status]
                      }`}
                    >
                      {LETTER_STATUS_LABEL[letter.status]}
                    </span>
                  </div>
                  <div className="mt-1 text-xs text-zinc-400">
                    {new Date(letter.updated_at).toLocaleDateString("id-ID")}
                  </div>
                </Link>
              ))}
            </div>
          </div>
        </aside>

        <section className="rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
            <div>
              <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
                Tulis Surat
              </h1>
              <p className="text-sm text-zinc-500">
                {form?.id ? `Draft v${form.version}` : "Draft baru belum tersimpan"} · autosave 30 detik
              </p>
            </div>
            <div className="flex items-center gap-3">
              {saveState !== "idle" && (
                <span className="text-xs text-zinc-500">
                  {saveState === "saving" && "Menyimpan..."}
                  {saveState === "saved" && `Tersimpan ${lastSavedAt ?? ""}`}
                  {saveState === "error" && "Gagal autosave"}
                </span>
              )}
              <button
                onClick={() => void saveDraft("manual")}
                disabled={!canSaveDraft || saveState === "saving"}
                className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Simpan Draft
              </button>
              <button
                onClick={() => void handlePreview()}
                disabled={!canSaveDraft || saveState === "saving" || previewing}
                className="rounded-lg border border-emerald-700 px-4 py-2 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-emerald-500 dark:text-emerald-300 dark:hover:bg-emerald-950"
              >
                {previewing ? "Membuat PDF..." : "Preview PDF"}
              </button>
              <button
                onClick={() => void handleSubmit()}
                disabled={!canSaveDraft || saveState === "saving" || submitting}
                className="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {submitting ? "Mengajukan..." : "Ajukan"}
              </button>
            </div>
          </div>

          <div className="p-5">
            {success && (
              <p className="mb-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
                {success}
              </p>
            )}
            {activeError && (
              <p
                role="alert"
                className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
              >
                {activeError}
              </p>
            )}
            {!loading && !canCompose && (
              <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
                Akun ini belum memiliki role pembuat surat.
              </p>
            )}
            {!loading && canCompose && creatorPositions.length === 0 && (
              <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
                Akun ini belum ditempatkan ke jabatan aktif.
              </p>
            )}
            {form && canCompose && creatorPositions.length > 0 && (
              <div className="grid gap-4">
                <div className="grid gap-4 md:grid-cols-4">
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Perusahaan
                    <select
                      value={form.company_id}
                      onChange={(e) =>
                        updateForm({ company_id: e.target.value, template_id: "" })
                      }
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    >
                      {companies.map((company) => (
                        <option key={company.id} value={company.id}>
                          {company.code}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Jenis Surat
                    <select
                      value={form.letter_type_id}
                      onChange={(e) => selectLetterType(e.target.value)}
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    >
                      {letterTypes.map((letterType) => (
                        <option key={letterType.id} value={letterType.id}>
                          {letterType.code} - {letterType.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 md:col-span-2">
                    Jabatan Pembuat
                    <select
                      value={form.creator_position_id}
                      onChange={(e) => updateForm({ creator_position_id: e.target.value })}
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    >
                      {creatorPositions.map((position) => (
                        <option key={position.position_id} value={position.position_id}>
                          {position.title} · {position.org_unit}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 md:col-span-2">
                    Template
                    <select
                      value={form.template_id}
                      onChange={(e) => applyTemplate(e.target.value)}
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    >
                      <option value="">Tanpa template</option>
                      {matchingTemplates.map((template) => (
                        <option key={template.id} value={template.id}>
                          {template.letter_type_code} v{template.version} · {template.company_code}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Klasifikasi
                    <select
                      value={form.classification}
                      onChange={(e) =>
                        updateForm({
                          classification: e.target
                            .value as LetterType["default_classification"],
                        })
                      }
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    >
                      <option value="biasa">Biasa</option>
                      <option value="terbatas">Terbatas</option>
                      <option value="rahasia">Rahasia</option>
                    </select>
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Prioritas
                    <select
                      value={form.priority}
                      onChange={(e) =>
                        updateForm({ priority: e.target.value as DraftLetterPayload["priority"] })
                      }
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    >
                      <option value="normal">Normal</option>
                      <option value="urgent">Urgent</option>
                    </select>
                  </label>
                </div>

                <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Penerima
                  <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950/40">
                    <div className="grid gap-2 md:grid-cols-[120px_150px_1fr_auto]">
                      <select
                        value={recipientType}
                        onChange={(e) => setRecipientType(e.target.value as RecipientType)}
                        className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                      >
                        <option value="to">To</option>
                        <option value="cc">CC</option>
                      </select>
                      <select
                        value={recipientTargetType}
                        onChange={(e) =>
                          changeRecipientTargetType(e.target.value as RecipientTargetType)
                        }
                        className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                      >
                        <option value="position">Jabatan</option>
                        <option value="org_unit">Unit</option>
                      </select>
                      <select
                        value={selectedRecipientTargetID}
                        onChange={(e) => setRecipientTargetID(e.target.value)}
                        className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                      >
                        {recipientOptions.map((option) => (
                          <option key={option.id} value={option.id}>
                            {"title" in option
                              ? `${option.title} · ${option.org_unit_name}`
                              : `${option.name} · ${option.code}`}
                          </option>
                        ))}
                      </select>
                      <button
                        type="button"
                        onClick={addRecipient}
                        disabled={!selectedRecipientTargetID}
                        className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-semibold text-white transition hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        Tambah
                      </button>
                    </div>

                    {recipientPolicyErrors.length > 0 && (
                      <div className="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-normal text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
                        {recipientPolicyErrors.map((item) => (
                          <p key={item.key}>
                            {item.label}: {item.message}
                          </p>
                        ))}
                      </div>
                    )}

                    <div className="mt-3 grid gap-3 md:grid-cols-2">
                      {(["to", "cc"] as const).map((type) => (
                        <div key={type}>
                          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-zinc-500">
                            {type === "to" ? "To" : "CC"}
                          </p>
                          <div className="flex min-h-10 flex-wrap gap-2">
                            {form.recipients
                              .filter((recipient) => recipient.type === type)
                              .map((recipient) => (
                                <button
                                  key={`${recipient.type}-${recipient.target_type}-${recipient.target_id}`}
                                  type="button"
                                  onClick={() => removeRecipient(recipient)}
                                  className="rounded-full border border-zinc-300 bg-white px-3 py-1 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
                                >
                                  {recipient.label} x
                                </button>
                              ))}
                            {form.recipients.filter((recipient) => recipient.type === type)
                              .length === 0 && (
                              <span className="text-xs font-normal text-zinc-400">
                                Belum ada penerima
                              </span>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </label>

                <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Perihal
                  <input
                    value={form.subject}
                    onChange={(e) => updateForm({ subject: e.target.value })}
                    maxLength={255}
                    className="h-11 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                  />
                </label>

                <div className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Lampiran
                  <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950/40">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="grid gap-1 text-xs font-normal text-zinc-500">
                        <span>
                          Maksimal 25 MB per file. PDF, Word, Excel, CSV, PNG, dan JPG.
                        </span>
                        {!canSaveDraft && (
                          <span className="font-semibold text-amber-700 dark:text-amber-300">
                            Isi perusahaan, jenis, jabatan, perihal, penerima, dan isi surat sebelum upload lampiran.
                          </span>
                        )}
                      </div>
                      <label
                        aria-disabled={!canUploadAttachment}
                        className={`inline-flex items-center rounded-lg px-3 py-2 text-xs font-semibold transition ${
                          canUploadAttachment
                            ? "cursor-pointer bg-emerald-700 text-white hover:bg-emerald-800"
                            : "cursor-not-allowed border border-zinc-300 bg-zinc-100 text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-500"
                        }`}
                      >
                        {uploadingAttachment ? "Mengunggah..." : "Tambah Lampiran"}
                        <input
                          type="file"
                          className="hidden"
                          disabled={!canUploadAttachment}
                          accept=".pdf,.docx,.xlsx,.xls,.csv,.png,.jpg,.jpeg"
                          onChange={(e) => {
                            const file = e.target.files?.[0] ?? null;
                            e.currentTarget.value = "";
                            void handleAttachmentSelected(file);
                          }}
                        />
                      </label>
                    </div>

                    <div className="mt-3 grid gap-2">
                      {attachments.length === 0 && (
                        <p className="rounded-lg border border-dashed border-zinc-300 px-3 py-3 text-xs font-normal text-zinc-500 dark:border-zinc-700">
                          Belum ada lampiran.
                        </p>
                      )}
                      {attachments.map((attachment) => (
                        <div
                          key={attachment.id}
                          className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-white px-3 py-2 dark:border-zinc-800 dark:bg-zinc-900"
                        >
                          <div className="min-w-0">
                            <p className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                              {attachment.file_name}
                            </p>
                            <p className="text-xs font-normal text-zinc-500">
                              {formatBytes(attachment.size_bytes)} · scan {attachment.scan_status}
                            </p>
                          </div>
                          <div className="flex items-center gap-2 text-xs">
                            {attachment.download_url && (
                              <a
                                href={attachment.download_url}
                                target="_blank"
                                rel="noreferrer"
                                className="rounded-lg border border-zinc-300 px-3 py-1.5 font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                              >
                                Buka
                              </a>
                            )}
                            <button
                              type="button"
                              onClick={() => void handleDeleteAttachment(attachment.id)}
                              className="rounded-lg border border-red-200 px-3 py-1.5 font-semibold text-red-700 hover:bg-red-50 dark:border-red-900 dark:text-red-300 dark:hover:bg-red-950"
                            >
                              Hapus
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>

                <div className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                  Isi Surat
                  <RichTextEditor
                    value={form.body_html}
                    onChange={(bodyHTML) => updateForm({ body_html: bodyHTML })}
                  />
                </div>
              </div>
            )}
          </div>
        </section>
    </main>
  );
}
