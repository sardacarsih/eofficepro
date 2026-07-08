import type {
  DashboardStat,
  DashboardStatAccent,
  DashboardStatIcon,
} from "@/lib/dashboard-data";
import {
  ArchiveIcon,
  CheckCircleIcon,
  InboxIcon,
  SendIcon,
  type IconProps,
} from "@/components/layout/icons";

const STAT_ICON: Record<DashboardStatIcon, (props: IconProps) => React.ReactElement> = {
  inbox: InboxIcon,
  send: SendIcon,
  approval: CheckCircleIcon,
  archive: ArchiveIcon,
};

const ACCENT_STYLE: Record<DashboardStatAccent, string> = {
  sky: "bg-sky-50 text-sky-600 dark:bg-sky-950 dark:text-sky-400",
  cyan: "bg-cyan-50 text-cyan-600 dark:bg-cyan-950 dark:text-cyan-400",
  amber: "bg-amber-50 text-amber-600 dark:bg-amber-950 dark:text-amber-400",
  violet: "bg-violet-50 text-violet-600 dark:bg-violet-950 dark:text-violet-400",
};

export default function StatCard({ stat }: { stat: DashboardStat }) {
  const Icon = STAT_ICON[stat.icon];

  return (
    <div className="group rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm transition duration-200 hover:-translate-y-0.5 hover:shadow-md dark:border-zinc-800 dark:bg-zinc-900">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
            {stat.label}
          </p>
          <p className="mt-2 text-3xl font-bold tabular-nums text-zinc-900 dark:text-zinc-50">
            {stat.value.toLocaleString("id-ID")}
          </p>
          <p className="mt-1 text-xs text-zinc-400 dark:text-zinc-500">
            {stat.note}
          </p>
        </div>
        <div
          className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-xl transition group-hover:scale-105 ${ACCENT_STYLE[stat.accent]}`}
        >
          <Icon className="h-5 w-5" />
        </div>
      </div>
    </div>
  );
}
