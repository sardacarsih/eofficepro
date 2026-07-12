# Task Contracts

Direktori ini menyimpan task contract tim agent (lihat root `AGENTS.md` bagian
Team workflow). Satu file per task, dibuat oleh Lead sebelum pekerjaan dibagi.

Konvensi:

- Nama file: `<id>-<slug>.md`, contoh `E03-4-approval-paralel.md`. Gunakan ID
  ticket `docs/BACKLOG.md` bila ada; bila tidak, pakai tanggal `YYYYMMDD-slug`.
- File task adalah satu-satunya sumber kebenaran kontrak untuk task tersebut.
  Web dan Mobile bekerja dari bagian "Kontrak data/API" di file ini, bukan dari
  ingatan percakapan.
- Backend adalah owner bagian kontrak; perubahan kontrak diedit di file ini
  (beri catatan tanggal) setelah memberi tahu Lead.
- Setiap agent menambahkan bagian `## Handoff — <Role>` di bawah kontrak
  ketika pekerjaannya selesai. Jangan menimpa handoff agent lain.
- Task yang selesai diintegrasikan tetap disimpan sebagai arsip; jangan dihapus.

## Template

```markdown
# <ID> — <Judul task>

Status: draft | contract-stable | in-progress | verified | integrated

Tujuan:

Perilaku/acceptance criteria:
-

Scope owner:
- Backend:
- Web:
- Mobile:
- QA:

Di luar scope:

Kontrak data/API:
<!-- endpoint, request/response shape, status & error semantics.
     Owner: Backend. Perubahan diberi catatan tanggal. -->

Authorization dan company scope:

Dependency/order:

Verification:

Handoff yang diwajibkan:

## Handoff — Backend

## Handoff — Web

## Handoff — Mobile

## Handoff — QA
```
