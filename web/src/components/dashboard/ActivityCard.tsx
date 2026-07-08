import Link from "next/link";
import type {
  DashboardActivity,
  DashboardActivityIcon,
} from "@/lib/dashboard-data";
import {
  ArrowRightIcon,
  CheckCircleIcon,
  ClockIcon,
  CornerUpRightIcon,
  InboxIcon,
  SendIcon,
  type IconProps,
} from "@/components/layout/icons";

const ACTIVITY_ICON: Record<
  DashboardActivityIcon,
  { icon: (props: IconProps) => React.ReactElement; style: string }
> = {
  "surat-masuk": {
    icon: InboxIcon,
    style: "bg-sky-50 text-sky-600 dark:bg-sky-950 dark:text-sky-400",
  },
  disposisi: {
    icon: CornerUpRightIcon,
    style: "bg-cyan-50 text-cyan-600 dark:bg-cyan-950 dark:text-cyan-400",
  },
  approval: {
    icon: CheckCircleIcon,
    style: "bg-amber-50 text-amber-600 dark:bg-amber-950 dark:text-amber-400",
  },
  "surat-keluar": {
    icon: SendIcon,
    style: "bg-violet-50 text-violet-600 dark:bg-violet-950 dark:text-violet-400",
  },
};

export default function ActivityCard({
  activities,
}: {
  activities: DashboardActivity[];
}) {
  return (
    <section className="flex h-full flex-col rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <div className="mb-4 flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold text-zinc-900 dark:text-zinc-50">
          Ringkasan Aktivitas
        </h2>
        <Link
          href="/approvals"
          className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-semibold text-navy-600 transition hover:bg-navy-50 dark:text-sky-400 dark:hover:bg-navy-900"
        >
          Lihat semua
          <ArrowRightIcon className="h-3.5 w-3.5" />
        </Link>
      </div>
      <ul className="flex flex-1 flex-col divide-y divide-zinc-100 dark:divide-zinc-800">
        {activities.map((activity) => {
          const meta = ACTIVITY_ICON[activity.icon];
          const Icon = meta.icon;
          return (
            <li key={activity.id} className="flex items-center gap-3 py-3">
              <div
                className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${meta.style}`}
              >
                <Icon className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-zinc-800 dark:text-zinc-200">
                  {activity.title}
                </p>
                <p className="mt-0.5 flex items-center gap-1 text-xs text-zinc-400 dark:text-zinc-500">
                  <ClockIcon className="h-3 w-3" />
                  {activity.time}
                </p>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
