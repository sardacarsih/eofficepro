"use client";

import { useMemo, useRef, useState } from "react";
import type { IncomingMailPoint } from "@/lib/dashboard-data";
import { ChevronDownIcon } from "@/components/layout/icons";

const RANGE_OPTIONS = [
  { value: 7, label: "7 Hari Terakhir" },
  { value: 14, label: "14 Hari Terakhir" },
  { value: 30, label: "30 Hari Terakhir" },
];

const VIEW_W = 640;
const VIEW_H = 250;
const PAD = { top: 16, right: 14, bottom: 30, left: 38 };
const CHART_W = VIEW_W - PAD.left - PAD.right;
const CHART_H = VIEW_H - PAD.top - PAD.bottom;
const GRID_ROWS = 4;

export default function TrendChart({ data }: { data: IncomingMailPoint[] }) {
  const [range, setRange] = useState(30);
  const [hover, setHover] = useState<number | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const points = useMemo(() => data.slice(-range), [data, range]);

  const maxValue = useMemo(() => {
    const peak = Math.max(1, ...points.map((p) => p.total));
    return Math.max(4, Math.ceil(peak / 4) * 4);
  }, [points]);

  const toX = (index: number) =>
    PAD.left +
    (points.length > 1 ? (index / (points.length - 1)) * CHART_W : CHART_W / 2);
  const toY = (value: number) => PAD.top + CHART_H - (value / maxValue) * CHART_H;

  const linePath = points
    .map(
      (p, i) => `${i === 0 ? "M" : "L"}${toX(i).toFixed(1)},${toY(p.total).toFixed(1)}`,
    )
    .join(" ");
  const areaPath = `${linePath} L${toX(points.length - 1).toFixed(1)},${PAD.top + CHART_H} L${toX(0).toFixed(1)},${PAD.top + CHART_H} Z`;

  const xLabelStep = Math.max(1, Math.ceil(points.length / 6));

  function handleMove(event: React.MouseEvent<SVGSVGElement>) {
    const svg = svgRef.current;
    if (!svg || points.length === 0) return;
    const rect = svg.getBoundingClientRect();
    const viewX = ((event.clientX - rect.left) / rect.width) * VIEW_W;
    const ratio = (viewX - PAD.left) / CHART_W;
    const index = Math.round(ratio * (points.length - 1));
    setHover(Math.min(points.length - 1, Math.max(0, index)));
  }

  const hovered = hover !== null ? points[hover] : null;

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-zinc-900 dark:text-zinc-50">
            Grafik Surat Masuk
          </h2>
          <p className="mt-0.5 text-xs text-zinc-400 dark:text-zinc-500">
            Trend volume surat masuk harian
          </p>
        </div>
        <div className="relative">
          <select
            value={range}
            onChange={(event) => {
              setRange(Number(event.target.value));
              setHover(null);
            }}
            aria-label="Rentang waktu grafik"
            className="appearance-none rounded-lg border border-zinc-200 bg-white py-1.5 pl-3 pr-8 text-xs font-medium text-zinc-700 shadow-sm transition hover:border-zinc-300 focus:border-navy-400 focus:outline-none dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300"
          >
            {RANGE_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <ChevronDownIcon className="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-400" />
        </div>
      </div>

      <div className="relative">
        <svg
          ref={svgRef}
          viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
          className="h-auto w-full"
          role="img"
          aria-label={`Grafik surat masuk ${range} hari terakhir, tertinggi ${Math.max(...points.map((p) => p.total))} surat per hari`}
          onMouseMove={handleMove}
          onMouseLeave={() => setHover(null)}
        >
          <defs>
            <linearGradient id="mail-trend-fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#0ea5e9" stopOpacity="0.28" />
              <stop offset="100%" stopColor="#0ea5e9" stopOpacity="0.02" />
            </linearGradient>
          </defs>

          {/* Grid horizontal + label sumbu Y */}
          {Array.from({ length: GRID_ROWS + 1 }, (_, i) => {
            const value = (maxValue / GRID_ROWS) * i;
            const y = toY(value);
            return (
              <g key={i}>
                <line
                  x1={PAD.left}
                  x2={VIEW_W - PAD.right}
                  y1={y}
                  y2={y}
                  className="stroke-zinc-100 dark:stroke-zinc-800"
                  strokeWidth={1}
                />
                <text
                  x={PAD.left - 8}
                  y={y + 3.5}
                  textAnchor="end"
                  className="fill-zinc-400 text-[10px] dark:fill-zinc-500"
                >
                  {Math.round(value)}
                </text>
              </g>
            );
          })}

          {/* Label sumbu X */}
          {points.map((p, i) =>
            i % xLabelStep === 0 ? (
              <text
                key={p.date}
                x={toX(i)}
                y={VIEW_H - 8}
                textAnchor="middle"
                className="fill-zinc-400 text-[10px] dark:fill-zinc-500"
              >
                {p.date}
              </text>
            ) : null,
          )}

          <path d={areaPath} fill="url(#mail-trend-fill)" />
          <path
            d={linePath}
            fill="none"
            stroke="#0284c7"
            strokeWidth={2.2}
            strokeLinecap="round"
            strokeLinejoin="round"
          />

          {hovered && hover !== null && (
            <g>
              <line
                x1={toX(hover)}
                x2={toX(hover)}
                y1={PAD.top}
                y2={PAD.top + CHART_H}
                stroke="#0284c7"
                strokeWidth={1}
                strokeDasharray="4 3"
                opacity={0.5}
              />
              <circle
                cx={toX(hover)}
                cy={toY(hovered.total)}
                r={4.5}
                fill="#0284c7"
                stroke="#fff"
                strokeWidth={2}
              />
            </g>
          )}
        </svg>

        {hovered && hover !== null && (
          <div
            className="pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-full rounded-lg bg-navy-900 px-2.5 py-1.5 text-center shadow-lg"
            style={{
              left: `${(toX(hover) / VIEW_W) * 100}%`,
              top: `${(toY(hovered.total) / VIEW_H) * 100 - 4}%`,
            }}
          >
            <p className="text-[10px] font-medium text-navy-300">{hovered.date}</p>
            <p className="text-xs font-bold text-white">{hovered.total} surat</p>
          </div>
        )}
      </div>
    </section>
  );
}
