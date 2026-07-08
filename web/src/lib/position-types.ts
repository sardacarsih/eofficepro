export const POSITION_TYPE_LABEL: Record<string, string> = {
  president_director: "President Director",
  vp_director: "Vice President Director",
  director: "Director",
  gm: "GM",
  dept_head: "Department Head",
  sub_dept_head: "Sub Department Head",
  division_head: "Division Head",
  assistant: "Assistant",
  secretary: "Secretary",
  staff: "Staff",
  auditor: "Auditor",
};

export const USER_POSITION_TYPES_BY_UNIT_LEVEL: Record<string, readonly string[]> = {
  office: ["president_director", "vp_director", "assistant", "staff"],
  directorate: ["director", "secretary", "auditor", "assistant", "staff"],
  biro: ["gm", "secretary", "assistant", "staff"],
  department: ["dept_head", "sub_dept_head", "division_head"],
  division: ["division_head", "staff"],
};

export const MASTER_POSITION_TYPES_BY_UNIT_LEVEL: Record<
  string,
  readonly string[]
> = {
  office: ["president_director", "vp_director", "assistant", "staff"],
  directorate: ["director", "secretary", "auditor", "assistant", "staff"],
  biro: ["gm", "secretary", "assistant", "staff"],
  department: ["dept_head", "sub_dept_head", "staff"],
  division: ["division_head", "staff"],
};

export const DEFAULT_APPROVER_POSITION_TYPES = new Set([
  "president_director",
  "vp_director",
  "director",
  "gm",
  "dept_head",
  "sub_dept_head",
  "division_head",
  "auditor",
]);
