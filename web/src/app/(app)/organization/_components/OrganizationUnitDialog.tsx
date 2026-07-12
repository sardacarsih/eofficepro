"use client";

import { useMemo, useState } from "react";
import { XIcon } from "@/components/layout/icons";
import type { Company, OrgUnit, OrgUnitPayload } from "@/lib/api";

const LEVELS: { value: OrgUnitPayload["unit_level"]; label: string }[] = [
  { value: "office", label: "Office" },
  { value: "directorate", label: "Direktorat" },
  { value: "biro", label: "Biro" },
  { value: "department", label: "Department" },
  { value: "division", label: "Division" },
];

const REGIONS: { value: NonNullable<OrgUnitPayload["region"]>; label: string }[] = [
  { value: "HO", label: "Head Office" },
  { value: "REG1", label: "Region 1" },
  { value: "REG2", label: "Region 2" },
  { value: "REPO_JKT", label: "Representative Office Jakarta" },
  { value: "REPO_PKB", label: "Representative Office Pekanbaru" },
];

interface OrganizationUnitDialogProps {
  unit: OrgUnit | null;
  suggestedParent: OrgUnit | null;
  companies: Company[];
  units: OrgUnit[];
  busy: boolean;
  error: string | null;
  onClose: () => void;
  onSave: (payload: OrgUnitPayload) => Promise<void>;
}

function descendantIDs(unit: OrgUnit | null): Set<string> {
  const ids = new Set<string>();
  function visit(current: OrgUnit) {
    current.children?.forEach((child) => {
      ids.add(child.id);
      visit(child);
    });
  }
  if (unit) visit(unit);
  return ids;
}

export default function OrganizationUnitDialog({
  unit,
  suggestedParent,
  companies,
  units,
  busy,
  error,
  onClose,
  onSave,
}: OrganizationUnitDialogProps) {
  const initialCompanyID = unit?.company_id ?? suggestedParent?.company_id ?? companies[0]?.id ?? "";
  const [companyID, setCompanyID] = useState(initialCompanyID);
  const [parentID, setParentID] = useState(unit?.parent_id ?? suggestedParent?.id ?? "");
  const [code, setCode] = useState(unit?.code ?? "");
  const [name, setName] = useState(unit?.name ?? "");
  const [unitLevel, setUnitLevel] = useState<OrgUnitPayload["unit_level"]>(
    (unit?.unit_level as OrgUnitPayload["unit_level"]) ?? "department",
  );
  const [region, setRegion] = useState<OrgUnitPayload["region"]>(
    (unit?.region as OrgUnitPayload["region"]) ?? null,
  );
  const [validationError, setValidationError] = useState<string | null>(null);

  const forbiddenParentIDs = useMemo(() => descendantIDs(unit), [unit]);
  const parentOptions = useMemo(
    () => units.filter(
      (candidate) =>
        candidate.company_id === companyID &&
        candidate.id !== unit?.id &&
        !forbiddenParentIDs.has(candidate.id),
    ),
    [companyID, forbiddenParentIDs, unit?.id, units],
  );

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    const payload: OrgUnitPayload = {
      company_id: companyID,
      parent_id: parentID || null,
      code: code.trim().toUpperCase(),
      name: name.trim(),
      unit_level: unitLevel,
      region,
    };
    if (!payload.company_id) return setValidationError("Perusahaan wajib dipilih.");
    if (!payload.code) return setValidationError("Kode unit wajib diisi.");
    if (!payload.name) return setValidationError("Nama unit wajib diisi.");
    setValidationError(null);
    await onSave(payload);
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/55 px-4 py-6 backdrop-blur-[2px]">
      <form
        role="dialog"
        aria-modal="true"
        aria-labelledby="org-unit-form-title"
        onSubmit={handleSubmit}
        className="w-full max-w-2xl overflow-hidden rounded-2xl border border-white/10 bg-white shadow-2xl dark:bg-zinc-900"
      >
        <header className="flex items-start justify-between border-b border-zinc-200 px-6 py-5 dark:border-zinc-800">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-sky-700 dark:text-sky-300">
              Company structure
            </p>
            <h2 id="org-unit-form-title" className="mt-1 text-lg font-semibold text-zinc-950 dark:text-zinc-50">
              {unit ? "Edit unit organisasi" : "Tambah unit organisasi"}
            </h2>
          </div>
          <button type="button" onClick={onClose} disabled={busy} aria-label="Tutup dialog" className="rounded-lg p-1.5 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800">
            <XIcon className="h-5 w-5" />
          </button>
        </header>

        <div className="grid gap-4 px-6 py-5 sm:grid-cols-2">
          {(validationError || error) && (
            <p role="alert" className="sm:col-span-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
              {validationError ?? error}
            </p>
          )}

          <label className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">
            Perusahaan
            <select
              value={companyID}
              disabled={Boolean(unit)}
              onChange={(event) => {
                setCompanyID(event.target.value);
                setParentID("");
              }}
              required
              className="mt-1.5 h-11 w-full rounded-lg border border-zinc-300 bg-white px-3 font-normal text-zinc-950 disabled:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50 dark:disabled:bg-zinc-800"
            >
              <option value="">Pilih perusahaan</option>
              {companies.map((company) => <option key={company.id} value={company.id}>[{company.code}] {company.name}</option>)}
            </select>
          </label>

          <label className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">
            Parent unit
            <select value={parentID} onChange={(event) => setParentID(event.target.value)} className="mt-1.5 h-11 w-full rounded-lg border border-zinc-300 bg-white px-3 font-normal text-zinc-950 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50">
              <option value="">Tanpa parent (root)</option>
              {parentOptions.map((parent) => <option key={parent.id} value={parent.id}>{parent.name} · {parent.code}</option>)}
            </select>
          </label>

          <label className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">
            Kode unit
            <input value={code} onChange={(event) => setCode(event.target.value.toUpperCase())} maxLength={30} required placeholder="Contoh: IT-SOFT" className="mt-1.5 h-11 w-full rounded-lg border border-zinc-300 bg-white px-3 font-mono font-normal uppercase text-zinc-950 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50" />
          </label>

          <label className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">
            Nama unit
            <input value={name} onChange={(event) => setName(event.target.value)} maxLength={150} required placeholder="Nama unit organisasi" className="mt-1.5 h-11 w-full rounded-lg border border-zinc-300 bg-white px-3 font-normal text-zinc-950 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50" />
          </label>

          <label className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">
            Level
            <select value={unitLevel} onChange={(event) => setUnitLevel(event.target.value as OrgUnitPayload["unit_level"])} className="mt-1.5 h-11 w-full rounded-lg border border-zinc-300 bg-white px-3 font-normal text-zinc-950 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50">
              {LEVELS.map((level) => <option key={level.value} value={level.value}>{level.label}</option>)}
            </select>
          </label>

          <label className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">
            Region
            <select value={region ?? ""} onChange={(event) => setRegion((event.target.value || null) as OrgUnitPayload["region"])} className="mt-1.5 h-11 w-full rounded-lg border border-zinc-300 bg-white px-3 font-normal text-zinc-950 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50">
              <option value="">Tanpa region</option>
              {REGIONS.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
            </select>
          </label>
        </div>

        <footer className="flex justify-end gap-3 border-t border-zinc-200 bg-zinc-50 px-6 py-4 dark:border-zinc-800 dark:bg-zinc-900">
          <button type="button" onClick={onClose} disabled={busy} className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-semibold text-zinc-700 dark:border-zinc-700 dark:text-zinc-200">Batal</button>
          <button type="submit" disabled={busy} className="rounded-lg bg-navy-700 px-4 py-2 text-sm font-semibold text-white hover:bg-navy-800 disabled:opacity-50">{busy ? "Menyimpan..." : "Simpan unit"}</button>
        </footer>
      </form>
    </div>
  );
}
