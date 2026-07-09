"use client";

import Image from "next/image";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { login } from "@/lib/api";

const LOGO_SRC = "/eoffice-logo.png";

const FEATURES = [
  {
    title: "Aman & Terpercaya",
    description: "Keamanan data tingkat enterprise",
    icon: ShieldIcon,
  },
  {
    title: "Kolaborasi Efektif",
    description: "Bekerja bersama lebih mudah",
    icon: UsersIcon,
  },
  {
    title: "Produktivitas Tinggi",
    description: "Kelola pekerjaan lebih efisien",
    icon: ClockIcon,
  },
];

function BrandLogo({ compact = false }: { compact?: boolean }) {
  return (
    <div className={compact ? "flex items-center gap-4" : "flex items-center gap-5 xl:gap-6"}>
      <span
        className={
          compact
            ? "relative block h-14 w-14 shrink-0 sm:h-16 sm:w-16"
            : "relative block h-20 w-20 shrink-0 lg:h-24 lg:w-24 2xl:h-28 2xl:w-28"
        }
      >
        <Image
          src={LOGO_SRC}
          alt="Logo eOffice Pro"
          fill
          priority
          sizes={
            compact
              ? "(max-width: 640px) 56px, 64px"
              : "(max-width: 1024px) 80px, (max-width: 1536px) 96px, 112px"
          }
          className="object-contain drop-shadow-[0_10px_24px_rgba(30,126,255,0.28)]"
        />
      </span>
      <div>
        <div
          className={
            compact
              ? "text-2xl font-extrabold tracking-tight text-[#06173a] sm:text-3xl"
              : "text-4xl font-extrabold leading-none tracking-tight text-[#06173a] lg:text-[46px] 2xl:text-[52px]"
          }
        >
          eOffice <span className="text-[#1764f5]">Pro</span>
        </div>
        <div
          className={
            compact
              ? "mt-1 text-lg font-semibold text-[#4d5a73] sm:text-xl"
              : "mt-2 text-2xl font-semibold leading-none text-[#4d5a73] lg:text-[28px] 2xl:text-[32px]"
          }
        >
          FKK Group
        </div>
      </div>
    </div>
  );
}

function BackgroundWaves() {
  return (
    <svg
      viewBox="0 0 820 360"
      preserveAspectRatio="none"
      aria-hidden="true"
      className="pointer-events-none absolute bottom-0 left-0 h-[38%] w-full"
      fill="none"
    >
      <path
        d="M-40 80C76 173 135 307 315 281c173-25 244-146 395-84 60 25 98 63 150 75"
        stroke="#2f7df6"
        strokeOpacity="0.16"
        strokeWidth="35"
      />
      <path
        d="M-80 124C64 249 178 341 338 284c135-48 210-152 354-109 67 20 118 65 188 72"
        stroke="#1d62e8"
        strokeOpacity="0.13"
        strokeWidth="2"
      />
      <path
        d="M-60 164C104 289 244 342 380 292c132-49 211-94 333-57 58 18 96 48 145 57"
        stroke="#63a5ff"
        strokeOpacity="0.16"
        strokeWidth="2"
      />
      <path
        d="M-15 226C104 304 245 353 372 324c111-25 167-88 279-79 87 7 147 54 217 59"
        stroke="#2f7df6"
        strokeOpacity="0.11"
        strokeWidth="2"
      />
    </svg>
  );
}

function UserIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
    >
      <path d="M20 21a8 8 0 0 0-16 0" />
      <circle cx="12" cy="7" r="4" />
    </svg>
  );
}

function LockIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
    >
      <rect x="4" y="10" width="16" height="10" rx="2" />
      <path d="M8 10V7a4 4 0 0 1 8 0v3" />
      <path d="M12 14v2" />
    </svg>
  );
}

function EyeIcon({ off }: { off: boolean }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className="h-6 w-6"
    >
      <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
      <circle cx="12" cy="12" r="3" />
      {off && <path d="M4 4l16 16" />}
    </svg>
  );
}

function ArrowLoginIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
    >
      <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" />
      <path d="m10 17 5-5-5-5" />
      <path d="M15 12H3" />
    </svg>
  );
}

function ShieldIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
    >
      <path d="M12 3 20 6v6c0 5-3.4 8.4-8 9-4.6-.6-8-4-8-9V6l8-3Z" />
      <path d="m9 12 2 2 4-5" />
    </svg>
  );
}

function UsersIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
    >
      <path d="M16 21v-2a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v2" />
      <circle cx="9.5" cy="7" r="4" />
      <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  );
}

function ClockIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
    >
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </svg>
  );
}

export default function LoginPage() {
  const router = useRouter();
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await login(identifier, password);
      router.push("/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login gagal");
    } finally {
      setLoading(false);
    }
  }

  const inputClass =
    "h-14 w-full rounded-[8px] border border-[#d8dfeb] bg-white pl-12 pr-4 text-base text-[#07183b] outline-none sm:h-16 sm:pl-14 sm:text-lg xl:h-[70px] xl:pl-16 xl:text-[20px] " +
    "shadow-[0_1px_0_rgba(15,33,64,0.03)] placeholder:text-[#8a97b5] " +
    "transition-[border-color,box-shadow] duration-200 focus:border-[#1e6eff] focus:ring-4 focus:ring-[#1e6eff]/15";

  return (
    <main className="relative flex min-h-dvh flex-1 items-center justify-center overflow-hidden bg-[#eef4fc] p-3 text-[#07183b] sm:p-5 lg:p-8 2xl:p-10">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_18%_18%,rgba(255,255,255,0.95),transparent_32%),radial-gradient(circle_at_78%_76%,rgba(191,216,255,0.45),transparent_30%)]" />

      <section className="relative flex min-h-[calc(100dvh-24px)] w-full max-w-[1800px] overflow-hidden rounded-[22px] bg-white/88 shadow-[0_26px_70px_rgba(23,53,99,0.18)] ring-1 ring-white/80 backdrop-blur sm:min-h-[calc(100dvh-40px)] sm:rounded-[28px] lg:min-h-[min(860px,calc(100dvh-64px))] 2xl:min-h-[min(900px,calc(100dvh-80px))]">
        <BackgroundWaves />

        <div className="relative hidden w-[47%] flex-col px-10 py-12 md:flex lg:px-14 xl:px-20 xl:py-16 2xl:px-28 2xl:py-20">
          <BrandLogo />

          <div className="mt-16 max-w-[470px] lg:mt-20 2xl:mt-28">
            <h1 className="text-4xl font-extrabold leading-tight tracking-tight text-[#07183b] lg:text-[44px] 2xl:text-[52px]">
              Selamat Datang
            </h1>
            <p className="mt-2 text-3xl font-medium leading-tight text-[#4d5a73] lg:text-[32px] 2xl:text-[38px]">
              di eOffice Pro
            </p>
            <div className="mt-7 h-1 w-16 rounded-full bg-[#1764f5]" />
            <p className="mt-7 text-lg leading-relaxed text-[#30405c] lg:text-[20px] 2xl:text-[22px]">
              Platform manajemen perkantoran digital untuk meningkatkan
              produktivitas dan kolaborasi kerja Anda.
            </p>
          </div>

          <ul className="mt-10 flex max-w-[520px] flex-col gap-5 lg:gap-6 2xl:mt-12 2xl:gap-7">
            {FEATURES.map((feature) => {
              const Icon = feature.icon;

              return (
                <li key={feature.title} className="flex items-center gap-5 2xl:gap-7">
                  <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[12px] bg-[#edf4ff] text-[#1764f5] shadow-[0_14px_30px_rgba(33,100,210,0.09)] 2xl:h-16 2xl:w-16">
                    <Icon className="h-8 w-8 2xl:h-9 2xl:w-9" />
                  </span>
                  <span>
                    <span className="block text-lg font-extrabold leading-tight text-[#07183b] lg:text-[20px] 2xl:text-[21px]">
                      {feature.title}
                    </span>
                    <span className="mt-2 block text-base leading-tight text-[#41506c] lg:text-[17px] 2xl:text-[18px]">
                      {feature.description}
                    </span>
                  </span>
                </li>
              );
            })}
          </ul>
        </div>

        <div className="relative hidden w-px self-stretch bg-[#dbe4f0] md:block" />

        <div className="relative flex min-w-0 flex-1 flex-col px-4 py-7 sm:px-7 sm:py-8 md:justify-center lg:px-12 xl:px-16 2xl:px-24">
          <div className="mb-8 md:hidden">
            <BrandLogo compact />
          </div>

          <div className="mx-auto w-full max-w-[760px] rounded-[22px] bg-white px-5 py-7 shadow-[0_22px_58px_rgba(30,56,97,0.14)] ring-1 ring-[#eef2f7] sm:rounded-[28px] sm:px-8 sm:py-9 md:px-10 lg:px-12 xl:px-16 xl:py-14 2xl:px-[72px]">
            <div>
              <h2 className="text-2xl font-extrabold leading-tight tracking-tight text-[#07183b] sm:text-[30px] xl:text-[34px]">
                Masuk ke Akun Anda
              </h2>
              <p className="mt-3 text-base leading-relaxed text-[#41506c] sm:mt-5 sm:text-lg xl:text-[21px]">
                Silakan masuk dengan email dan kata sandi Anda
              </p>
            </div>

            <form onSubmit={handleSubmit} className="mt-7 flex flex-col gap-5 sm:mt-9 sm:gap-6 xl:mt-10 xl:gap-7">
              <label className="flex flex-col gap-3 text-base font-extrabold text-[#07183b] sm:gap-4 sm:text-[17px]">
                Email atau Username
                <span className="relative block">
                  <UserIcon className="pointer-events-none absolute left-4 top-1/2 h-6 w-6 -translate-y-1/2 text-[#52617d] sm:left-5 sm:h-7 sm:w-7 xl:left-6" />
                  <input
                    type="text"
                    value={identifier}
                    onChange={(e) => setIdentifier(e.target.value)}
                    required
                    autoFocus
                    autoComplete="username"
                    placeholder="Masukkan email atau username Anda"
                    className={inputClass}
                  />
                </span>
              </label>

              <label className="flex flex-col gap-3 text-base font-extrabold text-[#07183b] sm:gap-4 sm:text-[17px]">
                Kata Sandi
                <span className="relative block">
                  <LockIcon className="pointer-events-none absolute left-4 top-1/2 h-6 w-6 -translate-y-1/2 text-[#52617d] sm:left-5 sm:h-7 sm:w-7 xl:left-6" />
                  <input
                    type={showPassword ? "text" : "password"}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                    autoComplete="current-password"
                    placeholder="Masukkan kata sandi Anda"
                    className={`${inputClass} pr-14 sm:pr-16`}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword((v) => !v)}
                    aria-label={
                      showPassword ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"
                    }
                    className="absolute inset-y-0 right-0 flex w-14 items-center justify-center rounded-r-[8px] text-[#52617d] transition hover:text-[#1764f5] focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-[#1764f5] sm:w-16"
                  >
                    <EyeIcon off={showPassword} />
                  </button>
                </span>
              </label>

              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <label className="flex w-fit items-center gap-3 text-base text-[#52617d] sm:text-[17px]">
                  <input
                    type="checkbox"
                    className="h-5 w-5 rounded border-[#b7c3d6] text-[#1764f5] focus:ring-[#1764f5]/25"
                  />
                  Ingat saya
                </label>
                <a
                  href="/forgot-password"
                  className="w-fit rounded text-base font-medium text-[#045fff] transition hover:text-[#004bd1] hover:underline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-[#1764f5] sm:text-[17px]"
                >
                  Lupa kata sandi?
                </a>
              </div>

              {error && (
                <p
                  role="alert"
                  className="flex items-start gap-3 rounded-[8px] border border-red-200 bg-red-50 px-4 py-3 text-[15px] leading-relaxed text-red-700"
                >
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    aria-hidden="true"
                    className="mt-0.5 h-5 w-5 shrink-0"
                  >
                    <circle cx="12" cy="12" r="9" />
                    <path d="M12 8v4M12 16h.01" />
                  </svg>
                  {error}
                </p>
              )}

              <button
                type="submit"
                disabled={loading}
                className="mt-1 flex h-14 items-center justify-center gap-3 rounded-[8px] bg-[#1764f5] text-lg font-extrabold text-white shadow-[0_14px_30px_rgba(23,100,245,0.28)] transition hover:bg-[#0757e8] focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-[#1764f5] active:bg-[#004bd1] disabled:cursor-not-allowed disabled:opacity-65 sm:h-16 sm:gap-4 sm:text-xl xl:h-[78px] xl:gap-5 xl:text-[21px]"
              >
                {loading ? (
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    aria-hidden="true"
                    className="h-7 w-7 animate-spin"
                  >
                    <circle
                      cx="12"
                      cy="12"
                      r="9"
                      stroke="currentColor"
                      strokeOpacity="0.3"
                      strokeWidth="3"
                    />
                    <path
                      d="M21 12a9 9 0 0 0-9-9"
                      stroke="currentColor"
                      strokeWidth="3"
                      strokeLinecap="round"
                    />
                  </svg>
                ) : (
                  <ArrowLoginIcon className="h-6 w-6 sm:h-7 sm:w-7 xl:h-8 xl:w-8" />
                )}
                {loading ? "Memproses..." : "Masuk"}
              </button>

              <p className="text-center text-base leading-relaxed text-[#52617d] sm:text-[17px]">
                Belum memiliki akun?{" "}
                <a
                  href="mailto:admin@fkkgroup.co.id"
                  className="font-medium text-[#045fff] hover:text-[#004bd1] hover:underline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-[#1764f5]"
                >
                  Hubungi administrator Anda
                </a>
              </p>
            </form>
          </div>
        </div>
      </section>
    </main>
  );
}
