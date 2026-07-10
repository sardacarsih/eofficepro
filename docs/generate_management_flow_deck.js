const pptxgen = require('pptxgenjs');
const path = require('path');

const pptx = new pptxgen();
pptx.layout = 'LAYOUT_WIDE';
pptx.author = 'eOffice Pro — PT Kalimantan Sawit Kusuma Group';
pptx.subject = 'Flow Tulis Surat dan Approval Digital';
pptx.title = 'Tulis Surat & Approval Digital eOffice Pro';
pptx.company = 'PT Kalimantan Sawit Kusuma Group';
pptx.lang = 'id-ID';
pptx.theme = {
  headFontFace: 'Aptos Display',
  bodyFontFace: 'Aptos',
  lang: 'id-ID',
};
pptx.defineLayout({ name: 'WIDE', width: 13.333, height: 7.5 });
pptx.layout = 'WIDE';

const C = {
  navy: '063B4C', teal: '028090', sea: '00A896', mint: 'B6E3D4', ice: 'E8F5F4',
  ink: '12333A', slate: '4B6470', line: 'CDE4E1', white: 'FFFFFF', sand: 'F8FBFA',
  amber: 'E7A52A', coral: 'D96554', green: '2B8A65', gray: 'EEF4F3',
};
const W = 13.333, H = 7.5;
const root = path.resolve(__dirname, '..');
const img = (...parts) => path.join(root, ...parts);

function shadow() { return { type: 'outer', color: '183B43', opacity: 0.12, blur: 2, angle: 45, distance: 1 }; }
function addBg(slide, color = C.sand) { slide.background = { color }; }
function addHeader(slide, title, kicker = 'eOFFICE PRO') {
  slide.addText(kicker, { x: 0.55, y: 0.34, w: 2.1, h: 0.2, fontFace: 'Aptos', fontSize: 8.5, bold: true, color: C.teal, charSpacing: 1.5, margin: 0 });
  slide.addText(title, { x: 0.55, y: 0.62, w: 11.8, h: 0.5, fontFace: 'Aptos Display', fontSize: 27, bold: true, color: C.navy, margin: 0, breakLine: false, fit: 'shrink' });
}
function addFooter(slide, n) {
  slide.addShape(pptx.ShapeType.line, { x: 0.55, y: 7.02, w: 12.2, h: 0, line: { color: C.line, width: 0.6 } });
  slide.addText('Sumber: PRD eOffice Pro, Ringkasan Eksekutif, dan implementasi flow terkini', { x: 0.55, y: 7.1, w: 10.6, h: 0.16, fontSize: 7.5, color: C.slate, margin: 0 });
  slide.addText(String(n).padStart(2, '0'), { x: 12.35, y: 7.08, w: 0.4, h: 0.2, fontSize: 8, bold: true, color: C.teal, align: 'right', margin: 0 });
}
function card(slide, x, y, w, h, title, body, accent = C.teal, options = {}) {
  slide.addShape(pptx.ShapeType.roundRect, { x, y, w, h, rectRadius: 0.08, fill: { color: C.white }, line: { color: C.line, width: 0.7 }, shadow: shadow() });
  slide.addShape(pptx.ShapeType.rect, { x, y, w: 0.08, h, fill: { color: accent }, line: { color: accent } });
  slide.addText(title, { x: x + 0.25, y: y + 0.2, w: w - 0.45, h: 0.27, fontSize: options.titleSize || 14, bold: true, color: C.ink, margin: 0, fit: 'shrink' });
  slide.addText(body, { x: x + 0.25, y: y + 0.55, w: w - 0.45, h: h - 0.7, fontSize: options.bodySize || 10.5, color: C.slate, breakLine: false, margin: 0.02, valign: 'top', fit: 'shrink', bullet: options.bullet });
}
function pill(slide, x, y, w, text, fill = C.ice, color = C.teal) {
  slide.addShape(pptx.ShapeType.roundRect, { x, y, w, h: 0.32, rectRadius: 0.06, fill: { color: fill }, line: { color: fill } });
  slide.addText(text, { x, y: y + 0.07, w, h: 0.14, fontSize: 8.2, bold: true, color, align: 'center', margin: 0, fit: 'shrink' });
}
function flowNode(slide, x, y, w, label, sub, n, fill = C.white, accent = C.teal) {
  slide.addShape(pptx.ShapeType.roundRect, { x, y, w, h: 1.04, rectRadius: 0.08, fill: { color: fill }, line: { color: accent, width: 1.1 }, shadow: shadow() });
  slide.addShape(pptx.ShapeType.ellipse, { x: x + 0.14, y: y + 0.15, w: 0.33, h: 0.33, fill: { color: accent }, line: { color: accent } });
  slide.addText(String(n), { x: x + 0.14, y: y + 0.23, w: 0.33, h: 0.1, fontSize: 7.8, bold: true, color: C.white, align: 'center', margin: 0 });
  slide.addText(label, { x: x + 0.55, y: y + 0.15, w: w - 0.66, h: 0.22, fontSize: 11, bold: true, color: C.ink, margin: 0, fit: 'shrink' });
  slide.addText(sub, { x: x + 0.15, y: y + 0.58, w: w - 0.3, h: 0.28, fontSize: 8.3, color: C.slate, margin: 0, align: 'center', fit: 'shrink' });
}
function arrow(slide, x, y, w, color = C.teal) { slide.addShape(pptx.ShapeType.chevron, { x, y, w, h: 0.22, fill: { color }, line: { color } }); }
function miniLine(slide, x, y, w, color = C.line) { slide.addShape(pptx.ShapeType.line, { x, y, w, h: 0, line: { color, width: 0.7 } }); }

// 1 — Cover
{
  const s = pptx.addSlide(); addBg(s, C.navy);
  s.addShape(pptx.ShapeType.arc, { x: 8.55, y: -1.35, w: 5.7, h: 5.7, adjustPoint: 0.25, line: { color: C.sea, transparency: 20, width: 8 }, adjustPoint: 0.2 });
  s.addShape(pptx.ShapeType.arc, { x: 9.65, y: 2.35, w: 3.1, h: 3.1, line: { color: C.mint, transparency: 30, width: 3 } });
  pill(s, 0.72, 0.73, 1.7, 'MANAGEMENT BRIEF', '0B5B69', C.mint);
  s.addText('Tulis Surat &\nApproval Digital', { x: 0.72, y: 1.42, w: 7.45, h: 1.5, fontFace: 'Aptos Display', fontSize: 34, bold: true, color: C.white, margin: 0, breakLine: false, fit: 'shrink' });
  s.addText('Flow bisnis yang terukur, aman, dan dapat ditelusuri untuk KSK Group', { x: 0.75, y: 3.15, w: 5.9, h: 0.5, fontSize: 16, color: 'C8E7E2', margin: 0, fit: 'shrink' });
  const sx = 8.1, sy = 1.2;
  ['TEMPLATE', 'APPROVAL', 'QR / ARSIP'].forEach((t, i) => {
    s.addShape(pptx.ShapeType.roundRect, { x: sx + i * 1.28, y: sy + i * 0.75, w: 1.72, h: 0.82, rectRadius: 0.07, fill: { color: i === 1 ? C.sea : '0B5260' }, line: { color: '2BB3B3', transparency: 50 }, shadow: shadow() });
    s.addText(t, { x: sx + i * 1.28 + 0.12, y: sy + i * 0.75 + 0.28, w: 1.48, h: 0.16, fontSize: 8.5, bold: true, color: C.white, align: 'center', margin: 0, fit: 'shrink' });
  });
  s.addText('PT Kalimantan Sawit Kusuma Group  |  Juli 2026', { x: 0.75, y: 6.58, w: 6.2, h: 0.2, fontSize: 9.5, color: '9ECAC5', margin: 0 });
}

// 2 — Why
{
  const s = pptx.addSlide(); addBg(s); addHeader(s, 'Mengapa digitalisasi surat perlu dipercepat?', 'KONTEKS BISNIS');
  card(s, 0.58, 1.52, 5.45, 3.9, 'Hari ini: proses fisik menciptakan jeda dan blind spot', 'Approval menunggu pejabat berada di lokasi.\nPengirim tidak melihat posisi surat.\nArsip sulit dicari dan bukti audit tersebar.\nCetak, kurir, dan duplikasi dokumen berulang.', C.coral, { bodySize: 13.2 });
  card(s, 7.3, 1.52, 5.45, 3.9, 'Dengan eOffice Pro: keputusan bergerak bersama dokumen', 'Surat dibuat dari template resmi.\nApproval mengikuti jabatan dan matrix.\nStatus, SLA, dan audit tersedia real-time.\nDokumen final terbit sebagai PDF ber-QR dan terdistribusi digital.', C.sea, { bodySize: 13.2 });
  s.addShape(pptx.ShapeType.chevron, { x: 6.35, y: 3.08, w: 0.68, h: 0.44, fill: { color: C.amber }, line: { color: C.amber } });
  s.addShape(pptx.ShapeType.roundRect, { x: 5.88, y: 3.76, w: 1.62, h: 0.36, rectRadius: 0.04, fill: { color: C.navy }, line: { color: C.navy } });
  s.addText('HAMBATAN → KENDALI', { x: 5.95, y: 3.88, w: 1.48, h: 0.1, fontSize: 7.1, bold: true, color: C.white, align: 'center', margin: 0, fit: 'shrink' });
  pill(s, 0.72, 5.82, 2.2, 'LEBIH CEPAT', C.ice, C.teal);
  pill(s, 3.05, 5.82, 2.2, 'TERLIHAT', C.ice, C.teal);
  pill(s, 7.52, 5.82, 2.2, 'TERAUDIT', C.ice, C.teal);
  pill(s, 9.85, 5.82, 2.2, 'PAPERLESS', C.ice, C.teal);
  addFooter(s, 2);
}

// 3 — Compose flow
{
  const s = pptx.addSlide(); addBg(s); addHeader(s, 'Flow end-to-end: Tulis Surat', 'PROSES PEMBUATAN');
  const nodes = [
    ['Pilih template', 'kop & format resmi'], ['Isi & penerima', 'To / CC berbasis jabatan'], ['Autosave', 'draft & versi terlindungi'], ['Lampiran aman', 'validasi + scan ClamAV'], ['Preview PDF', 'cek sebelum ajukan'], ['Ajukan', 'rute approval dibentuk'],
  ];
  nodes.forEach((v, i) => { const x = 0.48 + i * 2.14; flowNode(s, x, 1.72, 1.72, v[0], v[1], i + 1, C.white, i === 3 ? C.amber : C.teal); if (i < nodes.length - 1) arrow(s, x + 1.77, 2.12, 0.28); });
  s.addShape(pptx.ShapeType.roundRect, { x: 0.7, y: 3.3, w: 5.65, h: 2.7, rectRadius: 0.08, fill: { color: C.white }, line: { color: C.line }, shadow: shadow() });
  s.addText('Gerbang kualitas sebelum “Ajukan”', { x: 1.0, y: 3.6, w: 4.9, h: 0.3, fontSize: 16, bold: true, color: C.ink, align: 'center', margin: 0 });
  [['01', 'Template aktif', C.teal], ['02', 'Penerima valid', C.sea], ['03', 'Lampiran clean', C.amber], ['04', 'Preview siap', C.green]].forEach((v, i) => {
    const x = 1.02 + (i % 2) * 2.62, y = 4.2 + Math.floor(i / 2) * 0.86;
    s.addShape(pptx.ShapeType.roundRect, { x, y, w: 2.25, h: 0.58, rectRadius: 0.04, fill: { color: C.ice }, line: { color: v[2], width: 0.8 } });
    s.addText(v[0], { x: x + 0.12, y: y + 0.2, w: 0.28, h: 0.1, fontSize: 8, bold: true, color: v[2], margin: 0 });
    s.addText(v[1], { x: x + 0.48, y: y + 0.16, w: 1.62, h: 0.16, fontSize: 9.5, bold: true, color: C.ink, margin: 0, fit: 'shrink' });
  });
  card(s, 6.8, 3.35, 5.75, 1.15, 'Kontrol sebelum pengajuan', 'Klasifikasi, prioritas, penerima dan konten wajib tervalidasi; lampiran hanya dapat diajukan setelah status scan clean.', C.teal, { bodySize: 10.5 });
  card(s, 6.8, 4.85, 5.75, 1.15, 'Jejak draft tidak hilang', 'Autosave dan versi optimistis mencegah perubahan dari dua perangkat saling menimpa.', C.sea, { bodySize: 10.5 });
  addFooter(s, 3);
}

// 4 — Approval
{
  const s = pptx.addSlide(); addBg(s); addHeader(s, 'Approval mengikuti jabatan, bukan sekadar orang', 'WORKFLOW PERSETUJUAN');
  const roles = [
    ['Pembuat / Sekretaris', 'ajukan surat'], ['Dept. Head', 'review awal'], ['GM / Director', 'approval berjenjang'], ['VP / Presdir', 'final sesuai matrix'],
  ];
  roles.forEach((r, i) => { const x = 0.68 + i * 3.03; flowNode(s, x, 1.48, 2.35, r[0], r[1], i + 1, i === 3 ? C.ice : C.white, i === 3 ? C.sea : C.teal); if (i < roles.length - 1) arrow(s, x + 2.43, 1.88, 0.42); });
  s.addShape(pptx.ShapeType.roundRect, { x: 0.68, y: 3.25, w: 12.0, h: 2.52, rectRadius: 0.08, fill: { color: C.white }, line: { color: C.line }, shadow: shadow() });
  s.addText('Cabang keputusan yang terkendali', { x: 0.98, y: 3.54, w: 3.7, h: 0.25, fontSize: 15, bold: true, color: C.ink, margin: 0 });
  card(s, 0.98, 4.05, 3.55, 1.25, 'Setuju', 'Step berikutnya aktif; approval final mengunci nomor surat.', C.green, { bodySize: 9.6 });
  card(s, 4.85, 4.05, 3.55, 1.25, 'Minta revisi', 'Surat kembali ke pembuat dengan catatan yang dapat ditindaklanjuti.', C.amber, { bodySize: 9.6 });
  card(s, 8.72, 4.05, 3.55, 1.25, 'Tolak', 'Alur ditutup; alasan tercatat dalam timeline dan audit trail.', C.coral, { bodySize: 9.6 });
  s.addShape(pptx.ShapeType.roundRect, { x: 0.98, y: 5.79, w: 11.28, h: 0.44, rectRadius: 0.04, fill: { color: C.ice }, line: { color: C.line } });
  s.addText('Sekretaris dapat membuat surat atas nama atasan sesuai hubungan jabatan; setiap aksi tetap dapat ditelusuri.', { x: 1.2, y: 5.94, w: 10.85, h: 0.12, fontSize: 9.6, bold: true, color: C.navy, margin: 0, fit: 'shrink' });
  addFooter(s, 4);
}

// 5 — Publication
{
  const s = pptx.addSlide(); addBg(s); addHeader(s, 'Dari approval final ke surat resmi yang dapat diverifikasi', 'PENERBITAN & DISTRIBUSI');
  const stages = [
    ['Approval final', 'nomor resmi dikunci'], ['Job publikasi', 'retry bila storage bermasalah'], ['PDF + QR', 'dokumen final tervalidasi'], ['Inbox To / CC', 'distribusi otomatis'], ['Disposisi', 'instruksi & tenggat terlacak'],
  ];
  stages.forEach((v, i) => { const x = 0.45 + i * 2.55; flowNode(s, x, 1.62, 2.05, v[0], v[1], i + 1, i === 1 ? C.ice : C.white, i === 1 ? C.sea : C.teal); if (i < stages.length - 1) arrow(s, x + 2.11, 2.04, 0.33, C.sea); });
  s.addShape(pptx.ShapeType.roundRect, { x: 0.65, y: 3.24, w: 5.15, h: 2.7, rectRadius: 0.08, fill: { color: C.white }, line: { color: C.line }, shadow: shadow() });
  s.addText('Distribusi berbasis peran', { x: 0.98, y: 3.56, w: 4.5, h: 0.28, fontSize: 16, bold: true, color: C.ink, align: 'center', margin: 0 });
  s.addShape(pptx.ShapeType.roundRect, { x: 2.23, y: 4.18, w: 1.95, h: 0.55, rectRadius: 0.04, fill: { color: C.navy }, line: { color: C.navy } });
  s.addText('PDF + QR resmi', { x: 2.4, y: 4.38, w: 1.6, h: 0.1, fontSize: 8.7, bold: true, color: C.white, align: 'center', margin: 0 });
  arrow(s, 1.25, 5.02, 0.84, C.teal); arrow(s, 3.48, 5.02, 0.84, C.sea);
  s.addShape(pptx.ShapeType.roundRect, { x: 0.94, y: 5.36, w: 1.55, h: 0.38, rectRadius: 0.04, fill: { color: C.ice }, line: { color: C.teal } });
  s.addShape(pptx.ShapeType.roundRect, { x: 3.82, y: 5.36, w: 1.55, h: 0.38, rectRadius: 0.04, fill: { color: C.ice }, line: { color: C.sea } });
  s.addText('Inbox To / CC', { x: 1.03, y: 5.5, w: 1.36, h: 0.1, fontSize: 8.2, bold: true, color: C.ink, align: 'center', margin: 0 });
  s.addText('Disposisi & SLA', { x: 3.92, y: 5.5, w: 1.35, h: 0.1, fontSize: 8.2, bold: true, color: C.ink, align: 'center', margin: 0 });
  card(s, 6.25, 3.3, 6.05, 1.1, 'Keandalan proses', 'Approval tidak perlu diulang bila PDF atau storage sementara bermasalah; job publikasi akan mencoba kembali secara aman.', C.sea, { bodySize: 10.5 });
  card(s, 6.25, 4.75, 6.05, 1.1, 'Bukti keaslian', 'PDF final memiliki QR verifikasi; nomor surat, waktu terbit, distribusi dan tindakan lanjutan tersimpan sebagai jejak digital.', C.teal, { bodySize: 10.5 });
  addFooter(s, 5);
}

// 6 — Security
{
  const s = pptx.addSlide(); addBg(s); addHeader(s, 'Kontrol keamanan dan keandalan dibangun ke dalam flow', 'RISK CONTROL');
  s.addText('Matriks akses dokumen', { x: 0.7, y: 1.48, w: 3, h: 0.22, fontSize: 14, bold: true, color: C.ink, margin: 0 });
  [['Klasifikasi', 'Pembuat / Approver', 'To', 'CC'], ['Biasa', 'PDF + lampiran', 'PDF + lampiran', 'PDF + lampiran'], ['Terbatas', 'PDF + lampiran', 'PDF + lampiran', 'sesuai otorisasi'], ['Rahasia', 'PDF + lampiran', 'PDF + lampiran', 'tanpa PDF/lampiran']].forEach((row, r) => {
    const y = 1.87 + r * 0.48;
    [0.7, 3.28, 6.65, 9.2].forEach((x, c) => {
      s.addShape(pptx.ShapeType.rect, { x, y, w: [2.5, 3.27, 2.47, 3.15][c], h: 0.45, fill: { color: r === 0 ? C.navy : r === 3 ? 'FFF5E6' : C.white }, line: { color: C.line, width: 0.6 } });
      s.addText(row[c], { x: x + 0.1, y: y + 0.15, w: [2.3, 3.05, 2.25, 2.95][c], h: 0.1, fontSize: r === 0 ? 8.3 : 8.7, bold: r === 0 || c === 0, color: r === 0 ? C.white : (r === 3 && c === 3 ? C.coral : C.ink), align: c === 0 ? 'left' : 'center', margin: 0, fit: 'shrink' });
    });
  });
  card(s, 0.68, 4.18, 3.8, 1.35, 'Lampiran terkarantina', 'Format diperiksa, dipindai ClamAV, dan hanya file clean dapat dipakai.', C.amber, { bodySize: 9.7 });
  card(s, 4.78, 4.18, 3.8, 1.35, 'Unduhan terautentikasi', 'PDF/lampiran tidak dibagikan lewat URL object storage publik.', C.sea, { bodySize: 9.7 });
  card(s, 8.88, 4.18, 3.8, 1.35, 'Audit & retry', 'Aksi penting, notifikasi dan publikasi dicatat serta dapat diulang aman.', C.green, { bodySize: 9.7 });
  s.addShape(pptx.ShapeType.roundRect, { x: 0.68, y: 6.05, w: 12.0, h: 0.48, rectRadius: 0.05, fill: { color: C.navy }, line: { color: C.navy } });
  s.addText('Prinsip management: dokumen hanya bergerak bila proses, akses, dan bukti digitalnya sama-sama terjaga.', { x: 0.95, y: 6.19, w: 11.45, h: 0.15, fontSize: 10, bold: true, color: C.white, align: 'center', margin: 0 });
  addFooter(s, 6);
}

// 7 — Experience
{
  const s = pptx.addSlide(); addBg(s); addHeader(s, 'Pengalaman pengguna: membuat di web, menyetujui dari mana saja', 'ADOPSI PENGGUNA');
  s.addShape(pptx.ShapeType.roundRect, { x: 0.9, y: 1.5, w: 5.25, h: 3.45, rectRadius: 0.08, fill: { color: C.white }, line: { color: C.line }, shadow: shadow() });
  s.addShape(pptx.ShapeType.rect, { x: 0.9, y: 1.5, w: 5.25, h: 0.42, fill: { color: C.navy }, line: { color: C.navy } });
  s.addText('WEB / TABLET — Tulis Surat', { x: 1.15, y: 1.64, w: 2.7, h: 0.1, fontSize: 8.2, bold: true, color: C.white, margin: 0 });
  s.addShape(pptx.ShapeType.roundRect, { x: 1.22, y: 2.22, w: 1.1, h: 2.2, rectRadius: 0.04, fill: { color: C.ice }, line: { color: C.line } });
  s.addText('Template\nPenerima\nLampiran\nPreview', { x: 1.42, y: 2.5, w: 0.72, h: 1.35, fontSize: 8.2, color: C.slate, margin: 0, breakLine: false, fit: 'shrink' });
  s.addShape(pptx.ShapeType.roundRect, { x: 2.6, y: 2.22, w: 3.05, h: 0.48, rectRadius: 0.04, fill: { color: C.ice }, line: { color: C.line } });
  s.addText('Yth. Penerima Surat', { x: 2.82, y: 2.4, w: 2.6, h: 0.1, fontSize: 8.5, color: C.ink, margin: 0 });
  miniLine(s, 2.72, 3.14, 2.75); miniLine(s, 2.72, 3.47, 2.45); miniLine(s, 2.72, 3.8, 2.65);
  s.addShape(pptx.ShapeType.roundRect, { x: 4.25, y: 4.18, w: 1.22, h: 0.32, rectRadius: 0.04, fill: { color: C.teal }, line: { color: C.teal } });
  s.addText('AJUKAN', { x: 4.45, y: 4.29, w: 0.82, h: 0.08, fontSize: 7.6, bold: true, color: C.white, align: 'center', margin: 0 });
  s.addShape(pptx.ShapeType.roundRect, { x: 8.33, y: 1.28, w: 2.18, h: 4.05, rectRadius: 0.18, fill: { color: '152A34' }, line: { color: '152A34' }, shadow: shadow() });
  s.addShape(pptx.ShapeType.roundRect, { x: 8.48, y: 1.55, w: 1.88, h: 3.48, rectRadius: 0.1, fill: { color: C.white }, line: { color: C.white } });
  s.addText('ANDROID', { x: 8.75, y: 1.82, w: 1.35, h: 0.1, fontSize: 7.5, bold: true, color: C.teal, align: 'center', margin: 0 });
  s.addText('Approval menunggu', { x: 8.64, y: 2.18, w: 1.56, h: 0.19, fontSize: 9, bold: true, color: C.ink, align: 'center', margin: 0, fit: 'shrink' });
  s.addShape(pptx.ShapeType.roundRect, { x: 8.65, y: 2.6, w: 1.54, h: 0.88, rectRadius: 0.04, fill: { color: C.ice }, line: { color: C.line } });
  s.addText('SURAT URGENT\nPT KSK Group', { x: 8.79, y: 2.84, w: 1.25, h: 0.28, fontSize: 7.5, bold: true, color: C.ink, align: 'center', margin: 0, fit: 'shrink' });
  s.addShape(pptx.ShapeType.roundRect, { x: 8.67, y: 3.86, w: 0.67, h: 0.3, rectRadius: 0.04, fill: { color: C.green }, line: { color: C.green } });
  s.addShape(pptx.ShapeType.roundRect, { x: 9.5, y: 3.86, w: 0.67, h: 0.3, rectRadius: 0.04, fill: { color: C.amber }, line: { color: C.amber } });
  s.addText('SETUJU', { x: 8.73, y: 3.97, w: 0.56, h: 0.06, fontSize: 5.6, bold: true, color: C.white, align: 'center', margin: 0 });
  s.addText('REVISI', { x: 9.57, y: 3.97, w: 0.52, h: 0.06, fontSize: 5.6, bold: true, color: C.white, align: 'center', margin: 0 });
  s.addText('Ilustrasi UI responsif: fungsi inti tetap konsisten pada layar desktop, tablet, dan smartphone.', { x: 7.0, y: 5.0, w: 4.8, h: 0.22, fontSize: 8.7, italic: true, color: C.slate, align: 'center', margin: 0, fit: 'shrink' });
  card(s, 0.9, 5.35, 3.65, 0.92, 'Pembuat surat', 'Template, penerima, lampiran, preview dan status.', C.teal, { bodySize: 9.2 });
  card(s, 4.85, 5.35, 3.65, 0.92, 'Approver', 'Antrian prioritas, aksi setuju/revisi/tolak, tanda tangan approval.', C.sea, { bodySize: 9.2 });
  card(s, 8.8, 5.35, 3.65, 0.92, 'Mobile Android', 'Notifikasi, approval aman, unduh terautentikasi, pengingat SLA.', C.green, { bodySize: 9.2 });
  addFooter(s, 7);
}

// 8 — Metrics
{
  const s = pptx.addSlide(); addBg(s, C.navy);
  s.addText('Nilai bisnis yang perlu diukur', { x: 0.65, y: 0.7, w: 7.5, h: 0.45, fontFace: 'Aptos Display', fontSize: 28, bold: true, color: C.white, margin: 0 });
  s.addText('Target diukur pada pilot, 3 bulan, dan 6 bulan pasca go-live', { x: 0.67, y: 1.28, w: 7, h: 0.25, fontSize: 11.5, color: 'B9DCD8', margin: 0 });
  const metrics = [['<24 jam', 'approval normal'], ['<4 jam', 'surat urgent'], ['<30 detik', 'temu arsip'], ['≥80%', 'penurunan cetak']];
  metrics.forEach((m, i) => {
    const x = 0.68 + (i % 2) * 6.08, y = 1.9 + Math.floor(i / 2) * 2.15;
    s.addShape(pptx.ShapeType.roundRect, { x, y, w: 5.48, h: 1.62, rectRadius: 0.08, fill: { color: i === 3 ? '0B5E65' : '0A5360' }, line: { color: '248B90', transparency: 35 }, shadow: shadow() });
    s.addText(m[0], { x: x + 0.28, y: y + 0.33, w: 2.4, h: 0.46, fontSize: 27, bold: true, color: C.mint, margin: 0, fit: 'shrink' });
    s.addText('TARGET', { x: x + 4.38, y: y + 0.25, w: 0.72, h: 0.1, fontSize: 6.8, bold: true, color: C.mint, align: 'right', margin: 0 });
    s.addText(m[1], { x: x + 0.3, y: y + 0.95, w: 4.5, h: 0.22, fontSize: 12, color: C.white, margin: 0 });
  });
  s.addText('Leading indicators: adopsi ≥70% pengguna aktif mingguan | ≥50% approval Direksi/GM melalui Android | error rate <1%', { x: 0.7, y: 6.4, w: 11.8, h: 0.2, fontSize: 9.5, color: 'B9DCD8', align: 'center', margin: 0, fit: 'shrink' });
  s.addShape(pptx.ShapeType.line, { x: 0.55, y: 7.02, w: 12.2, h: 0, line: { color: '2B7880', width: 0.6 } });
  s.addText('Sumber: PRD eOffice Pro, Ringkasan Eksekutif, dan implementasi flow terkini', { x: 0.55, y: 7.1, w: 10.6, h: 0.16, fontSize: 7.5, color: 'B9DCD8', margin: 0 });
  s.addText('08', { x: 12.35, y: 7.08, w: 0.4, h: 0.2, fontSize: 8, bold: true, color: C.mint, align: 'right', margin: 0 });
}

// 9 — Decisions
{
  const s = pptx.addSlide(); addBg(s); addHeader(s, 'Keputusan yang dibutuhkan dari management', 'CALL TO ACTION');
  const decisions = [
    ['1', 'Keabsahan approval elektronik', 'Tetapkan SK internal, batas kewenangan, dan bukti persetujuan yang diakui perusahaan.'],
    ['2', 'Kebijakan SK / SP & PSrE', 'SK dan SP tetap tidak diajukan elektronik sampai Legal/Direksi menyetujui kebijakan atau PSrE tersedia.'],
    ['3', 'Penomoran, hosting & DR', 'Sahkan format nomor; tetapkan on-premise/cloud, backup, serta kesiapan koneksi site regional.'],
    ['4', 'Sponsor & pilot', 'Tunjuk sponsor eksekutif dan unit pilot untuk adopsi yang terukur.'],
  ];
  decisions.forEach((d, i) => {
    const y = 1.42 + i * 1.23;
    s.addShape(pptx.ShapeType.ellipse, { x: 0.72, y: y + 0.04, w: 0.52, h: 0.52, fill: { color: [C.teal, C.amber, C.sea, C.green][i] }, line: { color: [C.teal, C.amber, C.sea, C.green][i] } });
    s.addText(d[0], { x: 0.72, y: y + 0.18, w: 0.52, h: 0.12, fontSize: 10, bold: true, color: C.white, align: 'center', margin: 0 });
    s.addText(d[1], { x: 1.48, y, w: 3.4, h: 0.22, fontSize: 15, bold: true, color: C.ink, margin: 0 });
    s.addText(d[2], { x: 4.95, y: y + 0.02, w: 7.2, h: 0.36, fontSize: 10.5, color: C.slate, margin: 0, fit: 'shrink' });
    if (i < decisions.length - 1) s.addShape(pptx.ShapeType.line, { x: 1.48, y: y + 0.84, w: 10.7, h: 0, line: { color: C.line, width: 0.7 } });
  });
  s.addShape(pptx.ShapeType.roundRect, { x: 0.72, y: 6.12, w: 11.98, h: 0.52, rectRadius: 0.06, fill: { color: C.teal }, line: { color: C.teal } });
  s.addText('Hasil yang diharapkan: keputusan bergerak lebih cepat, dokumen dapat dipercaya, dan tanggung jawab terlihat.', { x: 0.98, y: 6.28, w: 11.45, h: 0.14, fontSize: 10.2, bold: true, color: C.white, align: 'center', margin: 0 });
  addFooter(s, 9);
}

pptx.writeFile({ fileName: path.join(__dirname, 'Tulis-Surat-Approval-Management.pptx') });
