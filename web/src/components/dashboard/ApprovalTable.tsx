import Link from "next/link";
import type {
  PendingApprovalItem,
  PendingApprovalStatus,
} from "@/lib/dashboard-data";
import { ArrowRightIcon } from "@/components/layout/icons";

const STATUS_LABEL: Record<PendingApprovalStatus, string> = {
  pending: "Pending",
};

const STATUS_STYLE: Record<PendingApprovalStatus, string> = {
  pending: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
};

export default function ApprovalTable({
  items,
}: {
  items: PendingApprovalItem[];
}) {
  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold text-zinc-900 dark:text-zinc-50">
            Pending Approval
          </h2>
          <p className="mt-0.5 text-xs text-zinc-400 dark:text-zinc-500">
            Dokumen yang menunggu persetujuan Anda
          </p>
        </div>
        <Link
          href="/approvals"
          className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-semibold text-navy-600 transition hover:bg-navy-50 dark:text-sky-400 dark:hover:bg-navy-900"
        >
          Lihat semua
          <ArrowRightIcon className="h-3.5 w-3.5" />
        </Link>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[560px] text-left text-sm">
          <thead>
            <tr className="border-b border-zinc-200 text-xs uppercase tracking-wide text-zinc-400 dark:border-zinc-800 dark:text-zinc-500">
              <th scope="col" className="py-2.5 pr-3 font-semibold">
                No
              </th>
              <th scope="col" className="py-2.5 pr-3 font-semibold">
                Dokumen
              </th>
              <th scope="col" className="py-2.5 pr-3 font-semibold">
                Pengaju
              </th>
              <th scope="col" className="py-2.5 pr-3 font-semibold">
                Tanggal
              </th>
              <th scope="col" className="py-2.5 pr-3 font-semibold">
                Status
              </th>
              <th scope="col" className="py-2.5 font-semibold">
                Aksi
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-100 dark:divide-zinc-800">
            {items.map((item, index) => (
              <tr
                key={item.id}
                className="transition hover:bg-zinc-50 dark:hover:bg-zinc-800/50"
              >
                <td className="py-3 pr-3 tabular-nums text-zinc-400 dark:text-zinc-500">
                  {index + 1}
                </td>
                <td className="py-3 pr-3 font-medium text-zinc-800 dark:text-zinc-200">
                  {item.document}
                </td>
                <td className="py-3 pr-3 text-zinc-600 dark:text-zinc-400">
                  {item.requester}
                </td>
                <td className="py-3 pr-3 text-zinc-500 dark:text-zinc-400">
                  {item.date}
                </td>
                <td className="py-3 pr-3">
                  <span
                    className={`inline-flex rounded-full px-2.5 py-0.5 text-[11px] font-semibold ${STATUS_STYLE[item.status]}`}
                  >
                    {STATUS_LABEL[item.status]}
                  </span>
                </td>
                <td className="py-3">
                  <Link
                    href="/approvals"
                    className="inline-flex rounded-lg bg-navy-700 px-3 py-1.5 text-xs font-semibold text-white shadow-sm transition hover:bg-navy-800"
                  >
                    Review
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
