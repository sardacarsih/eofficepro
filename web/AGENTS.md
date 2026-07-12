<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

# eOffice Pro Web Rules

Aturan root `AGENTS.md` tetap berlaku.

- Perlakukan `src/lib/api.ts` sebagai boundary utama komunikasi backend.
- Jaga request, response, status, dan error handling sesuai kontrak API aktual.
- Setiap halaman data harus menangani loading, empty, error, forbidden, dan
  success state yang relevan.
- Pertahankan aksesibilitas form, dialog, tabel, pagination, dan navigation.
- Perubahan visual material harus diperiksa pada viewport desktop dan viewport
  sempit yang relevan.
- Jalankan `npm run lint` dan `npm run build` sebelum handoff.
