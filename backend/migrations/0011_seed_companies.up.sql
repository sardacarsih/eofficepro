INSERT INTO companies (code, name, is_active)
VALUES
    ('KSK', 'PT. KALIMANTAN SAWIT KUSUMA', true),
    ('MTS', 'MITRA TANI SEJAHTERA', true),
    ('BBL', 'BERKAH BERSAMA LESTARI', true),
    ('FKK', 'PT. FAJAR KITA KUSUMA', true),
    ('DSU', 'PT. DWI SETIA UTAMA', true),
    ('FMA', 'PT. FAJAR MITRA AGRO', true),
    ('MSL', 'PT. MITRA SAUDARA LESTARI', true),
    ('FSL', 'PT. FAJAR SAUDARA LESTARI', true),
    ('PFL', 'KOP. PRODUSEN FAJAR LESTARI', true),
    ('FBN', 'PT. FAJAR BAHARI NUSANTARA', true),
    ('FTN', 'PT. FAJAR TIRTA NATURAL', true),
    ('FBB', 'PT. FAJAR BAHARI BERSAMA', true),
    ('MJL', 'PT. MITRA JERUK LESTARI', true),
    ('GRL', 'PT. GLOBAL REGENERASI LESTARI', true),
    ('MSS', 'KOP. MITRA SEJAHTERA SEJATI', true),
    ('FBM', 'FAJAR BALAI MANDIRI', true),
    ('FMB', 'FAJAR MITRA BAHARI', true),
    ('FPM', 'FAJAR PANGAN MANDIRI', true),
    ('FSK', 'PT. FAJAR SAUDARA KUSUMA', true),
    ('FAK', 'PT. FAJAR AGRO KALIMANTAN', true),
    ('KSR', 'CV. KALIMANTAN SAWIT RISET', true)
ON CONFLICT (code) DO NOTHING;
