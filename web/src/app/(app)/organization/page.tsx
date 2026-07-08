"use client";

import { useEffect, useMemo, useState } from "react";
import {
  BuildingIcon,
  ChevronDownIcon,
  SearchIcon,
} from "@/components/layout/icons";
import {
  getOrgTree,
  listPositions,
  type OrgUnit,
  type Position,
} from "@/lib/api";
import { POSITION_TYPE_LABEL } from "@/lib/position-types";

const LEVEL_LABEL: Record<string, string> = {
  directorate: "Direktorat",
  biro: "Biro",
  department: "Department",
  division: "Division",
  office: "Office",
};

const LEVEL_STYLE: Record<string, string> = {
  directorate:
    "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  biro: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  department:
    "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  division: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  office: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
};

const POSITION_TYPE_STYLE: Record<string, string> = {
  president_director:
    "bg-navy-100 text-navy-800 dark:bg-navy-950 dark:text-sky-300",
  vp_director: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  director:
    "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  gm: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  dept_head: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  division_head:
    "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  secretary:
    "bg-cyan-100 text-cyan-800 dark:bg-cyan-950 dark:text-cyan-300",
  auditor:
    "bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300",
};

const LEVEL_FILTERS = [
  { value: "all", label: "Semua" },
  { value: "directorate", label: "Direktorat" },
  { value: "biro", label: "Biro" },
  { value: "department", label: "Department" },
  { value: "division", label: "Division" },
  { value: "office", label: "Office" },
] as const;

const REGION_FILTERS = [
  { value: "all", label: "Semua" },
  { value: "HO", label: "HO" },
  { value: "REG1", label: "REG1" },
  { value: "REG2", label: "REG2" },
  { value: "REPO", label: "REPO" },
] as const;

type LevelFilter = (typeof LEVEL_FILTERS)[number]["value"];
type RegionFilter = (typeof REGION_FILTERS)[number]["value"];

interface OrgIndexes {
  byID: Map<string, OrgUnit>;
  parentByID: Map<string, OrgUnit>;
  allIDs: string[];
  defaultExpandedIDs: string[];
  countsByLevel: Map<string, number>;
  regions: Set<string>;
}

function regionGroup(region: string | null): string {
  if (!region) return "-";
  return region.startsWith("REPO") ? "REPO" : region;
}

function normalize(value: string | null | undefined): string {
  return (value ?? "").toLowerCase();
}

function hasChildren(unit: OrgUnit): boolean {
  return Boolean(unit.children && unit.children.length > 0);
}

function buildIndexes(tree: OrgUnit[]): OrgIndexes {
  const byID = new Map<string, OrgUnit>();
  const parentByID = new Map<string, OrgUnit>();
  const allIDs: string[] = [];
  const defaultExpandedIDs: string[] = [];
  const countsByLevel = new Map<string, number>();
  const regions = new Set<string>();

  function visit(unit: OrgUnit, parent: OrgUnit | null) {
    byID.set(unit.id, unit);
    allIDs.push(unit.id);
    countsByLevel.set(unit.unit_level, (countsByLevel.get(unit.unit_level) ?? 0) + 1);
    if (unit.region) regions.add(regionGroup(unit.region));
    if (parent) parentByID.set(unit.id, parent);

    if (hasChildren(unit) && unit.unit_level !== "department") {
      defaultExpandedIDs.push(unit.id);
    }

    unit.children?.forEach((child) => visit(child, unit));
  }

  tree.forEach((unit) => visit(unit, null));
  return { byID, parentByID, allIDs, defaultExpandedIDs, countsByLevel, regions };
}

function unitMatches(
  unit: OrgUnit,
  positions: Position[],
  query: string,
  levelFilter: LevelFilter,
  regionFilter: RegionFilter,
): boolean {
  const levelOK = levelFilter === "all" || unit.unit_level === levelFilter;
  const regionOK =
    regionFilter === "all" || regionGroup(unit.region) === regionFilter;
  const queryOK =
    query.length === 0 ||
    normalize(unit.name).includes(query) ||
    normalize(unit.code).includes(query) ||
    normalize(unit.region).includes(query) ||
    normalize(LEVEL_LABEL[unit.unit_level] ?? unit.unit_level).includes(query) ||
    positions.some(
      (position) =>
        normalize(position.title).includes(query) ||
        normalize(POSITION_TYPE_LABEL[position.position_type]).includes(query) ||
        normalize(position.holder_name).includes(query),
    );

  return levelOK && regionOK && queryOK;
}

function filterTree(
  units: OrgUnit[],
  positionsByUnit: Map<string, Position[]>,
  query: string,
  levelFilter: LevelFilter,
  regionFilter: RegionFilter,
): OrgUnit[] {
  return units.flatMap((unit) => {
    const children = filterTree(
      unit.children ?? [],
      positionsByUnit,
      query,
      levelFilter,
      regionFilter,
    );
    if (
      unitMatches(
        unit,
        positionsByUnit.get(unit.id) ?? [],
        query,
        levelFilter,
        regionFilter,
      ) ||
      children.length > 0
    ) {
      return [{ ...unit, children }];
    }
    return [];
  });
}

function countVisible(units: OrgUnit[]): number {
  return units.reduce(
    (sum, unit) => sum + 1 + countVisible(unit.children ?? []),
    0,
  );
}

function countDescendants(unit: OrgUnit): number {
  return (unit.children ?? []).reduce(
    (sum, child) => sum + 1 + countDescendants(child),
    0,
  );
}

function selectedOrFirst(
  selectedID: string | null,
  byID: Map<string, OrgUnit>,
  tree: OrgUnit[],
): OrgUnit | null {
  if (selectedID && byID.has(selectedID)) return byID.get(selectedID) ?? null;
  return tree[0] ?? null;
}

function SummaryStat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white px-4 py-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
        {label}
      </p>
      <p className="mt-1 text-lg font-semibold text-zinc-950 dark:text-zinc-50">
        {value}
      </p>
    </div>
  );
}

interface UnitNodeProps {
  unit: OrgUnit;
  depth: number;
  expandedIDs: Set<string>;
  filterActive: boolean;
  selectedID: string | null;
  positionsByUnit: Map<string, Position[]>;
  onSelect: (unit: OrgUnit) => void;
  onToggle: (id: string) => void;
}

function UnitNode({
  unit,
  depth,
  expandedIDs,
  filterActive,
  selectedID,
  positionsByUnit,
  onSelect,
  onToggle,
}: UnitNodeProps) {
  const childCount = unit.children?.length ?? 0;
  const unitPositions = positionsByUnit.get(unit.id) ?? [];
  const hasSecretary = unitPositions.some(
    (position) => position.position_type === "secretary",
  );
  const expandable = childCount > 0;
  const expanded = filterActive || expandedIDs.has(unit.id);
  const selected = selectedID === unit.id;

  return (
    <li>
      <div
        className="relative"
        style={{ paddingLeft: Math.min(depth, 5) * 18 }}
      >
        {depth > 0 && (
          <span
            aria-hidden
            className="absolute bottom-3 top-0 w-px bg-zinc-200 dark:bg-zinc-800"
            style={{ left: Math.min(depth, 5) * 18 - 10 }}
          />
        )}
        <div
          className={`grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2 rounded-lg border px-2.5 py-2 text-left shadow-sm transition sm:px-3 ${
            selected
              ? "border-navy-300 bg-navy-50 ring-2 ring-navy-100 dark:border-sky-700 dark:bg-navy-950/60 dark:ring-navy-900"
              : "border-zinc-200 bg-white hover:border-zinc-300 hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-700 dark:hover:bg-zinc-800/70"
          }`}
        >
          <button
            type="button"
            onClick={() => (expandable ? onToggle(unit.id) : onSelect(unit))}
            disabled={!expandable}
            aria-label={expanded ? `Ciutkan ${unit.name}` : `Perluas ${unit.name}`}
            aria-expanded={expandable ? expanded : undefined}
            className={`flex h-7 w-7 items-center justify-center rounded-md transition ${
              expandable
                ? "text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
                : "cursor-default text-zinc-300 dark:text-zinc-700"
            }`}
          >
            <ChevronDownIcon
              className={`h-4 w-4 transition-transform ${
                expanded ? "rotate-0" : "-rotate-90"
              }`}
            />
          </button>

          <button
            type="button"
            onClick={() => onSelect(unit)}
            className="min-w-0 text-left"
          >
            <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <span
                className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                  LEVEL_STYLE[unit.unit_level] ??
                  "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
                }`}
              >
                {LEVEL_LABEL[unit.unit_level] ?? unit.unit_level}
              </span>
              <span className="min-w-0 break-words text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                {unit.name}
              </span>
              <span className="font-mono text-[11px] font-medium text-zinc-400">
                {unit.code}
              </span>
              {unitPositions.length > 0 && (
                <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                  {unitPositions.length} jabatan
                </span>
              )}
              {hasSecretary && (
                <span className="rounded-full bg-cyan-100 px-2 py-0.5 text-[11px] font-semibold text-cyan-800 dark:bg-cyan-950 dark:text-cyan-300">
                  Secretary
                </span>
              )}
            </span>
          </button>

          <div className="flex items-center gap-2 justify-self-end">
            {childCount > 0 && (
              <span className="hidden rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-500 sm:inline-flex dark:bg-zinc-800 dark:text-zinc-400">
                {childCount}
              </span>
            )}
            <span className="min-w-[44px] text-right font-mono text-[11px] font-semibold text-zinc-400">
              {unit.region ?? "-"}
            </span>
          </div>
        </div>
      </div>

      {expanded && childCount > 0 && (
        <ul className="mt-2 flex flex-col gap-2">
          {unit.children?.map((child) => (
            <UnitNode
              key={child.id}
              unit={child}
              depth={depth + 1}
              expandedIDs={expandedIDs}
              filterActive={filterActive}
              selectedID={selectedID}
              positionsByUnit={positionsByUnit}
              onSelect={onSelect}
              onToggle={onToggle}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

interface DetailPanelProps {
  unit: OrgUnit | null;
  parent: OrgUnit | null;
  positions: Position[];
}

function DetailPanel({ unit, parent, positions }: DetailPanelProps) {
  if (!unit) {
    return (
      <aside className="rounded-xl border border-dashed border-zinc-300 bg-white px-5 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700 dark:bg-zinc-900">
        Pilih unit organisasi untuk melihat detail.
      </aside>
    );
  }

  const activeUnit = unit;
  const descendants = countDescendants(activeUnit);
  const sortedPositions = [...positions].sort((a, b) => {
    if (a.is_approver !== b.is_approver) return a.is_approver ? -1 : 1;
    return a.title.localeCompare(b.title);
  });

  return (
    <aside className="rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900 lg:sticky lg:top-24">
      <div className="border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
        <div className="flex items-start gap-3">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-navy-50 text-navy-700 dark:bg-navy-950 dark:text-sky-300">
            <BuildingIcon className="h-5 w-5" />
          </span>
          <div className="min-w-0">
            <p className="break-words text-base font-semibold text-zinc-950 dark:text-zinc-50">
              {unit.name}
            </p>
            <p className="mt-1 font-mono text-xs font-semibold text-zinc-400">
              {unit.code}
            </p>
          </div>
        </div>
      </div>

      <dl className="grid grid-cols-2 gap-3 px-5 py-4 text-sm">
        <div>
          <dt className="text-xs font-semibold uppercase tracking-wide text-zinc-400">
            Level
          </dt>
          <dd className="mt-1 text-zinc-900 dark:text-zinc-100">
            {LEVEL_LABEL[unit.unit_level] ?? unit.unit_level}
          </dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-wide text-zinc-400">
            Region
          </dt>
          <dd className="mt-1 text-zinc-900 dark:text-zinc-100">
            {unit.region ?? "-"}
          </dd>
        </div>
        <div className="col-span-2">
          <dt className="text-xs font-semibold uppercase tracking-wide text-zinc-400">
            Parent
          </dt>
          <dd className="mt-1 break-words text-zinc-900 dark:text-zinc-100">
            {parent?.name ?? "Root"}
          </dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-wide text-zinc-400">
            Child
          </dt>
          <dd className="mt-1 text-zinc-900 dark:text-zinc-100">
            {unit.children?.length ?? 0} langsung
          </dd>
        </div>
        <div>
          <dt className="text-xs font-semibold uppercase tracking-wide text-zinc-400">
            Turunan
          </dt>
          <dd className="mt-1 text-zinc-900 dark:text-zinc-100">
            {descendants} total
          </dd>
        </div>
      </dl>

      <div className="border-t border-zinc-200 px-5 py-4 dark:border-zinc-800">
        <div className="mb-3 flex items-center justify-between gap-3">
          <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
            Jabatan Aktif
          </p>
          <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
            {sortedPositions.length}
          </span>
        </div>

        {sortedPositions.length === 0 ? (
          <p className="rounded-lg border border-dashed border-zinc-300 px-3 py-4 text-sm text-zinc-500 dark:border-zinc-700">
            Belum ada jabatan aktif di unit ini.
          </p>
        ) : (
          <ul className="grid gap-2">
            {sortedPositions.map((position) => (
              <li
                key={position.id}
                className="rounded-lg border border-zinc-200 px-3 py-2 dark:border-zinc-800"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span
                    className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                      POSITION_TYPE_STYLE[position.position_type] ??
                      "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
                    }`}
                  >
                    {POSITION_TYPE_LABEL[position.position_type] ??
                      position.position_type}
                  </span>
                  {position.is_approver && (
                    <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                      Approver
                    </span>
                  )}
                </div>
                <p className="mt-2 break-words text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                  {position.title}
                </p>
                <p className="mt-1 break-words text-xs text-zinc-500">
                  {position.holder_name || "Belum ada pemegang aktif"}
                </p>
              </li>
            ))}
          </ul>
        )}
      </div>
    </aside>
  );
}

export default function OrganizationPage() {
  const [tree, setTree] = useState<OrgUnit[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");
  const [levelFilter, setLevelFilter] = useState<LevelFilter>("all");
  const [regionFilter, setRegionFilter] = useState<RegionFilter>("all");
  const [expandedIDs, setExpandedIDs] = useState<Set<string>>(new Set());
  const [selectedID, setSelectedID] = useState<string | null>(null);

  const indexes = useMemo(() => buildIndexes(tree), [tree]);
  const positionsByUnit = useMemo(() => {
    const next = new Map<string, Position[]>();
    positions.forEach((position) => {
      const current = next.get(position.org_unit_id) ?? [];
      current.push(position);
      next.set(position.org_unit_id, current);
    });
    return next;
  }, [positions]);

  useEffect(() => {
    Promise.all([getOrgTree(), listPositions()])
      .then(([org, positionData]) => {
        const nextIndexes = buildIndexes(org.tree);
        setTree(org.tree);
        setPositions(positionData.positions);
        setTotal(org.total);
        setExpandedIDs(new Set(nextIndexes.defaultExpandedIDs));
        setSelectedID(org.tree[0]?.id ?? null);
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat data"),
      )
      .finally(() => setLoading(false));
  }, []);

  const normalizedQuery = normalize(query.trim());
  const filterActive =
    normalizedQuery.length > 0 || levelFilter !== "all" || regionFilter !== "all";
  const visibleTree = useMemo(
    () => filterTree(tree, positionsByUnit, normalizedQuery, levelFilter, regionFilter),
    [tree, positionsByUnit, normalizedQuery, levelFilter, regionFilter],
  );
  const visibleCount = useMemo(() => countVisible(visibleTree), [visibleTree]);
  const selectedUnit = selectedOrFirst(selectedID, indexes.byID, tree);
  const selectedParent =
    selectedUnit ? indexes.parentByID.get(selectedUnit.id) ?? null : null;
  const selectedPositions = selectedUnit
    ? positionsByUnit.get(selectedUnit.id) ?? []
    : [];
  const directorateCount = indexes.countsByLevel.get("directorate") ?? 0;
  const biroCount = indexes.countsByLevel.get("biro") ?? 0;
  const departmentCount = indexes.countsByLevel.get("department") ?? 0;
  const divisionCount = indexes.countsByLevel.get("division") ?? 0;
  const activeRegionCount = indexes.regions.size;

  function toggleExpanded(id: string) {
    setExpandedIDs((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <main className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 px-4 py-6 sm:px-6">
      <section className="grid gap-4">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold text-zinc-950 dark:text-zinc-50">
              Struktur Organisasi
            </h1>
            <p className="mt-1 text-sm text-zinc-500">
              FKK Group · {total} unit aktif
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => setExpandedIDs(new Set(indexes.allIDs))}
              disabled={loading || indexes.allIDs.length === 0}
              className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Expand semua
            </button>
            <button
              type="button"
              onClick={() => setExpandedIDs(new Set())}
              disabled={loading || indexes.allIDs.length === 0}
              className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Collapse semua
            </button>
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
          <SummaryStat label="Total Unit" value={total} />
          <SummaryStat label="Direktorat" value={directorateCount} />
          <SummaryStat label="Biro" value={biroCount} />
          <SummaryStat label="Department" value={departmentCount} />
          <SummaryStat label="Division" value={divisionCount} />
          <SummaryStat label="Region Aktif" value={activeRegionCount} />
        </div>

        <div className="grid gap-3 rounded-xl border border-zinc-200 bg-white p-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-900 lg:grid-cols-[1fr_240px_170px]">
          <label className="relative block">
            <span className="sr-only">Cari organisasi</span>
            <SearchIcon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Cari nama, kode, atau region..."
              className="h-10 w-full rounded-lg border border-zinc-300 bg-white pl-9 pr-3 text-sm text-zinc-900 outline-none transition placeholder:text-zinc-400 focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
            />
          </label>
          <label className="block">
            <span className="sr-only">Filter level</span>
            <select
              value={levelFilter}
              onChange={(event) => setLevelFilter(event.target.value as LevelFilter)}
              className="h-10 w-full rounded-lg border border-zinc-300 bg-white px-3 text-sm text-zinc-900 outline-none transition focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
            >
              {LEVEL_FILTERS.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  Level: {filter.label}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="sr-only">Filter region</span>
            <select
              value={regionFilter}
              onChange={(event) => setRegionFilter(event.target.value as RegionFilter)}
              className="h-10 w-full rounded-lg border border-zinc-300 bg-white px-3 text-sm text-zinc-900 outline-none transition focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
            >
              {REGION_FILTERS.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  Region: {filter.label}
                </option>
              ))}
            </select>
          </label>
        </div>
      </section>

      {loading && <p className="text-sm text-zinc-500">Memuat struktur organisasi...</p>}
      {error && (
        <p
          role="alert"
          className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
        >
          {error}
        </p>
      )}

      {!loading && !error && tree.length === 0 && (
        <p className="rounded-xl border border-dashed border-zinc-300 bg-white px-4 py-10 text-center text-sm text-zinc-500 dark:border-zinc-700 dark:bg-zinc-900">
          Belum ada unit organisasi. Admin dapat menambahkan data organisasi via API.
        </p>
      )}

      {!loading && !error && tree.length > 0 && (
        <section className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
          <div className="rounded-xl border border-zinc-200 bg-white p-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2 border-b border-zinc-200 pb-3 dark:border-zinc-800">
              <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                Explorer Organisasi
              </p>
              <p className="text-xs text-zinc-500">
                {filterActive ? `${visibleCount} hasil cocok` : `${total} unit`}
              </p>
            </div>

            {visibleTree.length === 0 ? (
              <p className="rounded-lg border border-dashed border-zinc-300 px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-700">
                Tidak ada unit yang cocok dengan pencarian atau filter.
              </p>
            ) : (
              <ul className="flex flex-col gap-2">
                {visibleTree.map((unit) => (
                  <UnitNode
                    key={unit.id}
                    unit={unit}
                    depth={0}
                    expandedIDs={expandedIDs}
                    filterActive={filterActive}
                    selectedID={selectedID}
                    positionsByUnit={positionsByUnit}
                    onSelect={(nextUnit) => setSelectedID(nextUnit.id)}
                    onToggle={toggleExpanded}
                  />
                ))}
              </ul>
            )}
          </div>

          <DetailPanel
            key={selectedUnit?.id ?? "empty"}
            unit={selectedUnit}
            parent={selectedParent}
            positions={selectedPositions}
          />
        </section>
      )}
    </main>
  );
}
