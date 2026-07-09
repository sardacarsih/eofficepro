"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  createUser,
  deactivateUser,
  downloadImportTemplate,
  getOrgTree,
  getUserDeactivationImpact,
  importUsers,
  listAllPositions,
  listUsers,
  updateUser,
  type DeactivateUserPayload,
  type DeactivationImpact,
  type ImportResult,
  type OrgUnit,
  type PageMeta,
  type Position,
  type UserPayload,
  type UserPositionAssignment,
  type UserPositionPayload,
  type UserRow,
} from "@/lib/api";
import Pagination from "@/components/Pagination";
import {
  POSITION_TYPE_LABEL,
  USER_POSITION_TYPES_BY_UNIT_LEVEL,
} from "@/lib/position-types";
import { useCurrentUser } from "@/components/layout/CurrentUserProvider";
import {
  DownloadIcon,
  EditIcon,
  PlusIcon,
  SearchIcon,
  UploadIcon,
  XIcon,
} from "@/components/layout/icons";

const ROLE_OPTIONS = [
  { value: "admin", label: "Admin" },
  { value: "creator", label: "Creator" },
  { value: "approver", label: "Approver" },
  { value: "secretary", label: "Secretary" },
  { value: "auditor", label: "Auditor" },
] as const;

const STATUS_LABEL: Record<UserPayload["status"], string> = {
  active: "Aktif",
  inactive: "Nonaktif",
  locked: "Terkunci",
};

const STATUS_STYLE: Record<UserPayload["status"], string> = {
  active: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  inactive: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300",
  locked: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
};

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

type AssignmentType = "definitive" | "plt" | "plh";

const ASSIGNMENT_LABEL: Record<AssignmentType, string> = {
  definitive: "Definitif",
  plt: "Plt",
  plh: "Plh",
};

const ASSIGNMENT_STYLE: Record<AssignmentType, string> = {
  definitive: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  plt: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  plh: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
};

interface PendingPositionAssignment {
  temp_id: string;
  position_id: string;
  assignment_type: AssignmentType;
}

interface DeactivationWizardState {
  user: UserRow;
  impact: DeactivationImpact;
  positionReplacements: Record<
    string,
    {
      replacement_user_id: string;
      assignment_type: AssignmentType;
    }
  >;
  draftTransfers: Record<
    string,
    {
      replacement_user_id: string;
      replacement_position_id: string;
    }
  >;
  error: string | null;
}

interface UserFormState {
  nik: string;
  email: string;
  full_name: string;
  status: UserPayload["status"];
  roles: string[];
  password: string;
  positions: UserPositionAssignment[];
  ended_assignment_ids: string[];
  pending_positions: PendingPositionAssignment[];
  new_position_unit_level: string;
  new_position_org_unit_id: string;
  new_position_id: string;
  new_assignment_type: AssignmentType;
}

function emptyForm(): UserFormState {
  return {
    nik: "",
    email: "",
    full_name: "",
    status: "active",
    roles: ["creator"],
    password: "",
    positions: [],
    ended_assignment_ids: [],
    pending_positions: [],
    new_position_unit_level: "",
    new_position_org_unit_id: "",
    new_position_id: "",
    new_assignment_type: "definitive",
  };
}

function userToForm(user: UserRow): UserFormState {
  return {
    nik: user.nik,
    email: user.email,
    full_name: user.full_name,
    status: user.status as UserPayload["status"],
    roles: user.roles.length > 0 ? user.roles : ["creator"],
    password: "",
    positions: user.positions ?? [],
    ended_assignment_ids: [],
    pending_positions: [],
    new_position_unit_level: "",
    new_position_org_unit_id: "",
    new_position_id: "",
    new_assignment_type: "definitive",
  };
}

function compactPayload(form: UserFormState): UserPayload {
  return {
    nik: form.nik.trim(),
    email: form.email.trim(),
    full_name: form.full_name.trim(),
    status: form.status,
    roles: form.roles,
    positions: formPositionPayloads(form),
    ...(form.password.trim() ? { password: form.password } : {}),
  };
}

function formPositionPayloads(form: UserFormState): UserPositionPayload[] {
  return [
    ...form.positions
      .filter((position) => !form.ended_assignment_ids.includes(position.assignment_id))
      .map((position) => ({
        position_id: position.position_id,
        assignment_type: position.assignment_type,
      })),
    ...form.pending_positions.map((position) => ({
      position_id: position.position_id,
      assignment_type: position.assignment_type,
    })),
  ];
}

function positionLabel(position: Position): string {
  return `${position.title} · ${position.org_unit_name}`;
}

function flattenOrgUnits(units: OrgUnit[]): OrgUnit[] {
  return units.flatMap((unit) => [unit, ...flattenOrgUnits(unit.children ?? [])]);
}

function descendantUnitIDs(units: OrgUnit[], rootID: string): Set<string> {
  const childrenByParent = new Map<string, string[]>();
  units.forEach((unit) => {
    if (!unit.parent_id) return;
    const childIDs = childrenByParent.get(unit.parent_id) ?? [];
    childIDs.push(unit.id);
    childrenByParent.set(unit.parent_id, childIDs);
  });

  const descendants = new Set<string>();
  const pending = [...(childrenByParent.get(rootID) ?? [])];
  while (pending.length > 0) {
    const unitID = pending.pop();
    if (!unitID || descendants.has(unitID)) continue;
    descendants.add(unitID);
    pending.push(...(childrenByParent.get(unitID) ?? []));
  }
  return descendants;
}

function orgUnitLabel(unit: OrgUnit): string {
  const level = UNIT_LEVEL_LABEL[unit.unit_level] ?? unit.unit_level;
  return `${unit.name} · ${level}${unit.region ? ` · ${unit.region}` : ""}`;
}

function positionSearchText(position: Position): string {
  return [
    position.title,
    position.org_unit_name,
    position.position_type,
    POSITION_TYPE_LABEL[position.position_type],
    position.holder_name,
  ]
    .join(" ")
    .toLowerCase();
}

function assignmentLabel(position: UserPositionAssignment): string {
  return `${position.title} · ${position.org_unit_name}`;
}

function roleLabel(role: string): string {
  return ROLE_OPTIONS.find((option) => option.value === role)?.label ?? role;
}

function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
}

function SummaryTile({
  label,
  value,
  detail,
  tone = "zinc",
}: {
  label: string;
  value: number;
  detail: string;
  tone?: "navy" | "emerald" | "amber" | "zinc";
}) {
  const toneClass = {
    navy: "border-navy-200 bg-navy-50 text-navy-800 dark:border-navy-900 dark:bg-navy-950/70 dark:text-sky-200",
    emerald:
      "border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/50 dark:text-emerald-200",
    amber:
      "border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/50 dark:text-amber-200",
    zinc: "border-zinc-200 bg-white text-zinc-900 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100",
  }[tone];

  return (
    <div className={`rounded-lg border px-4 py-3 shadow-sm ${toneClass}`}>
      <p className="text-[11px] font-semibold uppercase tracking-wide opacity-70">
        {label}
      </p>
      <p className="mt-1 text-2xl font-semibold leading-none">{value}</p>
      <p className="mt-1 text-xs opacity-70">{detail}</p>
    </div>
  );
}

export default function UsersPage() {
  const router = useRouter();
  const me = useCurrentUser();
  const fileRef = useRef<HTMLInputElement>(null);
  const modalRef = useRef<HTMLFormElement>(null);
  const deactivationModalRef = useRef<HTMLDivElement>(null);
  const lastFocusedRef = useRef<HTMLElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const [users, setUsers] = useState<UserRow[]>([]);
  const [page, setPage] = useState(1);
  const [meta, setMeta] = useState<PageMeta | null>(null);
  const [positions, setPositions] = useState<Position[]>([]);
  const [orgUnits, setOrgUnits] = useState<OrgUnit[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [modalError, setModalError] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [deactivationWizard, setDeactivationWizard] =
    useState<DeactivationWizardState | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [actionUserID, setActionUserID] = useState<string | null>(null);
  const [editing, setEditing] = useState<UserRow | null>(null);
  const [form, setForm] = useState<UserFormState | null>(null);
  const [positionQuery, setPositionQuery] = useState("");
  const modalOpen = editing !== null || form !== null;

  async function reload() {
    const [userData, positionData, orgData] = await Promise.all([
      listUsers({ page }),
      listAllPositions(),
      getOrgTree(),
    ]);
    setUsers(userData.data);
    setMeta(userData.meta);
    setPositions(positionData.data);
    setOrgUnits(flattenOrgUnits(orgData.tree));
  }

  useEffect(() => {
    if (me && !me.roles.includes("admin")) {
      router.replace("/organization");
    }
  }, [me, router]);

  useEffect(() => {
    setLoading(true);
    Promise.all([listUsers({ page }), listAllPositions(), getOrgTree()])
      .then(([userData, positionData, orgData]) => {
        setUsers(userData.data);
        setMeta(userData.meta);
        setPositions(positionData.data);
        setOrgUnits(flattenOrgUnits(orgData.tree));
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Gagal memuat pengguna"),
      )
      .finally(() => setLoading(false));
  }, [page]);

  useEffect(() => {
    const activeContainer = modalOpen
      ? modalRef.current
      : deactivationWizard
        ? deactivationModalRef.current
        : null;
    if (!activeContainer) return;

    closeButtonRef.current?.focus();

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        if (busy) return;
        if (deactivationWizard) setDeactivationWizard(null);
        else {
          setEditing(null);
          setForm(null);
          setModalError(null);
        }
        setTimeout(() => lastFocusedRef.current?.focus(), 0);
        return;
      }
      if (event.key !== "Tab" || !activeContainer) return;

      const focusable = Array.from(
        activeContainer.querySelectorAll<HTMLElement>(
          'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
        ),
      );
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [modalOpen, deactivationWizard, busy]);

  const activeCount = useMemo(
    () => users.filter((user) => user.status === "active").length,
    [users],
  );
  const inactiveOrLockedCount = useMemo(
    () => users.filter((user) => user.status !== "active").length,
    [users],
  );
  const unassignedCount = useMemo(
    () => users.filter((user) => (user.positions ?? []).length === 0).length,
    [users],
  );

  function openCreate() {
    lastFocusedRef.current = document.activeElement as HTMLElement | null;
    setEditing(null);
    setForm(emptyForm());
    setPositionQuery("");
    setModalError(null);
  }

  function openEdit(user: UserRow) {
    lastFocusedRef.current = document.activeElement as HTMLElement | null;
    setEditing(user);
    setForm(userToForm(user));
    setPositionQuery("");
    setModalError(null);
  }

  function closeModal() {
    if (busy) return;
    setEditing(null);
    setForm(null);
    setPositionQuery("");
    setModalError(null);
    setTimeout(() => lastFocusedRef.current?.focus(), 0);
  }

  function closeDeactivationWizard() {
    if (busy) return;
    setDeactivationWizard(null);
    setTimeout(() => lastFocusedRef.current?.focus(), 0);
  }

  function toggleRole(role: string) {
    setForm((current) => {
      if (!current) return current;
      const exists = current.roles.includes(role);
      return {
        ...current,
        roles: exists
          ? current.roles.filter((item) => item !== role)
          : [...current.roles, role],
      };
    });
  }

  function addPendingPosition() {
    setForm((current) => {
      if (!current || !current.new_position_id) return current;

      const activePositionIDs = new Set(
        current.positions
          .filter((position) => !current.ended_assignment_ids.includes(position.assignment_id))
          .map((position) => position.position_id),
      );
      current.pending_positions.forEach((position) => activePositionIDs.add(position.position_id));
      if (activePositionIDs.has(current.new_position_id)) {
        setModalError("Jabatan tersebut sudah ada dalam penempatan aktif pengguna ini");
        return current;
      }

      setModalError(null);
      return {
        ...current,
        pending_positions: [
          ...current.pending_positions,
          {
            temp_id: crypto.randomUUID(),
            position_id: current.new_position_id,
            assignment_type: current.new_assignment_type,
          },
        ],
        new_position_unit_level: "",
        new_position_org_unit_id: "",
        new_position_id: "",
        new_assignment_type: "definitive",
      };
    });
  }

  function removePendingPosition(tempID: string) {
    setForm((current) =>
      current
        ? {
            ...current,
            pending_positions: current.pending_positions.filter(
              (position) => position.temp_id !== tempID,
            ),
          }
        : current,
    );
  }

  function endActivePosition(assignmentID: string) {
    setForm((current) =>
      current
        ? {
            ...current,
            ended_assignment_ids: current.ended_assignment_ids.includes(assignmentID)
              ? current.ended_assignment_ids
              : [...current.ended_assignment_ids, assignmentID],
          }
        : current,
    );
  }

  function undoEndActivePosition(assignmentID: string) {
    setForm((current) =>
      current
        ? {
            ...current,
            ended_assignment_ids: current.ended_assignment_ids.filter(
              (currentID) => currentID !== assignmentID,
            ),
          }
        : current,
    );
  }

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setBusy(true);
    setError(null);
    setImportResult(null);
    try {
      setImportResult(await importUsers(file));
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Import gagal");
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setModalError(null);
    try {
      if (form.roles.length === 0) {
        throw new Error("Pilih minimal satu role");
      }
      if (!editing && form.password.trim().length < 10) {
        throw new Error("Password awal minimal 10 karakter");
      }
      if (form.roles.includes("creator") && formPositionPayloads(form).length === 0) {
        throw new Error("Role Creator wajib memiliki minimal satu jabatan aktif");
      }
      const payload = compactPayload(form);
      if (editing) {
        await updateUser(editing.id, payload);
      } else {
        await createUser(payload);
      }
      await reload();
      setEditing(null);
      setForm(null);
    } catch (err) {
      setModalError(err instanceof Error ? err.message : "Gagal menyimpan pengguna");
    } finally {
      setBusy(false);
    }
  }

  async function handleDeactivate(user: UserRow) {
    lastFocusedRef.current = document.activeElement as HTMLElement | null;
    setActionUserID(user.id);
    setError(null);
    try {
      const { impact } = await getUserDeactivationImpact(user.id);
      if (!impact.has_impact) {
        if (!confirm(`Nonaktifkan ${user.full_name} (${user.nik})?`)) return;
        await deactivateUser(user.id);
        await reload();
        return;
      }
      setDeactivationWizard({
        user,
        impact,
        positionReplacements: Object.fromEntries(
          impact.positions.map((position) => [
            position.position_id,
            { replacement_user_id: "", assignment_type: "definitive" as AssignmentType },
          ]),
        ),
        draftTransfers: Object.fromEntries(
          impact.drafts.map((draft) => [
            draft.letter_id,
            { replacement_user_id: "", replacement_position_id: "" },
          ]),
        ),
        error: null,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menonaktifkan");
    } finally {
      setActionUserID(null);
    }
  }

  async function submitDeactivationWizard() {
    if (!deactivationWizard) return;
    setBusy(true);
    setDeactivationWizard((current) => (current ? { ...current, error: null } : current));
    try {
      const payload: DeactivateUserPayload = {
        position_replacements: deactivationWizard.impact.positions.map((position) => {
          const replacement = deactivationWizard.positionReplacements[position.position_id];
          return {
            position_id: position.position_id,
            replacement_user_id: replacement?.replacement_user_id ?? "",
            assignment_type: replacement?.assignment_type ?? "definitive",
          };
        }),
        draft_transfers: deactivationWizard.impact.drafts.map((draft) => {
          const transfer = deactivationWizard.draftTransfers[draft.letter_id];
          return {
            letter_id: draft.letter_id,
            replacement_user_id: transfer?.replacement_user_id ?? "",
            replacement_position_id: transfer?.replacement_position_id ?? "",
          };
        }),
      };
      await deactivateUser(deactivationWizard.user.id, payload);
      await reload();
      setDeactivationWizard(null);
    } catch (err) {
      setDeactivationWizard((current) =>
        current
          ? {
              ...current,
              error: err instanceof Error ? err.message : "Gagal menonaktifkan pengguna",
            }
          : current,
      );
    } finally {
      setBusy(false);
    }
  }

  async function handleReactivate(user: UserRow) {
    setActionUserID(user.id);
    setError(null);
    try {
      await updateUser(user.id, {
        nik: user.nik,
        email: user.email,
        full_name: user.full_name,
        status: "active",
        roles: user.roles,
        positions: user.positions.map((position) => ({
          position_id: position.position_id,
          assignment_type: position.assignment_type,
        })),
      });
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengaktifkan");
    } finally {
      setActionUserID(null);
    }
  }

  const visibleFormPositions =
    form?.positions.filter(
      (position) => !form.ended_assignment_ids.includes(position.assignment_id),
    ) ?? [];
  const endedFormPositions =
    form?.positions.filter((position) =>
      form.ended_assignment_ids.includes(position.assignment_id),
    ) ?? [];
  const selectedFormPositionIDs = new Set([
    ...(form?.positions.map((position) => position.position_id) ?? []),
    ...(form?.pending_positions.map((position) => position.position_id) ?? []),
  ]);
  const normalizedPositionQuery = positionQuery.trim().toLowerCase();
  const selectedNewPositionUnitID = form?.new_position_org_unit_id ?? "";
  const selectedNewPositionUnitLevel = form?.new_position_unit_level ?? "";
  const departmentDescendantIDs =
    selectedNewPositionUnitLevel === "department" && selectedNewPositionUnitID
      ? descendantUnitIDs(orgUnits, selectedNewPositionUnitID)
      : new Set<string>();
  const allowedPositionTypes =
    USER_POSITION_TYPES_BY_UNIT_LEVEL[selectedNewPositionUnitLevel] ?? [];
  const filteredAvailablePositions = positions.filter((position) => {
    const typeOK = allowedPositionTypes.includes(position.position_type);
    const unitOK =
      selectedNewPositionUnitID.length > 0 &&
      (selectedNewPositionUnitLevel === "department"
        ? ((position.position_type === "dept_head" ||
            position.position_type === "sub_dept_head") &&
            position.org_unit_id === selectedNewPositionUnitID) ||
          (position.position_type === "division_head" &&
            departmentDescendantIDs.has(position.org_unit_id))
        : position.org_unit_id === selectedNewPositionUnitID);
    const queryOK =
      normalizedPositionQuery.length === 0 ||
      positionSearchText(position).includes(normalizedPositionQuery);
    return typeOK && unitOK && queryOK;
  });
  const positionOptions = filteredAvailablePositions.sort((a, b) => {
    const typeCompare =
      allowedPositionTypes.indexOf(a.position_type) -
      allowedPositionTypes.indexOf(b.position_type);
    if (typeCompare !== 0) return typeCompare;
    const unitCompare = a.org_unit_name.localeCompare(b.org_unit_name);
    if (unitCompare !== 0) return unitCompare;
    return a.title.localeCompare(b.title);
  });
  const sortedOrgUnits = useMemo(
    () =>
      [...orgUnits].sort((a, b) => {
        const levelCompare =
          UNIT_LEVEL_OPTIONS.findIndex((option) => option.value === a.unit_level) -
          UNIT_LEVEL_OPTIONS.findIndex((option) => option.value === b.unit_level);
        if (levelCompare !== 0) return levelCompare;
        return a.name.localeCompare(b.name);
      }),
    [orgUnits],
  );
  const orgUnitOptions = sortedOrgUnits.filter(
    (unit) => unit.unit_level === form?.new_position_unit_level,
  );
  const pendingPositionDetails = new Map(
    positions.map((position) => [position.id, position]),
  );
  const creatorMissingPosition =
    form?.roles.includes("creator") && formPositionPayloads(form).length === 0;
  const activeReplacementCandidates = users.filter(
    (user) => user.status === "active" && user.id !== deactivationWizard?.user.id,
  );

  function draftPositionOptions(userID: string) {
    const selectedUser = users.find((user) => user.id === userID);
    const options =
      selectedUser?.positions.map((position) => ({
        position_id: position.position_id,
        label: assignmentLabel(position),
      })) ?? [];
    if (!deactivationWizard) return options;

    deactivationWizard.impact.positions.forEach((position) => {
      const replacement =
        deactivationWizard.positionReplacements[position.position_id];
      if (replacement?.replacement_user_id !== userID) return;
      if (options.some((option) => option.position_id === position.position_id)) return;
      options.push({
        position_id: position.position_id,
        label: `${position.title} · ${position.org_unit_name}`,
      });
    });
    return options;
  }

  return (
    <>
      <main className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 px-4 py-6 sm:px-6">
        <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900 sm:p-5">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div className="min-w-0">
              <p className="text-xs font-semibold uppercase tracking-wide text-navy-600 dark:text-sky-300">
                Administrasi Akses
              </p>
              <h1 className="mt-1 text-2xl font-semibold text-zinc-950 dark:text-zinc-50">
                Pengguna
              </h1>
              <p className="mt-1 max-w-2xl text-sm text-zinc-500">
                Kelola akun, role, status, dan penempatan jabatan pengguna eOffice Pro.
              </p>
            </div>

            <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
              <button
                type="button"
                onClick={() =>
                  downloadImportTemplate().catch(() =>
                    setError("Gagal mengunduh template"),
                  )
                }
                className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                <DownloadIcon className="h-4 w-4" />
                Unduh Template
              </button>
              <label className="inline-flex h-10 cursor-pointer items-center justify-center gap-2 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-300 dark:hover:bg-zinc-800">
                <UploadIcon className="h-4 w-4" />
                {busy ? "Memproses..." : "Import Excel"}
                <input
                  ref={fileRef}
                  type="file"
                  accept=".xlsx"
                  onChange={handleImport}
                  disabled={busy}
                  className="hidden"
                />
              </label>
              <button
                type="button"
                onClick={openCreate}
                className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-navy-700 px-4 text-sm font-semibold text-white shadow-sm transition hover:bg-navy-800"
              >
                <PlusIcon className="h-4 w-4" />
                Tambah Pengguna
              </button>
            </div>
          </div>

          <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryTile
              label="Total Pengguna"
              value={users.length}
              detail="Akun terdaftar"
              tone="navy"
            />
            <SummaryTile
              label="Aktif"
              value={activeCount}
              detail="Siap memakai aplikasi"
              tone="emerald"
            />
            <SummaryTile
              label="Nonaktif / Locked"
              value={inactiveOrLockedCount}
              detail="Butuh tindak lanjut"
              tone="amber"
            />
            <SummaryTile
              label="Tanpa Jabatan"
              value={unassignedCount}
              detail="Belum ditempatkan"
            />
          </div>
        </section>

        {error && (
          <p
            role="alert"
            className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
          >
            {error}
          </p>
        )}

        {importResult && (
          <div className="rounded-lg border border-zinc-200 bg-white px-4 py-3 text-sm shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
            <p className="font-medium text-zinc-900 dark:text-zinc-100">
              Import selesai: {importResult.imported} berhasil,{" "}
              {importResult.failed} gagal.
            </p>
            {importResult.errors.length > 0 && (
              <ul className="mt-2 list-inside list-disc text-red-700 dark:text-red-300">
                {importResult.errors.map((item) => (
                  <li key={item.row}>
                    Baris {item.row}: {item.error}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        <section className="hidden overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900 lg:block">
          <div className="border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                  Daftar Pengguna
                </p>
                <p className="text-xs text-zinc-500">
                  {activeCount} aktif pada halaman ini dari {meta?.total ?? users.length} total pengguna
                </p>
              </div>
              <span className="rounded-full bg-zinc-100 px-2.5 py-1 text-xs font-semibold text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                {positions.length} jabatan tersedia
              </span>
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[960px] text-left text-sm">
              <thead className="border-b border-zinc-200 bg-zinc-50 text-xs uppercase tracking-wide text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
                <tr>
                  <th className="px-4 py-3">Pengguna</th>
                  <th className="px-4 py-3">Akses</th>
                  <th className="px-4 py-3">Jabatan Aktif</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-zinc-500">
                      Memuat pengguna...
                    </td>
                  </tr>
                )}
                {!loading && users.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-zinc-500">
                      Belum ada pengguna.
                    </td>
                  </tr>
                )}
                {!loading &&
                  users.map((user) => {
                    const status = user.status as UserPayload["status"];
                    const actionBusy = actionUserID === user.id;
                    const userPositions = user.positions ?? [];

                    return (
                      <tr
                        key={user.id}
                        className="border-b border-zinc-100 align-top transition hover:bg-zinc-50/70 last:border-0 dark:border-zinc-800/60 dark:hover:bg-zinc-800/40"
                      >
                        <td className="px-4 py-4">
                          <div className="flex items-start gap-3">
                            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-navy-50 text-xs font-bold text-navy-700 ring-1 ring-navy-100 dark:bg-navy-950 dark:text-sky-200 dark:ring-navy-900">
                              {initialsOf(user.full_name) || "U"}
                            </div>
                            <div className="min-w-0">
                              <div className="flex flex-wrap items-center gap-2">
                                <p className="break-words font-semibold text-zinc-950 dark:text-zinc-50">
                                  {user.full_name}
                                </p>
                                {user.id === me?.id && (
                                  <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-700 dark:bg-sky-950 dark:text-sky-300">
                                    Anda
                                  </span>
                                )}
                              </div>
                              <p className="mt-1 break-all text-xs text-zinc-500">
                                {user.email}
                              </p>
                              <p className="mt-1 font-mono text-[11px] font-semibold text-zinc-400">
                                NIK {user.nik}
                              </p>
                            </div>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex flex-wrap gap-1">
                            {user.roles.length > 0 ? (
                              user.roles.map((role) => (
                                <span
                                  key={role}
                                  className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
                                >
                                  {roleLabel(role)}
                                </span>
                              ))
                            ) : (
                              <span className="text-zinc-400">-</span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3 text-zinc-600 dark:text-zinc-400">
                          {userPositions.length > 0 ? (
                            <div className="grid max-w-md gap-1.5">
                              {userPositions.slice(0, 2).map((position) => (
                                <span key={position.assignment_id} className="text-xs">
                                  {assignmentLabel(position)}
                                  <span
                                    className={`ml-2 rounded-full px-1.5 py-0.5 text-[10px] font-semibold ${ASSIGNMENT_STYLE[position.assignment_type]}`}
                                  >
                                    {ASSIGNMENT_LABEL[position.assignment_type]}
                                  </span>
                                </span>
                              ))}
                              {userPositions.length > 2 && (
                                <span className="text-xs font-semibold text-zinc-400">
                                  +{userPositions.length - 2} jabatan lain
                                </span>
                              )}
                            </div>
                          ) : (
                            <span className="text-xs text-amber-700 dark:text-amber-300">
                              Belum ditempatkan
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${STATUS_STYLE[status] ?? STATUS_STYLE.inactive}`}
                          >
                            {STATUS_LABEL[status] ?? user.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex justify-end gap-2">
                            <button
                              type="button"
                              onClick={() => openEdit(user)}
                              className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-zinc-200 px-2.5 text-xs font-semibold text-zinc-600 transition hover:bg-zinc-100 hover:text-zinc-950 dark:border-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-800 dark:hover:text-white"
                            >
                              <EditIcon className="h-3.5 w-3.5" />
                              Edit
                            </button>
                            {user.status === "active" ? (
                              <button
                                type="button"
                                onClick={() => handleDeactivate(user)}
                                disabled={actionBusy}
                                className="inline-flex h-8 items-center rounded-lg border border-red-200 px-2.5 text-xs font-semibold text-red-600 transition hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950"
                              >
                                {actionBusy ? "Memproses" : "Nonaktifkan"}
                              </button>
                            ) : (
                              <button
                                type="button"
                                onClick={() => handleReactivate(user)}
                                disabled={actionBusy}
                                className="inline-flex h-8 items-center rounded-lg border border-emerald-200 px-2.5 text-xs font-semibold text-emerald-700 transition hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-emerald-900 dark:text-emerald-400 dark:hover:bg-emerald-950"
                              >
                                {actionBusy ? "Memproses" : "Aktifkan"}
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
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
            <p className="rounded-xl border border-zinc-200 bg-white px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900">
              Memuat pengguna...
            </p>
          )}
          {!loading && users.length === 0 && (
            <p className="rounded-xl border border-zinc-200 bg-white px-4 py-8 text-center text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900">
              Belum ada pengguna.
            </p>
          )}
          {!loading &&
            users.map((user) => {
              const status = user.status as UserPayload["status"];
              const actionBusy = actionUserID === user.id;
              const userPositions = user.positions ?? [];

              return (
                <article
                  key={user.id}
                  className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
                >
                  <div className="flex items-start gap-3">
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-navy-50 text-sm font-bold text-navy-700 ring-1 ring-navy-100 dark:bg-navy-950 dark:text-sky-200 dark:ring-navy-900">
                      {initialsOf(user.full_name) || "U"}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h2 className="break-words text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                          {user.full_name}
                        </h2>
                        <span
                          className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${STATUS_STYLE[status] ?? STATUS_STYLE.inactive}`}
                        >
                          {STATUS_LABEL[status] ?? user.status}
                        </span>
                        {user.id === me?.id && (
                          <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[11px] font-semibold text-sky-700 dark:bg-sky-950 dark:text-sky-300">
                            Anda
                          </span>
                        )}
                      </div>
                      <p className="mt-1 break-all text-xs text-zinc-500">{user.email}</p>
                      <p className="mt-1 font-mono text-[11px] font-semibold text-zinc-400">
                        NIK {user.nik}
                      </p>
                    </div>
                  </div>

                  <div className="mt-3 flex flex-wrap gap-1.5">
                    {user.roles.length > 0 ? (
                      user.roles.map((role) => (
                        <span
                          key={role}
                          className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
                        >
                          {roleLabel(role)}
                        </span>
                      ))
                    ) : (
                      <span className="text-xs text-zinc-400">Tanpa role</span>
                    )}
                  </div>

                  <div className="mt-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 dark:border-zinc-800 dark:bg-zinc-950/50">
                    <p className="text-[11px] font-semibold uppercase tracking-wide text-zinc-400">
                      Jabatan Aktif
                    </p>
                    {userPositions.length > 0 ? (
                      <div className="mt-2 grid gap-1.5">
                        {userPositions.slice(0, 3).map((position) => (
                          <p
                            key={position.assignment_id}
                            className="break-words text-xs text-zinc-700 dark:text-zinc-300"
                          >
                            {assignmentLabel(position)}
                            <span
                              className={`ml-2 rounded-full px-1.5 py-0.5 text-[10px] font-semibold ${ASSIGNMENT_STYLE[position.assignment_type]}`}
                            >
                              {ASSIGNMENT_LABEL[position.assignment_type]}
                            </span>
                          </p>
                        ))}
                        {userPositions.length > 3 && (
                          <p className="text-xs font-semibold text-zinc-400">
                            +{userPositions.length - 3} jabatan lain
                          </p>
                        )}
                      </div>
                    ) : (
                      <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">
                        Belum ditempatkan
                      </p>
                    )}
                  </div>

                  <div className="mt-3 flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={() => openEdit(user)}
                      className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-zinc-300 px-3 text-xs font-semibold text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                    >
                      <EditIcon className="h-3.5 w-3.5" />
                      Edit
                    </button>
                    {user.status === "active" ? (
                      <button
                        type="button"
                        onClick={() => handleDeactivate(user)}
                        disabled={actionBusy}
                        className="h-9 rounded-lg border border-red-200 px-3 text-xs font-semibold text-red-600 transition hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950"
                      >
                        {actionBusy ? "Memproses" : "Nonaktifkan"}
                      </button>
                    ) : (
                      <button
                        type="button"
                        onClick={() => handleReactivate(user)}
                        disabled={actionBusy}
                        className="h-9 rounded-lg border border-emerald-200 px-3 text-xs font-semibold text-emerald-700 transition hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-emerald-900 dark:text-emerald-400 dark:hover:bg-emerald-950"
                      >
                        {actionBusy ? "Memproses" : "Aktifkan"}
                      </button>
                    )}
                  </div>
                </article>
              );
            })}
          <Pagination
            page={page}
            totalPages={meta?.total_pages ?? 1}
            onPageChange={setPage}
            disabled={loading}
          />
        </section>
      </main>

      {modalOpen && form && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="user-form-title"
          className="fixed inset-0 z-50 flex items-stretch justify-center overflow-y-auto bg-zinc-950/50 px-3 py-4 sm:items-center sm:px-6"
        >
          <form
            ref={modalRef}
            onSubmit={handleSubmit}
            className="flex max-h-full w-full max-w-4xl flex-col overflow-hidden rounded-xl bg-white shadow-2xl dark:bg-zinc-900 sm:max-h-[calc(100vh-2rem)]"
          >
            <div className="flex shrink-0 items-start justify-between gap-4 border-b border-zinc-200 px-4 py-4 dark:border-zinc-800 sm:px-6">
              <div className="min-w-0">
                <p className="text-xs font-semibold uppercase tracking-wide text-navy-600 dark:text-sky-300">
                  {editing ? "Perbarui Akun" : "Akun Baru"}
                </p>
                <h2
                  id="user-form-title"
                  className="mt-1 text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  {editing ? "Edit Pengguna" : "Tambah Pengguna"}
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  Identitas, hak akses, dan penempatan jabatan dikelola dari satu form.
                </p>
              </div>
              <button
                ref={closeButtonRef}
                type="button"
                onClick={closeModal}
                className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-zinc-200 text-zinc-500 transition hover:bg-zinc-100 hover:text-zinc-900 dark:border-zinc-800 dark:hover:bg-zinc-800 dark:hover:text-white"
                aria-label="Tutup"
              >
                <XIcon className="h-4 w-4" />
              </button>
            </div>

            <div className="grid flex-1 gap-4 overflow-y-auto px-4 py-4 sm:px-6">
              <section className="rounded-xl border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900">
                <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h3 className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                      Identitas
                    </h3>
                    <p className="mt-1 text-xs text-zinc-500">
                      Data utama pengguna yang terlihat di workflow surat.
                    </p>
                  </div>
                  <span
                    className={`rounded-full px-2.5 py-1 text-xs font-semibold ${STATUS_STYLE[form.status]}`}
                  >
                    {STATUS_LABEL[form.status]}
                  </span>
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    NIK
                    <input
                      value={form.nik}
                      onChange={(e) =>
                        setForm((current) =>
                          current ? { ...current, nik: e.target.value } : current,
                        )
                      }
                      required
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    />
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Email
                    <input
                      type="email"
                      value={form.email}
                      onChange={(e) =>
                        setForm((current) =>
                          current ? { ...current, email: e.target.value } : current,
                        )
                      }
                      required
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    />
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200 sm:col-span-2">
                    Nama Lengkap
                    <input
                      value={form.full_name}
                      onChange={(e) =>
                        setForm((current) =>
                          current ? { ...current, full_name: e.target.value } : current,
                        )
                      }
                      required
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    />
                  </label>
                </div>
              </section>

              <section className="rounded-xl border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900">
                <div className="mb-4">
                  <h3 className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                    Akses
                  </h3>
                  <p className="mt-1 text-xs text-zinc-500">
                    Status akun, role aplikasi, dan password awal atau reset.
                  </p>
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    Status
                    <select
                      value={form.status}
                      onChange={(e) =>
                        setForm((current) =>
                          current
                            ? {
                                ...current,
                                status: e.target.value as UserPayload["status"],
                              }
                            : current,
                        )
                      }
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    >
                      <option value="active">Aktif</option>
                      <option value="inactive">Nonaktif</option>
                      <option value="locked">Terkunci</option>
                    </select>
                  </label>
                  <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                    {editing ? "Reset Password" : "Password Awal"}
                    <input
                      type="password"
                      value={form.password}
                      onChange={(e) =>
                        setForm((current) =>
                          current ? { ...current, password: e.target.value } : current,
                        )
                      }
                      required={!editing}
                      minLength={editing ? undefined : 10}
                      placeholder={
                        editing
                          ? "Kosongkan jika password tetap"
                          : "Minimal 10 karakter"
                      }
                      className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                    />
                  </label>

                  <fieldset className="sm:col-span-2">
                    <legend className="mb-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                      Role
                    </legend>
                    <div className="flex flex-wrap gap-2">
                      {ROLE_OPTIONS.map((role) => {
                        const checked = form.roles.includes(role.value);

                        return (
                          <label
                            key={role.value}
                            className={`inline-flex h-10 cursor-pointer items-center rounded-lg border px-3 text-sm font-semibold transition ${
                              checked
                                ? "border-navy-600 bg-navy-50 text-navy-800 shadow-sm dark:border-sky-700 dark:bg-navy-950 dark:text-sky-200"
                                : "border-zinc-300 bg-white text-zinc-600 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-300 dark:hover:bg-zinc-800"
                            }`}
                          >
                            <input
                              type="checkbox"
                              checked={checked}
                              onChange={() => toggleRole(role.value)}
                              className="sr-only"
                            />
                            {role.label}
                          </label>
                        );
                      })}
                    </div>
                  </fieldset>
                </div>
              </section>

              <fieldset className="rounded-xl border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900">
                <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <legend className="text-sm font-semibold text-zinc-950 dark:text-zinc-50">
                      Penempatan Jabatan
                    </legend>
                    <p className="mt-1 text-xs text-zinc-500">
                      Creator wajib memiliki minimal satu jabatan aktif.
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                      {visibleFormPositions.length} aktif
                    </span>
                    <span className="rounded-full bg-navy-100 px-2 py-0.5 text-[11px] font-semibold text-navy-800 dark:bg-navy-950 dark:text-sky-300">
                      {form.pending_positions.length} baru
                    </span>
                    {endedFormPositions.length > 0 && (
                      <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">
                        {endedFormPositions.length} diakhiri
                      </span>
                    )}
                  </div>
                </div>

                <div className="grid gap-3 rounded-lg border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950/50">
                  {visibleFormPositions.length === 0 && form.pending_positions.length === 0 && (
                    <p className="rounded-lg border border-dashed border-amber-300 bg-amber-50 px-3 py-3 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300">
                      Belum ada jabatan aktif untuk pengguna ini.
                    </p>
                  )}
                  {visibleFormPositions.map((position) => (
                    <div
                      key={position.assignment_id}
                      className="grid gap-3 rounded-lg border border-zinc-200 bg-white px-3 py-3 dark:border-zinc-800 dark:bg-zinc-900 sm:grid-cols-[1fr_auto]"
                    >
                      <div className="min-w-0">
                        <p className="break-words text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                          {assignmentLabel(position)}
                        </p>
                        <span
                          className={`mt-1 inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold ${ASSIGNMENT_STYLE[position.assignment_type]}`}
                        >
                          {ASSIGNMENT_LABEL[position.assignment_type]}
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => endActivePosition(position.assignment_id)}
                        className="h-9 rounded-lg border border-red-200 px-3 text-xs font-semibold text-red-700 transition hover:bg-red-50 dark:border-red-900 dark:text-red-300 dark:hover:bg-red-950"
                      >
                        Akhiri
                      </button>
                    </div>
                  ))}
                  {endedFormPositions.map((position) => (
                    <div
                      key={position.assignment_id}
                      className="grid gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-3 dark:border-amber-900 dark:bg-amber-950/40 sm:grid-cols-[1fr_auto]"
                    >
                      <div className="min-w-0">
                        <p className="break-words text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                          {assignmentLabel(position)}
                        </p>
                        <span className="mt-1 inline-flex rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">
                          Akan diakhiri
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => undoEndActivePosition(position.assignment_id)}
                        className="h-9 rounded-lg border border-amber-300 px-3 text-xs font-semibold text-amber-800 transition hover:bg-amber-100 dark:border-amber-800 dark:text-amber-300 dark:hover:bg-amber-950"
                      >
                        Batalkan
                      </button>
                    </div>
                  ))}
                  {form.pending_positions.map((assignment) => {
                    const position = pendingPositionDetails.get(assignment.position_id);

                    return (
                      <div
                        key={assignment.temp_id}
                        className="grid gap-3 rounded-lg border border-dashed border-navy-300 bg-white px-3 py-3 dark:border-sky-800 dark:bg-zinc-900 sm:grid-cols-[1fr_auto]"
                      >
                        <div className="min-w-0">
                          <p className="break-words text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                            {position ? positionLabel(position) : "Jabatan dipilih"}
                          </p>
                          <span
                            className={`mt-1 inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold ${ASSIGNMENT_STYLE[assignment.assignment_type]}`}
                          >
                            {ASSIGNMENT_LABEL[assignment.assignment_type]} baru
                          </span>
                        </div>
                        <button
                          type="button"
                          onClick={() => removePendingPosition(assignment.temp_id)}
                          className="h-9 rounded-lg border border-zinc-300 px-3 text-xs font-semibold text-zinc-700 transition hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                        >
                          Hapus
                        </button>
                      </div>
                    );
                  })}
                </div>

                <div className="mt-3 grid gap-3 rounded-lg border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900">
                  <div className="grid gap-3 md:grid-cols-2">
                    <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                      Level Unit
                      <select
                        value={form.new_position_unit_level}
                        onChange={(e) => {
                          setPositionQuery("");
                          setForm((current) =>
                            current
                              ? {
                                  ...current,
                                  new_position_unit_level: e.target.value,
                                  new_position_org_unit_id: "",
                                  new_position_id: "",
                                }
                              : current,
                          );
                        }}
                        className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
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
                        value={form.new_position_org_unit_id}
                        onChange={(e) => {
                          setPositionQuery("");
                          setForm((current) =>
                            current
                              ? {
                                  ...current,
                                  new_position_org_unit_id: e.target.value,
                                  new_position_id: "",
                                }
                              : current,
                          );
                        }}
                        disabled={!form.new_position_unit_level}
                        className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                      >
                        <option value="">
                          {form.new_position_unit_level
                            ? "Pilih unit organisasi"
                            : "Pilih level dahulu"}
                        </option>
                        {orgUnitOptions.map((unit) => (
                          <option key={unit.id} value={unit.id}>
                            {orgUnitLabel(unit)}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>

                  <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                    <label className="relative block text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                      <span className="mb-2 block">Cari Jabatan</span>
                      <SearchIcon className="pointer-events-none absolute bottom-3 left-3 h-4 w-4 text-zinc-400" />
                      <input
                        type="search"
                        value={positionQuery}
                        onChange={(e) => setPositionQuery(e.target.value)}
                        placeholder="Nama, tipe, atau pemegang..."
                        disabled={!form.new_position_org_unit_id}
                        className="h-10 w-full rounded-lg border border-zinc-300 bg-white pl-9 pr-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                      />
                    </label>
                    <label className="flex min-w-0 flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                      Jabatan
                      <select
                        value={form.new_position_id}
                        onChange={(e) =>
                          setForm((current) =>
                            current
                              ? {
                                  ...current,
                                  new_position_id: e.target.value,
                                }
                              : current,
                          )
                        }
                        disabled={!form.new_position_org_unit_id}
                        className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                      >
                        <option value="">
                          {form.new_position_org_unit_id
                            ? "Pilih jabatan"
                            : "Pilih unit dahulu"}
                        </option>
                        {form.new_position_unit_level === "department"
                          ? USER_POSITION_TYPES_BY_UNIT_LEVEL.department.map(
                              (positionType) => {
                                const groupedPositions = positionOptions.filter(
                                  (position) =>
                                    position.position_type === positionType,
                                );
                                return (
                                  <optgroup
                                    key={positionType}
                                    label={
                                      POSITION_TYPE_LABEL[positionType] ??
                                      positionType
                                    }
                                  >
                                    {groupedPositions.length === 0 ? (
                                      <option
                                        value={`unavailable-${positionType}`}
                                        disabled
                                      >
                                        Belum ada jabatan tersedia
                                      </option>
                                    ) : (
                                      groupedPositions.map((position) => {
                                        const alreadySelected =
                                          selectedFormPositionIDs.has(position.id);
                                        return (
                                          <option
                                            key={position.id}
                                            value={position.id}
                                            disabled={alreadySelected}
                                          >
                                            {position.title} ·{" "}
                                            {position.org_unit_name}
                                            {alreadySelected
                                              ? " · sudah ditempatkan"
                                              : position.holder_name
                                                ? ` · saat ini: ${position.holder_name}`
                                                : ""}
                                          </option>
                                        );
                                      })
                                    )}
                                  </optgroup>
                                );
                              },
                            )
                          : positionOptions.map((position) => {
                              const alreadySelected =
                                selectedFormPositionIDs.has(position.id);
                              return (
                                <option
                                  key={position.id}
                                  value={position.id}
                                  disabled={alreadySelected}
                                >
                                  {position.title}
                                  {alreadySelected
                                    ? " · sudah ditempatkan"
                                    : position.holder_name
                                      ? ` · saat ini: ${position.holder_name}`
                                      : ""}
                                </option>
                              );
                            })}
                      </select>
                    </label>
                  </div>

                  <div className="grid gap-3 md:grid-cols-[160px_auto]">
                    <label className="flex flex-col gap-2 text-sm font-semibold text-zinc-800 dark:text-zinc-200">
                      Tipe
                      <select
                        value={form.new_assignment_type}
                        onChange={(e) =>
                          setForm((current) =>
                            current
                              ? {
                                  ...current,
                                  new_assignment_type: e.target.value as AssignmentType,
                                }
                              : current,
                          )
                        }
                        disabled={!form.new_position_id}
                        className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                      >
                        <option value="definitive">Definitif</option>
                        <option value="plt">Plt</option>
                        <option value="plh">Plh</option>
                      </select>
                    </label>
                    <div className="flex items-end">
                      <button
                        type="button"
                        onClick={addPendingPosition}
                        disabled={!form.new_position_id}
                        className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-navy-600 px-4 text-sm font-semibold text-navy-700 transition hover:bg-navy-50 disabled:cursor-not-allowed disabled:border-zinc-300 disabled:text-zinc-400 dark:border-sky-600 dark:text-sky-300 dark:hover:bg-navy-950 dark:disabled:border-zinc-700 dark:disabled:text-zinc-500 md:w-auto"
                      >
                        <PlusIcon className="h-4 w-4" />
                        Tambah
                      </button>
                    </div>
                  </div>
                  {positionOptions.length === 0 && (
                    <p className="text-xs text-amber-700 dark:text-amber-300">
                      {form.new_position_org_unit_id
                        ? "Tidak ada jabatan yang cocok dengan filter."
                        : "Pilih level dan unit organisasi terlebih dahulu."}
                    </p>
                  )}
                </div>

                <p className="mt-3 text-xs font-normal text-zinc-500">
                  {creatorMissingPosition
                    ? "Role Creator wajib memiliki minimal satu jabatan aktif sebelum pengguna bisa disimpan."
                    : "Pengguna dengan role pembuat surat harus memiliki jabatan aktif agar dapat memakai halaman Tulis Surat."}
                </p>
              </fieldset>

              {modalError && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  {modalError}
                </p>
              )}
            </div>

            <div className="flex shrink-0 flex-col-reverse gap-2 border-t border-zinc-200 bg-white px-4 py-4 dark:border-zinc-800 dark:bg-zinc-900 sm:flex-row sm:justify-end sm:px-6">
              <button
                type="button"
                onClick={closeModal}
                disabled={busy}
                className="h-10 rounded-lg border border-zinc-300 px-4 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={busy || Boolean(creatorMissingPosition)}
                className="h-10 rounded-lg bg-navy-700 px-4 text-sm font-semibold text-white shadow-sm transition hover:bg-navy-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busy ? "Menyimpan..." : editing ? "Simpan Perubahan" : "Tambah Pengguna"}
              </button>
            </div>
          </form>
        </div>
      )}

      {deactivationWizard && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="deactivation-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
        >
          <div
            ref={deactivationModalRef}
            className="max-h-full w-full max-w-3xl overflow-y-auto rounded-xl bg-white shadow-2xl dark:bg-zinc-900"
          >
            <div className="flex items-start justify-between border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <div>
                <h2
                  id="deactivation-title"
                  className="text-lg font-semibold text-zinc-950 dark:text-zinc-50"
                >
                  Nonaktifkan {deactivationWizard.user.full_name}
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  Pilih pengganti agar jabatan, draft, dan approval berjalan tidak yatim.
                </p>
              </div>
              <button
                ref={closeButtonRef}
                type="button"
                onClick={closeDeactivationWizard}
                className="rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-semibold text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:border-zinc-800 dark:hover:bg-zinc-800 dark:hover:text-white"
                aria-label="Tutup"
              >
                Tutup
              </button>
            </div>

            <div className="grid gap-5 px-6 py-5">
              {deactivationWizard.impact.positions.length > 0 && (
                <section className="grid gap-3">
                  <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                    Pengganti Jabatan
                  </h3>
                  {deactivationWizard.impact.positions.map((position) => {
                    const replacement =
                      deactivationWizard.positionReplacements[position.position_id];

                    return (
                      <div
                        key={position.position_id}
                        className="grid gap-3 rounded-lg border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950/50 sm:grid-cols-[1fr_220px_140px]"
                      >
                        <div>
                          <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                            {position.title}
                          </p>
                          <p className="text-xs text-zinc-500">{position.org_unit_name}</p>
                        </div>
                        <label className="flex flex-col gap-1 text-xs font-semibold text-zinc-700 dark:text-zinc-300">
                          Pengganti
                          <select
                            value={replacement?.replacement_user_id ?? ""}
                            onChange={(e) =>
                              setDeactivationWizard((current) =>
                                current
                                  ? {
                                      ...current,
                                      positionReplacements: {
                                        ...current.positionReplacements,
                                        [position.position_id]: {
                                          replacement_user_id: e.target.value,
                                          assignment_type:
                                            current.positionReplacements[position.position_id]
                                              ?.assignment_type ?? "definitive",
                                        },
                                      },
                                    }
                                  : current,
                              )
                            }
                            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                          >
                            <option value="">Pilih pengguna</option>
                            {activeReplacementCandidates.map((candidate) => (
                              <option key={candidate.id} value={candidate.id}>
                                {candidate.full_name} ({candidate.nik})
                              </option>
                            ))}
                          </select>
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold text-zinc-700 dark:text-zinc-300">
                          Tipe
                          <select
                            value={replacement?.assignment_type ?? "definitive"}
                            onChange={(e) =>
                              setDeactivationWizard((current) =>
                                current
                                  ? {
                                      ...current,
                                      positionReplacements: {
                                        ...current.positionReplacements,
                                        [position.position_id]: {
                                          replacement_user_id:
                                            current.positionReplacements[position.position_id]
                                              ?.replacement_user_id ?? "",
                                          assignment_type: e.target.value as AssignmentType,
                                        },
                                      },
                                    }
                                  : current,
                              )
                            }
                            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                          >
                            <option value="definitive">Definitif</option>
                            <option value="plt">Plt</option>
                            <option value="plh">Plh</option>
                          </select>
                        </label>
                      </div>
                    );
                  })}
                </section>
              )}

              {deactivationWizard.impact.drafts.length > 0 && (
                <section className="grid gap-3">
                  <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                    Alihkan Draft/Revisi
                  </h3>
                  {deactivationWizard.impact.drafts.map((draft) => {
                    const transfer = deactivationWizard.draftTransfers[draft.letter_id];
                    const positionOptions = draftPositionOptions(
                      transfer?.replacement_user_id ?? "",
                    );

                    return (
                      <div
                        key={draft.letter_id}
                        className="grid gap-3 rounded-lg border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950/50 sm:grid-cols-[1fr_220px_220px]"
                      >
                        <div>
                          <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                            {draft.subject}
                          </p>
                          <p className="text-xs text-zinc-500">
                            {draft.creator_position_title} · {STATUS_LABEL[draft.status as UserPayload["status"]] ?? draft.status}
                          </p>
                        </div>
                        <label className="flex flex-col gap-1 text-xs font-semibold text-zinc-700 dark:text-zinc-300">
                          Pemilik Baru
                          <select
                            value={transfer?.replacement_user_id ?? ""}
                            onChange={(e) =>
                              setDeactivationWizard((current) =>
                                current
                                  ? {
                                      ...current,
                                      draftTransfers: {
                                        ...current.draftTransfers,
                                        [draft.letter_id]: {
                                          replacement_user_id: e.target.value,
                                          replacement_position_id: "",
                                        },
                                      },
                                    }
                                  : current,
                              )
                            }
                            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                          >
                            <option value="">Pilih pengguna</option>
                            {activeReplacementCandidates.map((candidate) => (
                              <option key={candidate.id} value={candidate.id}>
                                {candidate.full_name} ({candidate.nik})
                              </option>
                            ))}
                          </select>
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold text-zinc-700 dark:text-zinc-300">
                          Jabatan Baru
                          <select
                            value={transfer?.replacement_position_id ?? ""}
                            disabled={!transfer?.replacement_user_id}
                            onChange={(e) =>
                              setDeactivationWizard((current) =>
                                current
                                  ? {
                                      ...current,
                                      draftTransfers: {
                                        ...current.draftTransfers,
                                        [draft.letter_id]: {
                                          replacement_user_id:
                                            current.draftTransfers[draft.letter_id]
                                              ?.replacement_user_id ?? "",
                                          replacement_position_id: e.target.value,
                                        },
                                      },
                                    }
                                  : current,
                              )
                            }
                            className="h-10 rounded-lg border border-zinc-300 bg-white px-3 text-sm font-normal text-zinc-950 outline-none focus:border-navy-500 focus:ring-2 focus:ring-navy-500/15 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-50"
                          >
                            <option value="">Pilih jabatan</option>
                            {positionOptions.map((option) => (
                              <option key={option.position_id} value={option.position_id}>
                                {option.label}
                              </option>
                            ))}
                          </select>
                        </label>
                      </div>
                    );
                  })}
                </section>
              )}

              {deactivationWizard.impact.approval_steps.length > 0 && (
                <section className="grid gap-3">
                  <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                    Approval Terdampak
                  </h3>
                  <div className="grid gap-2 rounded-lg border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950/50">
                    {deactivationWizard.impact.approval_steps.map((step) => (
                      <p
                        key={step.step_id}
                        className="text-xs text-zinc-600 dark:text-zinc-300"
                      >
                        {step.subject} · {step.position_title} · {step.status}
                      </p>
                    ))}
                  </div>
                </section>
              )}

              {deactivationWizard.error && (
                <p
                  role="alert"
                  className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
                >
                  {deactivationWizard.error}
                </p>
              )}
            </div>

            <div className="flex justify-end gap-2 border-t border-zinc-200 px-6 py-4 dark:border-zinc-800">
              <button
                type="button"
                onClick={closeDeactivationWizard}
                disabled={busy}
                className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={submitDeactivationWizard}
                disabled={busy}
                className="rounded-lg bg-red-700 px-4 py-2 text-sm font-semibold text-white transition hover:bg-red-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busy ? "Menonaktifkan..." : "Nonaktifkan"}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
