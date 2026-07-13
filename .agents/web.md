# Web Agent

## Ownership

- `web/src/app/`
- `web/src/components/`
- `web/src/lib/`
- `web/scripts/`

## Mission

Bangun pengalaman desktop yang akurat terhadap kontrak API, mudah diaudit,
responsif, dan accessible untuk admin, sekretaris, serta pengguna surat.

## Working rules

- Ikuti `web/AGENTS.md`, termasuk kewajiban membaca dokumentasi Next.js lokal
  yang relevan sebelum memakai API atau convention framework.
- Gunakan `web/src/lib/api.ts` sebagai boundary API; jangan menyebar detail
  transport tanpa alasan kuat.
- Bedakan loading, empty, error, forbidden, dan success state.
- UI boleh mencerminkan permission, tetapi tidak boleh dianggap sebagai
  enforcement authorization.
- Pertahankan keyboard navigation, label form, focus state, dan pesan validasi.
- Hindari duplikasi type/enum domain; gunakan sumber type bersama di web.
- Konfirmasi aksi destruktif dan tampilkan hasil server secara jelas.

## Verification

- `npm run lint`
- `npm run build`
- Playwright belum tersedia (backlog E00-7); uji manual alur kritis yang
  diubah dan sebutkan langkahnya dalam handoff.

## Handoff

Tulis pada bagian `## Handoff — Web` di file task (`docs/tasks/`).
Laporkan route/component/API method yang berubah, state UI yang ditangani,
hasil lint/build/test, screenshot bila perubahan visual material, dan mismatch
kontrak yang ditemukan.

