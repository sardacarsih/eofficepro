"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  activatePosition,
  createPosition,
  deactivatePosition,
  getOrgTree,
  getPositionDeactivationImpact,
  listPositions,
  updatePosition,
  type OrgUnit,
  type PageMeta,
  type Position,
  type PositionDeactivationImpact,
  type PositionPayload,
} from "@/lib/api";
import Pagination from "@/components/Pagination";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import {
  DEFAULT_APPROVER_POSITION_TYPES,
  MASTER_POSITION_TYPES_BY_UNIT_LEVEL,
  POSITION_TYPE_LABEL,
} from "@/lib/position-types";
import {
  BriefcaseIcon,
  EditIcon,
  PlusIcon,
  SearchIcon,
  XIcon,
} from "@/components/layout/icons";

const UNIT_LEVEL_OPTIONS = [
  { value: "office", label: "Office" },
  { value: "directorate", label: "Direktorat" },
  { value: "biro", label: "Biro" },
  { value: "department", label: "Department" },
  { value: "division", label: "Division" },
] as const;

const UNIT_LEVEL_LABEL: Record<string, string> = Object.fromEntries(
  UNIT_LEVEL_OPTIONS.map((option) => [option.value, option.label]),
);

const IMPACT_LABELS: Array<{
  key: keyof Omit<PositionDeactivationImpact, "can_deactivate">;
  label: string;
}> = [
  { key: "active_assignments", label: "Penempatan pengguna aktif" },
  { key: "active_subordinates", label: "Jabatan bawahan aktif" },
  { key: "active_delegations", label: "Delegasi aktif" },
  { key: "active_drafts", label: "Draft atau revisi aktif" },
  { key: "active_approvals", label: "Tahap approval berjalan" },
  { key: "active_dispositions", label: "Disposisi aktif" },
];

interface PositionFormState {
  unit_level: string;
  org_unit_id: string;
  position_type: string;
  title: string;
  reports_to: string;
  is_approver: boolean;
}

interface DeactivationDialog {
  position: Position;
  impact: PositionDeactivationImpact;
}

function emptyForm(): PositionFormState {
  return {
    unit_level: "",
    org_unit_id: "",
    position_type: "",
    title: "",
    reports_to: "",
    is_approver: false,
  };
}

function positionToForm(position: Position): PositionFormState {
  return {
    unit_level: position.org_unit_level,
    org_unit_id: position.org_unit_id,
    position_type: position.position_type,
    title: position.title,
    reports_to: position.reports_to ?? "",
    is_approver: position.is_approver,
  };
}

function flattenOrgUnits(units: OrgUnit[]): OrgUnit[] {
  return units.flatMap((unit) => [unit, ...flattenOrgUnits(unit.children ?? [])]);
}

function unitLabel(unit: OrgUnit): string {
  const level = UNIT_LEVEL_LABEL[unit.unit_level] ?? unit.unit_level;
  return `${unit.name} · ${level}${unit.region ? ` · ${unit.region}` : ""}`;
}

function positionTitleSuggestion(positionType: string, unit?: OrgUnit): string {
  if (!positionType || !unit) return "";
  const typeLabel = POSITION_TYPE_LABEL[positionType] ?? positionType;
  const prefixes: Record<string, string> = {
    office: "",
    directorate: "Directorate ",
    biro: "Biro ",
    department: "Department ",
    division: "Division ",
  };
  const prefix = prefixes[unit.unit_level] ?? "";
  const suffix = prefix && unit.name.startsWith(prefix)
    ? unit.name.slice(prefix.length)
    : unit.name;
  if (unit.name.toLowerCase() === typeLabel.toLowerCase()) return typeLabel;
  return `${typeLabel} ${suffix}`.trim();
}

function ancestorUnits(unit: OrgUnit, unitsByID: Map<string, OrgUnit>): OrgUnit[] {
  const ancestors: OrgUnit[] = [];
  let parentID = unit.parent_id;
  while (parentID) {
    const parent = unitsByID.get(parentID);
    if (!parent) break;
    ancestors.push(parent);
    parentID = parent.parent_id;
  }
  return ancestors;
}

function suggestManagerID(
  positionType: string,
  unit: OrgUnit | undefined,
  positions: Position[],
  unitsByID: Map<string, OrgUnit>,
  editingID?: string,
): string {
  if (!unit || positionType === "president_director") return "";
  const active = positions.filter(
    (position) => position.is_active && position.id !== editingID,
  );
  const atUnit = (unitID: string, types: string[]) => {
    for (const positionType of types) {
      const match = active.find(
        (position) =>
          position.org_unit_id === unitID &&
          position.position_type === positionType,
      );
      if (match) return match.id;
    }
    return "";
  };
  const atAncestors = (types: string[]) => {
    for (const ancestor of ancestorUnits(unit, unitsByID)) {
      const match = atUnit(ancestor.id, types);
      if (match) return match;
    }
    return "";
  };
  const global = (types: string[]) => {
    for (const positionType of types) {
      const match = active.find(
        (position) => position.position_type === positionType,
      );
      if (match) return match.id;
    }
    return "";
  };

  switch (positionType) {
    case "vp_director":
      return global(["president_director"]);
    case "director":
      return global(["vp_director", "president_director"]);
    case "auditor":
      return global(["president_director"]);
    case "gm":
      return atAncestors(["director"]) || global(["director"]);
    case "dept_head":
      return atAncestors(["gm", "director"]);
    case "sub_dept_head":
      return atUnit(unit.id, ["dept_head"]);
    case "division_head":
      return atAncestors(["sub_dept_head", "dept_head"]);
    case "secretary":
      return atUnit(unit.id, ["director", "gm", "auditor"]);
    case "assistant":
    case "staff": {
      const preferredByLevel: Record<string, string[]> = {
        office: ["president_director", "vp_director"],
        directorate: ["director", "auditor"],
        biro: ["gm"],
        department: ["sub_dept_head", "dept_head"],
        division: ["division_head"],
      };
      return (
        atUnit(unit.id, preferredByLevel[unit.unit_level] ?? []) ||
        atAncestors([
          "division_head",
          "sub_dept_head",
          "dept_head",
          "gm",
          "director",
          "vp_director",
          "president_director",
        ])
      );
    }
    default:
      return "";
  }
}

function subordinatePositionIDs(positions: Position[], rootID: string): Set<string> {
  const children = new Map<string, string[]>();
  positions.forEach((position) => {
    if (!position.reports_to) return;
    const ids = children.get(position.reports_to) ?? [];
    ids.push(position.id);
    children.set(position.reports_to, ids);
  });
  const result = new Set<string>();
  const pending = [...(children.get(rootID) ?? [])];
  while (pending.length > 0) {
    const id = pending.pop();
    if (!id || result.has(id)) continue;
    result.add(id);
    pending.push(...(children.get(id) ?? []));
  }
  return result;
}

function compactPayload(form: PositionFormState): PositionPayload {
  return {
    org_unit_id: form.org_unit_id,
    title: form.title.trim(),
    position_type: form.position_type,
    reports_to: form.reports_to || null,
    is_approver: form.is_approver,
  };
}

function SummaryTile({
  label,
  value,
  detail,
}: {
  label: string;
  value: number;
  detail: string;
}) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white px-4 py-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <p className="text-xs font-semibold uppercase text-zinc-500">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-zinc-950 dark:text-zinc-50">
        {value}
      </p>
      <p className="text-xs text-zinc-500">{detail}</p>
    </div>
  );
}

export default function PositionsPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const [positions, setPositions] = useState<Position[]>([]);
  const [page, setPage] = useState(1);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [orgUnits, setOrgUnits] = useState<OrgUnit[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [actionID, setActionID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [modalError, setModalError] = useState<string | null>(null);
  const [editing, setEditing] = useState<Position | null>(null);
  const [form, setForm] = useState<PositionFormState | null>(null);
  const [query, setQuery] = useState("");
  const [levelFilter, setLevelFilter] = useState("");
  const [unitFilter, setUnitFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "inactive">(
    "active",
  );
  const [deactivationDialog, setDeactivationDialog] =
    useState<DeactivationDialog | null>(null);

  const unitsByID = useMemo(
    () => new Map(orgUnits.map((unit) => [unit.id, unit])),
    [orgUnits],
  );

  async function reload() {
    const [positionData, orgData] = await Promise.all([
      listPositions({ includeInactive: true, page }),
      getOrgTree(),
    ]);
    setPositions(positionData.data);
    setMeta(positionData.meta);
    setOrgUnits(flattenOrgUnits(orgData.tree));
  }

  useEffect(() => {
    if (me && !me.roles.includes("admin")) {
      router.replace("/organization");
    }
  }, [me, router]);

  useEffect(() => {
    queueMicrotask(() => setLoading(true));
    Promise.all([listPositions({ includeInactive: true, page }), getOrgTree()])
      .then(([positionData, orgData]) => {
        setPositions(positionData.data);
        setMeta(positionData.meta);
        setOrgUnits(flattenOrgUnits(orgData.tree));
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Gagal memuat master jabatan");
      })
      .finally(() => setLoading(false));
  }, [page]);

  const activePositions = positions.filter((position) => position.is_active);
  const occupiedCount = activePositions.filter((position) => position.holder_name).length;
  const vacantCount = activePositions.length - occupiedCount;

  const filterUnitOptions = orgUnits
    .filter((unit) => !levelFilter || unit.unit_level === levelFilter)
    .sort((a, b) => a.name.localeCompare(b.name));

  const normalizedQuery = query.trim().toLowerCase();
  const filteredPositions = positions.filter((position) => {
    const queryOK =
      !normalizedQuery ||
      [
        position.title,
        position.org_unit_name,
        position.position_type,
        POSITION_TYPE_LABEL[position.position_type],
        position.holder_name,
        position.reports_to_title,
      ]
        .join(" ")
        .toLowerCase()
        .includes(normalizedQuery);
    const levelOK = !levelFilter || position.org_unit_level === levelFilter;
    const unitOK = !unitFilter || position.org_unit_id === unitFilter;
    const statusOK =
      statusFilter === "all" ||
      (statusFilter === "active" ? position.is_active : !position.is_active);
    return queryOK && levelOK && unitOK && statusOK;
  });

  const formUnitOptions = orgUnits
    .filter((unit) => unit.unit_level === form?.unit_level)
    .sort((a, b) => a.name.localeCompare(b.name));
  const formPositionTypes =
    MASTER_POSITION_TYPES_BY_UNIT_LEVEL[form?.unit_level ?? ""] ?? [];
  const forbiddenManagerIDs = editing
    ? subordinatePositionIDs(positions, editing.id)
    : new Set<string>();
  const managerOptions = positions
    .filter(
      (position) =>
        position.is_active &&
        position.id !== editing?.id &&
        !forbiddenManagerIDs.has(position.id),
    )
    .sort((a, b) => {
      const unitCompare = a.org_unit_name.localeCompare(b.org_unit_name);
      return unitCompare || a.title.localeCompare(b.title);
    });

  function openCreate() {
    setEditing(null);
    setForm(emptyForm());
    setModalError(null);
  }

  function openEdit(position: Position) {
    setEditing(position);
    setForm(positionToForm(position));
    setModalError(null);
  }

  function closeModal() {
    if (busy) return;
    setEditing(null);
    setForm(null);
    setModalError(null);
  }

  function applyIdentityChange(
    nextLevel: string,
    nextUnitID: string,
    nextType: string,
  ) {
    const unit = unitsByID.get(nextUnitID);
    setForm((current) =>
      current
        ? {
            ...current,
            unit_level: nextLevel,
            org_unit_id: nextUnitID,
            position_type: nextType,
            title: positionTitleSuggestion(nextType, unit),
            reports_to: suggestManagerID(
              nextType,
              unit,
              positions,
              unitsByID,
              editing?.id,
            ),
            is_approver: DEFAULT_APPROVER_POSITION_TYPES.has(nextType),
          }
        : current,
    );
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!form) return;
    setBusy(true);
    setModalError(null);
    try {
      const payload = compactPayload(form);
      if (!payload.org_unit_id || !payload.position_type || !payload.title) {
        throw new Error("Unit, tipe, dan nama jabatan wajib diisi");
      }
      if (payload.position_type !== "president_director" && !payload.reports_to) {
        throw new Error("Atasan langsung wajib dipilih");
      }
      if (editing) {
        await updatePosition(editing.id, payload);
      } else {
        await createPosition(payload);
      }
      await reload();
      setEditing(null);
      setForm(null);
    } catch (err) {
      setModalError(err instanceof Error ? err.message : "Gagal menyimpan jabatan");
    } finally {
      setBusy(false);
    }
  }

  async function previewDeactivation(position: Position) {
    setActionID(position.id);
    setError(null);
    try {
      const data = await getPositionDeactivationImpact(position.id);
      setDeactivationDialog({ position, impact: data.impact });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memeriksa jabatan");
    } finally {
      setActionID(null);
    }
  }

  async function confirmDeactivation() {
    if (!deactivationDialog?.impact.can_deactivate) return;
    setBusy(true);
    setError(null);
    try {
      await deactivatePosition(deactivationDialog.position.id);
      await reload();
      setDeactivationDialog(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menonaktifkan jabatan");
    } finally {
      setBusy(false);
    }
  }

  async function handleActivate(position: Position) {
    setActionID(position.id);
    setError(null);
    try {
      await activatePosition(position.id);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengaktifkan jabatan");
    } finally {
      setActionID(null);
    }
  }

  return (
    <>
      <main className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 px-4 py-6 sm:px-6">
        <header className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase text-navy-600 dark:text-cyan-400">
              Master Organisasi
            </p>
            <h1 className="mt-1 text-2xl font-semibold text-zinc-950 dark:text-zinc-50">
              Jabatan
            </h1>
          </div>
          <button
            type="button"
            onClick={openCreate}
            className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-navy-700 px-4 text-sm font-semibold text-white shadow-sm transition hover:bg-navy-800"
          >
            <PlusIcon className="h-4 w-4" />
            Tambah Jabatan
          </button>
        </header>

        <section className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <SummaryTile
            label="Jabatan Aktif"
            value={activePositions.length}
            detail={`${positions.length - activePositions.length} nonaktif`}
          />
          <SummaryTile
            label="Terisi"
            value={occupiedCount}
            detail="Memiliki pemegang aktif"
          />
          <SummaryTile
            label="Kosong"
            value={vacantCount}
            detail="Siap ditempatkan"
          />
        </section>

        <section className="grid gap-3 border-y border-zinc-200 py-4 dark:border-zinc-800 lg:grid-cols-[minmax(240px,1fr)_180px_minmax(220px,1fr)_160px]">
          <label className="relative">
            <span className="sr-only">Cari jabatan</span>
            <SearchIcon className="pointer-events-none absolute left-3 top-3 h-4 w-4 text-zinc-400" />
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Cari jabatan, unit, atasan, atau pemegang..."
              className="h-10 w-full rounded-lg border border-zinc-300 bg-white pl-9 pr-3 text-sm text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-50"
            />
            <span className="mt-1 block text-[11px] text-zinc-400">
              Pencarian dan filter hanya berlaku pada halaman ini
            </span>
          </label>
          <select
            aria-label="Filter level unit"
            value={levelFilter}
            onChange={(event) => {
              setLevelFilter(event.target.value);
              setUnitFilter("");
            }}
            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm text-zinc-950 outline-none focus:border-navy-500 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-50"
          >
            <option value="">Semua level</option>
            {UNIT_LEVEL_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <select
            aria-label="Filter unit organisasi"
            value={unitFilter}
            onChange={(event) => setUnitFilter(event.target.value)}
            className="h-10 min-w-0 rounded-lg border border-zinc-300 bg-white px-3 text-sm text-zinc-950 outline-none focus:border-navy-500 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-50"
          >
            <option value="">Semua unit</option>
            {filterUnitOptions.map((unit) => (
              <option key={unit.id} value={unit.id}>
                {unit.name}
              </option>
            ))}
          </select>
          <select
            aria-label="Filter status"
            value={statusFilter}
            onChange={(event) =>
              setStatusFilter(event.target.value as "all" | "active" | "inactive")
            }
            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm text-zinc-950 outline-none focus:border-navy-500 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-50"
          >
            <option value="all">Semua status</option>
            <option value="active">Aktif</option>
            <option value="inactive">Nonaktif</option>
          </select>
        </section>

        {error && (
          <p
            role="alert"
            className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
          >
            {error}
          </p>
        )}

        <section className="hidden overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900 lg:block">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[1080px] text-left text-sm">
              <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
                <tr>
                  <th className="px-4 py-3">Jabatan</th>
                  <th className="px-4 py-3">Unit</th>
                  <th className="px-4 py-3">Atasan</th>
                  <th className="px-4 py-3">Pemegang</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={6} className="px-4 py-10 text-center text-zinc-500">
                      Memuat jabatan...
                    </td>
                  </tr>
                )}
                {!loading && filteredPositions.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-10 text-center text-zinc-500">
                      Tidak ada jabatan yang sesuai.
                    </td>
                  </tr>
                )}
                {!loading &&
                  filteredPositions.map((position) => (
                    <tr
                      key={position.id}
                      className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/70"
                    >
                      <td className="px-4 py-3">
                        <p className="font-semibold text-zinc-900 dark:text-zinc-100">
                          {position.title}
                        </p>
                        <p className="text-xs text-zinc-500">
                          {POSITION_TYPE_LABEL[position.position_type] ??
                            position.position_type}
                          {position.is_approver ? " · Approver" : ""}
                        </p>
                      </td>
                      <td className="px-4 py-3">
                        <p className="text-zinc-700 dark:text-zinc-300">
                          {position.org_unit_name}
                        </p>
                        <p className="text-xs text-zinc-500">
                          {UNIT_LEVEL_LABEL[position.org_unit_level] ??
                            position.org_unit_level}
                        </p>
                      </td>
                      <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">
                        {position.reports_to_title || "Puncak organisasi"}
                      </td>
                      <td className="px-4 py-3">
                        {position.holder_name ? (
                          <span className="font-medium text-zinc-800 dark:text-zinc-200">
                            {position.holder_name}
                          </span>
                        ) : (
                          <span className="text-amber-700 dark:text-amber-300">
                            Belum terisi
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                            position.is_active
                              ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                              : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                          }`}
                        >
                          {position.is_active ? "Aktif" : "Nonaktif"}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => openEdit(position)}
                            className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-zinc-300 px-2.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                          >
                            <EditIcon className="h-3.5 w-3.5" />
                            Edit
                          </button>
                          {position.is_active ? (
                            <button
                              type="button"
                              onClick={() => previewDeactivation(position)}
                              disabled={actionID === position.id}
                              className="h-8 rounded-lg border border-red-200 px-2.5 text-xs font-semibold text-red-700 hover:bg-red-50 disabled:opacity-50 dark:border-red-900 dark:text-red-300 dark:hover:bg-red-950"
                            >
                              Nonaktifkan
                            </button>
                          ) : (
                            <button
                              type="button"
                              onClick={() => handleActivate(position)}
                              disabled={actionID === position.id}
                              className="h-8 rounded-lg border border-emerald-200 px-2.5 text-xs font-semibold text-emerald-700 hover:bg-emerald-50 disabled:opacity-50 dark:border-emerald-900 dark:text-emerald-300 dark:hover:bg-emerald-950"
                            >
                              Aktifkan
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
          <div className="px-4">
            <Pagination
              page={page}
              totalPages={meta?.total_pages ?? 1}
              onPageChange={setPage}
              disabled={loading}
            />
          </div>
        </section>

        <section className="grid gap-3 lg:hidden">
          {loading && (
            <p className="py-8 text-center text-sm text-zinc-500">Memuat jabatan...</p>
          )}
          {!loading && filteredPositions.length === 0 && (
            <p className="py-8 text-center text-sm text-zinc-500">
              Tidak ada jabatan yang sesuai.
            </p>
          )}
          {filteredPositions.map((position) => (
            <article
              key={position.id}
              className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">
                    {position.title}
                  </h2>
                  <p className="mt-1 text-xs text-zinc-500">
                    {position.org_unit_name} ·{" "}
                    {UNIT_LEVEL_LABEL[position.org_unit_level] ??
                      position.org_unit_level}
                  </p>
                </div>
                <span
                  className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                    position.is_active
                      ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                      : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
                  }`}
                >
                  {position.is_active ? "Aktif" : "Nonaktif"}
                </span>
              </div>
              <dl className="mt-3 grid grid-cols-2 gap-3 border-t border-zinc-100 pt-3 text-xs dark:border-zinc-800">
                <div>
                  <dt className="text-zinc-500">Atasan</dt>
                  <dd className="mt-0.5 text-zinc-800 dark:text-zinc-200">
                    {position.reports_to_title || "Puncak organisasi"}
                  </dd>
                </div>
                <div>
                  <dt className="text-zinc-500">Pemegang</dt>
                  <dd className="mt-0.5 text-zinc-800 dark:text-zinc-200">
                    {position.holder_name || "Belum terisi"}
                  </dd>
                </div>
              </dl>
              <div className="mt-4 flex gap-2">
                <button
                  type="button"
                  onClick={() => openEdit(position)}
                  className="inline-flex h-9 flex-1 items-center justify-center gap-2 rounded-lg border border-zinc-300 text-xs font-semibold text-zinc-700 dark:border-zinc-700 dark:text-zinc-300"
                >
                  <EditIcon className="h-3.5 w-3.5" />
                  Edit
                </button>
                {position.is_active ? (
                  <button
                    type="button"
                    onClick={() => previewDeactivation(position)}
                    className="h-9 flex-1 rounded-lg border border-red-200 text-xs font-semibold text-red-700 dark:border-red-900 dark:text-red-300"
                  >
                    Nonaktifkan
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={() => handleActivate(position)}
                    className="h-9 flex-1 rounded-lg border border-emerald-200 text-xs font-semibold text-emerald-700 dark:border-emerald-900 dark:text-emerald-300"
                  >
                    Aktifkan
                  </button>
                )}
              </div>
            </article>
          ))}
          <Pagination
            page={page}
            totalPages={meta?.total_pages ?? 1}
            onPageChange={setPage}
            disabled={loading}
          />
        </section>
      </main>

      {form && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="position-form-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <form
            onSubmit={handleSubmit}
            className="flex max-h-full w-full max-w-2xl flex-col overflow-hidden rounded-xl bg-white shadow-2xl dark:bg-zinc-900"
          >
            <header className="flex items-start justify-between border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
              <div>
                <p className="text-xs font-semibold uppercase text-navy-600 dark:text-cyan-400">
                  Master Jabatan
                </p>
                <h2
                  id="position-form-title"
                  className="mt-1 text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  {editing ? "Edit Jabatan" : "Tambah Jabatan"}
                </h2>
              </div>
              <button
                type="button"
                onClick={closeModal}
                aria-label="Tutup"
                className="rounded-lg border border-zinc-200 p-2 text-zinc-500 hover:bg-zinc-100 dark:border-zinc-800 dark:hover:bg-zinc-800"
              >
                <XIcon className="h-4 w-4" />
              </button>
            </header>

            <div className="grid gap-4 overflow-y-auto px-5 py-5 sm:grid-cols-2">
              {editing?.identity_locked && (
                <p className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300 sm:col-span-2">
                  Unit dan tipe terkunci karena jabatan sudah memiliki histori.
                </p>
              )}

              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Level Unit
                <select
                  value={form.unit_level}
                  disabled={Boolean(editing?.identity_locked)}
                  onChange={(event) =>
                    applyIdentityChange(event.target.value, "", "")
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="">Pilih level unit</option>
                  {UNIT_LEVEL_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex min-w-0 flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Unit Organisasi
                <select
                  value={form.org_unit_id}
                  disabled={!form.unit_level || Boolean(editing?.identity_locked)}
                  onChange={(event) =>
                    applyIdentityChange(form.unit_level, event.target.value, "")
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="">
                    {form.unit_level ? "Pilih unit organisasi" : "Pilih level dahulu"}
                  </option>
                  {formUnitOptions.map((unit) => (
                    <option key={unit.id} value={unit.id}>
                      {unitLabel(unit)}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Tipe Jabatan
                <select
                  value={form.position_type}
                  disabled={!form.org_unit_id || Boolean(editing?.identity_locked)}
                  onChange={(event) =>
                    applyIdentityChange(
                      form.unit_level,
                      form.org_unit_id,
                      event.target.value,
                    )
                  }
                  required
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="">
                    {form.org_unit_id ? "Pilih tipe jabatan" : "Pilih unit dahulu"}
                  </option>
                  {formPositionTypes.map((positionType) => (
                    <option key={positionType} value={positionType}>
                      {POSITION_TYPE_LABEL[positionType] ?? positionType}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                Nama Jabatan
                <input
                  value={form.title}
                  onChange={(event) =>
                    setForm((current) =>
                      current ? { ...current, title: event.target.value } : current,
                    )
                  }
                  required
                  maxLength={150}
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                />
              </label>

              <label className="flex min-w-0 flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 sm:col-span-2">
                Atasan Langsung
                <select
                  value={form.reports_to}
                  disabled={!form.position_type}
                  onChange={(event) =>
                    setForm((current) =>
                      current
                        ? { ...current, reports_to: event.target.value }
                        : current,
                    )
                  }
                  required={form.position_type !== "president_director"}
                  className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                >
                  <option value="">
                    {form.position_type === "president_director"
                      ? "Tanpa atasan langsung"
                      : "Pilih atasan langsung"}
                  </option>
                  {managerOptions.map((position) => (
                    <option key={position.id} value={position.id}>
                      {position.title} · {position.org_unit_name}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex items-center gap-3 rounded-lg border border-zinc-300 px-3 py-2.5 text-sm font-semibold text-zinc-800 dark:border-zinc-700 dark:text-zinc-200 sm:col-span-2">
                <input
                  type="checkbox"
                  checked={form.is_approver}
                  onChange={(event) =>
                    setForm((current) =>
                      current
                        ? { ...current, is_approver: event.target.checked }
                        : current,
                    )
                  }
                  className="h-4 w-4 rounded border-zinc-300 text-navy-700 focus:ring-navy-600"
                />
                Bertindak sebagai approver
              </label>

              {modalError && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300 sm:col-span-2"
                >
                  {modalError}
                </p>
              )}
            </div>

            <footer className="flex shrink-0 flex-col-reverse gap-2 border-t border-zinc-200 px-5 py-4 dark:border-zinc-800 sm:flex-row sm:justify-end">
              <button
                type="button"
                onClick={closeModal}
                disabled={busy}
                className="h-10 rounded-lg border border-zinc-300 px-4 text-sm font-semibold text-zinc-700 hover:bg-zinc-100 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={busy}
                className="h-10 rounded-lg bg-navy-700 px-4 text-sm font-semibold text-white shadow-sm hover:bg-navy-800 disabled:opacity-50"
              >
                {busy ? "Menyimpan..." : editing ? "Simpan Perubahan" : "Tambah Jabatan"}
              </button>
            </footer>
          </form>
        </div>
      )}

      {deactivationDialog && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="position-deactivation-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <div className="w-full max-w-lg rounded-xl bg-white shadow-2xl dark:bg-zinc-900">
            <header className="flex items-start justify-between border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
              <div>
                <h2
                  id="position-deactivation-title"
                  className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  Nonaktifkan Jabatan
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  {deactivationDialog.position.title}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setDeactivationDialog(null)}
                aria-label="Tutup"
                className="rounded-lg border border-zinc-200 p-2 text-zinc-500 hover:bg-zinc-100 dark:border-zinc-800 dark:hover:bg-zinc-800"
              >
                <XIcon className="h-4 w-4" />
              </button>
            </header>

            <div className="px-5 py-5">
              {deactivationDialog.impact.can_deactivate ? (
                <div className="flex items-start gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300">
                  <BriefcaseIcon className="mt-0.5 h-5 w-5 shrink-0" />
                  Jabatan tidak memiliki proses aktif dan dapat dinonaktifkan.
                </div>
              ) : (
                <div>
                  <p className="text-sm text-zinc-600 dark:text-zinc-300">
                    Selesaikan dependensi berikut sebelum menonaktifkan jabatan.
                  </p>
                  <dl className="mt-4 divide-y divide-zinc-200 border-y border-zinc-200 text-sm dark:divide-zinc-800 dark:border-zinc-800">
                    {IMPACT_LABELS.filter(
                      ({ key }) => deactivationDialog.impact[key] > 0,
                    ).map(({ key, label }) => (
                      <div
                        key={key}
                        className="flex items-center justify-between gap-4 py-3"
                      >
                        <dt className="text-zinc-600 dark:text-zinc-300">{label}</dt>
                        <dd className="font-semibold text-zinc-950 dark:text-zinc-50">
                          {deactivationDialog.impact[key]}
                        </dd>
                      </div>
                    ))}
                  </dl>
                </div>
              )}
            </div>

            <footer className="flex justify-end gap-2 border-t border-zinc-200 px-5 py-4 dark:border-zinc-800">
              <button
                type="button"
                onClick={() => setDeactivationDialog(null)}
                disabled={busy}
                className="h-10 rounded-lg border border-zinc-300 px-4 text-sm font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Tutup
              </button>
              {deactivationDialog.impact.can_deactivate && (
                <button
                  type="button"
                  onClick={confirmDeactivation}
                  disabled={busy}
                  className="h-10 rounded-lg bg-red-700 px-4 text-sm font-semibold text-white hover:bg-red-800 disabled:opacity-50"
                >
                  {busy ? "Memproses..." : "Nonaktifkan"}
                </button>
              )}
            </footer>
          </div>
        </div>
      )}
    </>
  );
}
