import { chromium, expect } from "@playwright/test";

const baseURL = process.env.EOFFICE_WEB_URL ?? "http://localhost:3000";
const adminEmail = process.env.EOFFICE_ADMIN_EMAIL ?? "admin@ksk.local";
const adminPassword = process.env.EOFFICE_ADMIN_PASSWORD ?? "GantiSegera#2026";
const secretaryEmail =
  process.env.EOFFICE_SECRETARY_EMAIL ?? "secretary.gm.fa@ksk.local";
const secretaryPassword =
  process.env.EOFFICE_SECRETARY_PASSWORD ?? adminPassword;
const channel = process.env.PLAYWRIGHT_CHANNEL;

const browser = await chromium.launch({
  headless: true,
  ...(channel ? { channel } : {}),
});

try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  await page.goto(`${baseURL}/login`);
  await page.getByPlaceholder("Masukkan email atau username Anda").fill(adminEmail);
  await page.getByPlaceholder("Masukkan kata sandi Anda").fill(adminPassword);
  await page.getByRole("button", { name: "Masuk" }).click();
  await page.waitForURL(/\/organization$/, { timeout: 15000 });

  await page.goto(`${baseURL}/organization`);
  await page
    .getByPlaceholder("Cari nama, kode, atau region...")
    .fill("Biro Finance & Accounting");
  await expect(page.getByText("Directorate Finance & Accounting")).toBeVisible();
  await expect(page.getByText("Biro Finance & Accounting")).toBeVisible();
  await page.getByText("Biro Finance & Accounting").click();
  await expect(page.getByText("Jabatan Aktif")).toBeVisible();
  await expect(page.getByText("GM Finance & Accounting").first()).toBeVisible();
  await expect(
    page.getByText("Secretary GM Finance & Accounting").first(),
  ).toBeVisible();
  await expect(page.getByText("Secretary").first()).toBeVisible();

  await page.goto(`${baseURL}/users`);
  await page.getByRole("button", { name: "Tambah Pengguna" }).click();
  await page.getByLabel("Tipe Jabatan").selectOption("secretary");
  await page
    .getByPlaceholder("Nama, unit, tipe, atau pemegang...")
    .fill("Secretary GM Finance & Accounting");
  await expect(
    page.locator("select").filter({
      has: page.locator("option", {
        hasText: "Secretary GM Finance & Accounting",
      }),
    }),
  ).toBeVisible();

  await page.evaluate(() => {
    localStorage.clear();
  });
  await page.goto(`${baseURL}/login`);
  await page.getByPlaceholder("Masukkan email atau username Anda").fill(secretaryEmail);
  await page.getByPlaceholder("Masukkan kata sandi Anda").fill(secretaryPassword);
  await page.getByRole("button", { name: "Masuk" }).click();
  await page.waitForURL(/\/organization$/, { timeout: 15000 });
  await page.goto(`${baseURL}/compose`);
  await expect(page.getByText("Atas Nama")).toBeVisible();
  await expect(
    page.getByText("GM Finance & Accounting", { exact: true }),
  ).toBeVisible();
} finally {
  await browser.close();
}
