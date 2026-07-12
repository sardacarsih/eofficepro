// Seed data dev minimal: go run ./cmd/seed
// Idempoten — aman dijalankan ulang untuk menyiapkan akun admin dan jabatan pembuat.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/kskgroup/eofficepro/internal/auth"
	"github.com/kskgroup/eofficepro/internal/config"
)

type orgSeedUnit struct {
	Code       string
	ParentCode string
	Name       string
	Level      string
	Region     string
}

type seededPositions struct {
	PresidentID     string
	VPID            string
	Directors       map[string]string
	Secretaries     map[string]string
	GMs             map[string]string
	BiroSecretaries map[string]string
	DeptHeads       map[string]string
	SubDeptHeads    map[string]string
	DivisionHeads   map[string]string
}

var orgSeedUnits = []orgSeedUnit{
	{Code: "PRESIDIR", Name: "President Director", Level: "office", Region: "HO"},
	{Code: "INSPECT", ParentCode: "PRESIDIR", Name: "Inspectorate Internal Audit & Risk Management", Level: "directorate", Region: "HO"},
	{Code: "VPD", ParentCode: "PRESIDIR", Name: "Vice President Director", Level: "office", Region: "HO"},

	{Code: "DIR-SM", ParentCode: "VPD", Name: "Directorate Sales & Marketing", Level: "directorate", Region: "HO"},
	{Code: "DEP-MKT-REL", ParentCode: "DIR-SM", Name: "Department Marketing Relation", Level: "department", Region: "HO"},
	{Code: "DEP-SALES-ADM", ParentCode: "DIR-SM", Name: "Department Sales & Administration", Level: "department", Region: "HO"},

	{Code: "DIR-PRL", ParentCode: "VPD", Name: "Directorate Public Relation, Legal, Licensing, Partnership & CSR", Level: "directorate", Region: "HO"},
	{Code: "BIR-PRL", ParentCode: "DIR-PRL", Name: "Biro Public Relation, Legal & Licensing, Partnership & CSR Program", Level: "biro", Region: "HO"},
	{Code: "DEP-PR", ParentCode: "BIR-PRL", Name: "Department Public Relation", Level: "department", Region: "HO"},
	{Code: "DEP-LEGAL-LIC", ParentCode: "BIR-PRL", Name: "Department Legal Advocasi & Licensing", Level: "department", Region: "HO"},
	{Code: "DEP-PARTNER", ParentCode: "BIR-PRL", Name: "Department Partnership", Level: "department", Region: "HO"},
	{Code: "DEP-CSR-R1", ParentCode: "BIR-PRL", Name: "Department CSR Program Regional I", Level: "department", Region: "REG1"},
	{Code: "DEP-CSR-R2", ParentCode: "BIR-PRL", Name: "Department CSR Program Regional II", Level: "department", Region: "REG2"},

	{Code: "DIR-IS", ParentCode: "VPD", Name: "Directorate Information System", Level: "directorate", Region: "HO"},
	{Code: "DEP-IT-SW", ParentCode: "DIR-IS", Name: "Department IT Software", Level: "department", Region: "HO"},
	{Code: "DEP-IT-HW", ParentCode: "DIR-IS", Name: "Department IT Hardware", Level: "department", Region: "HO"},

	{Code: "DIR-FA", ParentCode: "VPD", Name: "Directorate Finance & Accounting", Level: "directorate", Region: "HO"},
	{Code: "BIR-FA", ParentCode: "DIR-FA", Name: "Biro Finance & Accounting", Level: "biro", Region: "HO"},
	{Code: "DEP-HO-FIN", ParentCode: "BIR-FA", Name: "Department HO Finance", Level: "department", Region: "HO"},
	{Code: "DEP-HO-ACC", ParentCode: "BIR-FA", Name: "Department HO Accounting", Level: "department", Region: "HO"},
	{Code: "DEP-FA-R1", ParentCode: "BIR-FA", Name: "Department Finance & Accounting Regional I", Level: "department", Region: "REG1"},
	{Code: "DEP-FA-R2", ParentCode: "BIR-FA", Name: "Department Finance & Accounting Regional II", Level: "department", Region: "REG2"},
	{Code: "DEP-FA-PARTNER", ParentCode: "BIR-FA", Name: "Department Finance & Accounting Partnership", Level: "department", Region: "HO"},

	{Code: "DIR-PLT", ParentCode: "VPD", Name: "Directorate Plantation", Level: "directorate", Region: "HO"},
	{Code: "BIR-PLT", ParentCode: "DIR-PLT", Name: "Biro Plantation", Level: "biro", Region: "HO"},
	{Code: "DEP-PLT-R1", ParentCode: "BIR-PLT", Name: "Department Plantation Regional I", Level: "department", Region: "REG1"},
	{Code: "DEP-PLT-R2", ParentCode: "BIR-PLT", Name: "Department Plantation Regional II", Level: "department", Region: "REG2"},
	{Code: "DEP-MAP-WMS", ParentCode: "BIR-PLT", Name: "Department Mapping & WMS", Level: "department", Region: "HO"},
	{Code: "DEP-RND", ParentCode: "BIR-PLT", Name: "Department Research & Development", Level: "department", Region: "HO"},

	{Code: "DIR-CE", ParentCode: "VPD", Name: "Directorate Civil & Engineering", Level: "directorate", Region: "HO"},
	{Code: "BIR-CE", ParentCode: "DIR-CE", Name: "Biro Civil & Engineering", Level: "biro", Region: "HO"},
	{Code: "DEP-POM-R1", ParentCode: "BIR-CE", Name: "Department Palm Oil Mill Regional I", Level: "department", Region: "REG1"},
	{Code: "DEP-POM-R2", ParentCode: "BIR-CE", Name: "Department Palm Oil Mill Regional II", Level: "department", Region: "REG2"},
	{Code: "DEP-PACK-FERT", ParentCode: "BIR-CE", Name: "Department Packaging Fertilizer", Level: "department", Region: "HO"},
	{Code: "DEP-CIVIL", ParentCode: "BIR-CE", Name: "Department Civil", Level: "department", Region: "HO"},
	{Code: "DEP-FFB-GRADE", ParentCode: "BIR-CE", Name: "Department FFB Grading", Level: "department", Region: "HO"},

	{Code: "DIR-HRGA", ParentCode: "VPD", Name: "Directorate Human Resources & General Affairs", Level: "directorate", Region: "HO"},
	{Code: "BIR-HRGA", ParentCode: "DIR-HRGA", Name: "Biro Human Resources & General Affairs", Level: "biro", Region: "HO"},
	{Code: "DEP-HO-HRGA", ParentCode: "BIR-HRGA", Name: "Department HO HRGA", Level: "department", Region: "HO"},
	{Code: "DEP-HRGA-R1", ParentCode: "BIR-HRGA", Name: "Department HRGA Regional I", Level: "department", Region: "REG1"},
	{Code: "DEP-HRGA-R2", ParentCode: "BIR-HRGA", Name: "Department HRGA Regional II", Level: "department", Region: "REG2"},
	{Code: "DEP-TRAINING", ParentCode: "BIR-HRGA", Name: "Department Training Center", Level: "department", Region: "HO"},
	{Code: "OFF-REPO-JKT", ParentCode: "BIR-HRGA", Name: "Representative Office Jakarta", Level: "office", Region: "REPO_JKT"},
	{Code: "OFF-REPO-PKB", ParentCode: "BIR-HRGA", Name: "Representative Office Pangkalan Bun", Level: "office", Region: "REPO_PKB"},

	{Code: "DIR-SE", ParentCode: "VPD", Name: "Directorate Sustainability & Environment", Level: "directorate", Region: "HO"},
	{Code: "BIR-SE", ParentCode: "DIR-SE", Name: "Biro Sustainability & Environment", Level: "biro", Region: "HO"},
	{Code: "DEP-COMPLIANCE", ParentCode: "BIR-SE", Name: "Department Compliance", Level: "department", Region: "HO"},
	{Code: "DEP-ENV", ParentCode: "BIR-SE", Name: "Department Environment", Level: "department", Region: "HO"},
	{Code: "DEP-SAFETY", ParentCode: "BIR-SE", Name: "Department Safety", Level: "department", Region: "HO"},
	{Code: "DEP-HEALTH-R1", ParentCode: "BIR-SE", Name: "Department Health Regional I", Level: "department", Region: "REG1"},
	{Code: "DEP-HEALTH-R2", ParentCode: "BIR-SE", Name: "Department Health Regional II", Level: "department", Region: "REG2"},

	{Code: "DIR-TPP", ParentCode: "VPD", Name: "Directorate Tax, Purchase & Port", Level: "directorate", Region: "HO"},
	{Code: "BIR-TPP", ParentCode: "DIR-TPP", Name: "Biro Tax, Purchase & Port", Level: "biro", Region: "HO"},
	{Code: "DEP-TAX", ParentCode: "BIR-TPP", Name: "Department Tax", Level: "department", Region: "HO"},
	{Code: "DEP-PURCH-LOG", ParentCode: "BIR-TPP", Name: "Department Purchase & Logistic", Level: "department", Region: "HO"},
	{Code: "DEP-PORT", ParentCode: "BIR-TPP", Name: "Department Port", Level: "department", Region: "HO"},

	// Division Head labels transcribed from docs/KSK HOLDING  2026- des25.pdf.
	{Code: "DIV-IT-SW", ParentCode: "DEP-IT-SW", Name: "Division IT Software", Level: "division", Region: "HO"},
	{Code: "DIV-CSR-R1-CORPSR", ParentCode: "DEP-CSR-R1", Name: "Division Corporate & Social Responsibility", Level: "division", Region: "REG1"},
	{Code: "DIV-CSR-R2-CSR", ParentCode: "DEP-CSR-R2", Name: "Division CSR Program", Level: "division", Region: "REG2"},
	{Code: "DIV-PARTNER-PART", ParentCode: "DEP-PARTNER", Name: "Division Partnership", Level: "division", Region: "HO"},
	{Code: "DIV-PARTNER-PADM", ParentCode: "DEP-PARTNER", Name: "Division Partnership Administration", Level: "division", Region: "HO"},
	{Code: "DIV-PARTNER-ADMP", ParentCode: "DEP-PARTNER", Name: "Division Administration Partnership", Level: "division", Region: "HO"},
	{Code: "DIV-PR-PUBREL", ParentCode: "DEP-PR", Name: "Division Public Relation", Level: "division", Region: "HO"},

	{Code: "DIV-FA-R1-FIN", ParentCode: "DEP-FA-R1", Name: "Division Finance", Level: "division", Region: "REG1"},
	{Code: "DIV-FA-R1-ACC", ParentCode: "DEP-FA-R1", Name: "Division Accounting", Level: "division", Region: "REG1"},
	{Code: "DIV-FA-R1-LOG", ParentCode: "DEP-FA-R1", Name: "Division Logistic", Level: "division", Region: "REG1"},
	{Code: "DIV-FA-R2-FIN", ParentCode: "DEP-FA-R2", Name: "Division Finance", Level: "division", Region: "REG2"},
	{Code: "DIV-FA-R2-ACC", ParentCode: "DEP-FA-R2", Name: "Division Accounting", Level: "division", Region: "REG2"},
	{Code: "DIV-FA-R2-LOG", ParentCode: "DEP-FA-R2", Name: "Division Logistic", Level: "division", Region: "REG2"},

	{Code: "DIV-PLT-R1-NE", ParentCode: "DEP-PLT-R1", Name: "Division Plantation NE", Level: "division", Region: "REG1"},
	{Code: "DIV-PLT-R1-SE", ParentCode: "DEP-PLT-R1", Name: "Division Plantation SE", Level: "division", Region: "REG1"},
	{Code: "DIV-PLT-R1-BBLE", ParentCode: "DEP-PLT-R1", Name: "Division Plantation BBLE", Level: "division", Region: "REG1"},
	{Code: "DIV-PLT-R1-SBE", ParentCode: "DEP-PLT-R1", Name: "Division Plantation SBE", Level: "division", Region: "REG1"},
	{Code: "DIV-PLT-R1-JE", ParentCode: "DEP-PLT-R1", Name: "Division Plantation JE", Level: "division", Region: "REG1"},
	{Code: "DIV-PLT-R1-FMA", ParentCode: "DEP-PLT-R1", Name: "Division Plantation FMA", Level: "division", Region: "REG1"},
	{Code: "DIV-PLT-R1-ADMIN", ParentCode: "DEP-PLT-R1", Name: "Division Administration", Level: "division", Region: "REG1"},
	{Code: "DIV-PLT-R2-MBE", ParentCode: "DEP-PLT-R2", Name: "Division Plantation MBE", Level: "division", Region: "REG2"},
	{Code: "DIV-PLT-R2-GLE", ParentCode: "DEP-PLT-R2", Name: "Division Plantation GLE", Level: "division", Region: "REG2"},
	{Code: "DIV-PLT-R2-TERINDAK", ParentCode: "DEP-PLT-R2", Name: "Division Plantation Terindak", Level: "division", Region: "REG2"},
	{Code: "DIV-PLT-R2-ADMIN", ParentCode: "DEP-PLT-R2", Name: "Division Administration", Level: "division", Region: "REG2"},
	{Code: "DIV-MAP-MAPPING", ParentCode: "DEP-MAP-WMS", Name: "Division Mapping", Level: "division", Region: "HO"},
	{Code: "DIV-MAP-WMS", ParentCode: "DEP-MAP-WMS", Name: "Division Water Management System", Level: "division", Region: "HO"},
	{Code: "DIV-RND-FERTPROD", ParentCode: "DEP-RND", Name: "Division Fertilizer & Production", Level: "division", Region: "HO"},
	{Code: "DIV-RND-PEST", ParentCode: "DEP-RND", Name: "Division Pest & Diseases Plant Protection", Level: "division", Region: "HO"},

	{Code: "DIV-POM-R1-MAINT", ParentCode: "DEP-POM-R1", Name: "Division Maintenance", Level: "division", Region: "REG1"},
	{Code: "DIV-POM-R1-ELEC", ParentCode: "DEP-POM-R1", Name: "Division Electrical", Level: "division", Region: "REG1"},
	{Code: "DIV-POM-R1-PROCESS", ParentCode: "DEP-POM-R1", Name: "Division Process", Level: "division", Region: "REG1"},
	{Code: "DIV-POM-R1-LAB", ParentCode: "DEP-POM-R1", Name: "Division Laboratorium", Level: "division", Region: "REG1"},
	{Code: "DIV-POM-R1-ADMIN", ParentCode: "DEP-POM-R1", Name: "Division Administration", Level: "division", Region: "REG1"},
	{Code: "DIV-POM-R2-MAINT", ParentCode: "DEP-POM-R2", Name: "Division Maintenance", Level: "division", Region: "REG2"},
	{Code: "DIV-POM-R2-ELEC", ParentCode: "DEP-POM-R2", Name: "Division Electrical", Level: "division", Region: "REG2"},
	{Code: "DIV-POM-R2-PROSES", ParentCode: "DEP-POM-R2", Name: "Division Proses", Level: "division", Region: "REG2"},
	{Code: "DIV-POM-R2-LAB", ParentCode: "DEP-POM-R2", Name: "Division Laboratorium", Level: "division", Region: "REG2"},
	{Code: "DIV-POM-R2-BIOMASS", ParentCode: "DEP-POM-R2", Name: "Division Biomass Pelleting FSK", Level: "division", Region: "REG2"},
	{Code: "DIV-POM-R2-ADMIN", ParentCode: "DEP-POM-R2", Name: "Division Administration", Level: "division", Region: "REG2"},
	{Code: "DIV-FFB-GRADE", ParentCode: "DEP-FFB-GRADE", Name: "Division FFB Grading", Level: "division", Region: "HO"},
	{Code: "DIV-PACK-FERT", ParentCode: "DEP-PACK-FERT", Name: "Division Packaging Fertilizer", Level: "division", Region: "HO"},
	{Code: "DIV-CIVIL-CIVIL", ParentCode: "DEP-CIVIL", Name: "Division Civil", Level: "division", Region: "HO"},

	{Code: "DIV-HRGA-R1-GA", ParentCode: "DEP-HRGA-R1", Name: "Division General Affairs", Level: "division", Region: "REG1"},
	{Code: "DIV-HRGA-R1-HR", ParentCode: "DEP-HRGA-R1", Name: "Division Human Resources", Level: "division", Region: "REG1"},
	{Code: "DIV-HRGA-R1-TRANS", ParentCode: "DEP-HRGA-R1", Name: "Division Transport", Level: "division", Region: "REG1"},
	{Code: "DIV-HRGA-R2-GA", ParentCode: "DEP-HRGA-R2", Name: "Division General Affairs", Level: "division", Region: "REG2"},
	{Code: "DIV-HRGA-R2-HR", ParentCode: "DEP-HRGA-R2", Name: "Division Human Resources", Level: "division", Region: "REG2"},
	{Code: "DIV-HRGA-R2-HEALTH", ParentCode: "DEP-HRGA-R2", Name: "Division Health", Level: "division", Region: "REG2"},
	{Code: "DIV-HRGA-R2-TRANS", ParentCode: "DEP-HRGA-R2", Name: "Division Transport", Level: "division", Region: "REG2"},

	{Code: "DIV-COMPLIANCE", ParentCode: "DEP-COMPLIANCE", Name: "Division Compliance", Level: "division", Region: "HO"},
	{Code: "DIV-SAFETY", ParentCode: "DEP-SAFETY", Name: "Division Safety", Level: "division", Region: "HO"},
	{Code: "DIV-ENV", ParentCode: "DEP-ENV", Name: "Division Environment", Level: "division", Region: "HO"},

	{Code: "DIV-PORT-ADMIN", ParentCode: "DEP-PORT", Name: "Division Administration Port", Level: "division", Region: "HO"},
	{Code: "DIV-PORT-OPR", ParentCode: "DEP-PORT", Name: "Division Operational Port", Level: "division", Region: "HO"},
}

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	email := getenv("ADMIN_EMAIL", "admin@ksk.local")
	password := getenv("ADMIN_PASSWORD", "GantiSegera#2026")
	userID := ensureAdminUser(ctx, db, email, password)
	companyID := ensureCompany(ctx, db)
	orgUnitIDs := seedOrganization(ctx, db, companyID)
	positions := seedPositions(ctx, db, orgUnitIDs)

	creatorPositionID := positions.DeptHeads["DEP-IT-SW"]
	ensureUserPosition(ctx, db, userID, creatorPositionID)
	approverID := ensureDevUser(ctx, db, "APR001", getenv("APPROVER_EMAIL", "approver@ksk.local"), "Approver Development", password, nil)
	ensureUserPosition(ctx, db, approverID, positions.Directors["DIR-IS"])

	divisionCreatorID := ensureDevUser(ctx, db, "DIV001", getenv("DIVISION_CREATOR_EMAIL", "division.creator@ksk.local"), "Division Creator Finance", password, []string{"creator"})
	ensureUserPosition(ctx, db, divisionCreatorID, positions.DivisionHeads["DIV-FA-R1-FIN"])
	deptApproverID := ensureDevUser(ctx, db, "DFA001", getenv("DEPT_APPROVER_EMAIL", "dept.approver.fa@ksk.local"), "Department Approver Finance Regional I", password, nil)
	ensureUserPosition(ctx, db, deptApproverID, positions.DeptHeads["DEP-FA-R1"])
	directorApproverID := ensureDevUser(ctx, db, "DIRFA001", getenv("DIRECTOR_FA_EMAIL", "director.fa@ksk.local"), "Director Approver Finance", password, nil)
	ensureUserPosition(ctx, db, directorApproverID, positions.Directors["DIR-FA"])
	vpApproverID := ensureDevUser(ctx, db, "VP001", getenv("VP_APPROVER_EMAIL", "vp.approver@ksk.local"), "Vice President Director Approver", password, nil)
	ensureUserPosition(ctx, db, vpApproverID, positions.VPID)
	presidentApproverID := ensureDevUser(ctx, db, "PD001", getenv("PRESIDENT_APPROVER_EMAIL", "president.approver@ksk.local"), "President Director Approver", password, nil)
	ensureUserPosition(ctx, db, presidentApproverID, positions.PresidentID)
	seedGMUsers(ctx, db, password, positions)
	seedSecretaryUsers(ctx, db, password, positions)
	removeLegacyDevSeed(ctx, db, companyID)

	log.Printf("seed siap: %s (NIK ADM001) punya role admin/creator; %d unit organisasi aktif tersedia", email, len(orgSeedUnits))
}

func seedGMUsers(ctx context.Context, db *pgxpool.Pool, password string, positions seededPositions) {
	gmUsers := map[string]struct {
		nik      string
		email    string
		fullName string
	}{
		"BIR-PRL":  {nik: "GMPRL001", email: "gm.prl@ksk.local", fullName: "GM Public Relation, Legal & Licensing, Partnership & CSR Program"},
		"BIR-FA":   {nik: "GMFA001", email: "gm.fa@ksk.local", fullName: "GM Finance & Accounting"},
		"BIR-PLT":  {nik: "GMPLT001", email: "gm.plt@ksk.local", fullName: "GM Plantation"},
		"BIR-CE":   {nik: "GMCE001", email: "gm.ce@ksk.local", fullName: "GM Civil & Engineering"},
		"BIR-HRGA": {nik: "GMHRGA001", email: "gm.hrga@ksk.local", fullName: "GM Human Resources & General Affairs"},
		"BIR-SE":   {nik: "GMSE001", email: "gm.se@ksk.local", fullName: "GM Sustainability & Environment"},
		"BIR-TPP":  {nik: "GMTPP001", email: "gm.tpp@ksk.local", fullName: "GM Tax, Purchase & Port"},
	}

	for biroCode, gmPositionID := range positions.GMs {
		user, ok := gmUsers[biroCode]
		if !ok {
			log.Fatalf("seed user gm untuk %s belum dikonfigurasi", biroCode)
		}
		userID := ensureDevUser(ctx, db, user.nik, user.email, user.fullName, password, nil)
		ensureUserPosition(ctx, db, userID, gmPositionID)
	}
}

func seedSecretaryUsers(ctx context.Context, db *pgxpool.Pool, password string, positions seededPositions) {
	secretaryUsers := map[string]struct {
		nik      string
		email    string
		fullName string
	}{
		"DIR-SM":   {nik: "SECSM001", email: "secretary.sm@ksk.local", fullName: "Secretary Director Sales & Marketing"},
		"DIR-PRL":  {nik: "SECPRL001", email: "secretary.prl@ksk.local", fullName: "Secretary Director Public Relation, Legal, Licensing, Partnership & CSR"},
		"DIR-IS":   {nik: "SECIS001", email: "secretary.is@ksk.local", fullName: "Secretary Director Information System"},
		"DIR-FA":   {nik: "SECFA001", email: getenv("SECRETARY_FA_EMAIL", "secretary.fa@ksk.local"), fullName: "Secretary Director Finance & Accounting"},
		"DIR-PLT":  {nik: "SECPLT001", email: "secretary.plt@ksk.local", fullName: "Secretary Director Plantation"},
		"DIR-CE":   {nik: "SECCE001", email: "secretary.ce@ksk.local", fullName: "Secretary Director Civil & Engineering"},
		"DIR-HRGA": {nik: "SECHRGA001", email: "secretary.hrga@ksk.local", fullName: "Secretary Director Human Resources & General Affairs"},
		"DIR-SE":   {nik: "SECSE001", email: "secretary.se@ksk.local", fullName: "Secretary Director Sustainability & Environment"},
		"DIR-TPP":  {nik: "SECTPP001", email: "secretary.tpp@ksk.local", fullName: "Secretary Director Tax, Purchase & Port"},
	}

	for directorateCode, secretaryPositionID := range positions.Secretaries {
		user, ok := secretaryUsers[directorateCode]
		if !ok {
			log.Fatalf("seed user secretary untuk %s belum dikonfigurasi", directorateCode)
		}
		userID := ensureDevUser(ctx, db, user.nik, user.email, user.fullName, password, []string{"secretary"})
		ensureUserPosition(ctx, db, userID, secretaryPositionID)
	}

	biroSecretaryUsers := map[string]struct {
		nik      string
		email    string
		fullName string
	}{
		"BIR-PRL":  {nik: "SECGMPRL001", email: "secretary.gm.prl@ksk.local", fullName: "Secretary GM Public Relation, Legal & Licensing, Partnership & CSR Program"},
		"BIR-FA":   {nik: "SECGMFA001", email: getenv("SECRETARY_GM_FA_EMAIL", "secretary.gm.fa@ksk.local"), fullName: "Secretary GM Finance & Accounting"},
		"BIR-PLT":  {nik: "SECGMPLT001", email: "secretary.gm.plt@ksk.local", fullName: "Secretary GM Plantation"},
		"BIR-CE":   {nik: "SECGMCE001", email: "secretary.gm.ce@ksk.local", fullName: "Secretary GM Civil & Engineering"},
		"BIR-HRGA": {nik: "SECGMHRGA001", email: "secretary.gm.hrga@ksk.local", fullName: "Secretary GM Human Resources & General Affairs"},
		"BIR-SE":   {nik: "SECGMSE001", email: "secretary.gm.se@ksk.local", fullName: "Secretary GM Sustainability & Environment"},
		"BIR-TPP":  {nik: "SECGMTPP001", email: "secretary.gm.tpp@ksk.local", fullName: "Secretary GM Tax, Purchase & Port"},
	}

	for biroCode, secretaryPositionID := range positions.BiroSecretaries {
		user, ok := biroSecretaryUsers[biroCode]
		if !ok {
			log.Fatalf("seed user secretary gm untuk %s belum dikonfigurasi", biroCode)
		}
		userID := ensureDevUser(ctx, db, user.nik, user.email, user.fullName, password, []string{"secretary"})
		ensureUserPosition(ctx, db, userID, secretaryPositionID)
	}
}

func ensureAdminUser(ctx context.Context, db *pgxpool.Pool, email string, password string) string {
	var userID string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE nik = 'ADM001' OR email = $1
		LIMIT 1`, email).Scan(&userID)
	if err == nil {
		ensureUserRoles(ctx, db, userID, []string{"admin", "creator"})
		return userID
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("cek admin: %v", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	if err := db.QueryRow(ctx, `
		INSERT INTO users (nik, email, full_name, password_hash)
		VALUES ('ADM001', $1, 'Administrator Sistem', $2) RETURNING id::text`,
		email, hash).Scan(&userID); err != nil {
		log.Fatalf("insert admin: %v", err)
	}
	ensureUserRoles(ctx, db, userID, []string{"admin", "creator"})
	return userID
}

func ensureDevUser(ctx context.Context, db *pgxpool.Pool, nik string, email string, fullName string, password string, roles []string) string {
	var userID string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE nik = $1 OR email = $2
		LIMIT 1`, nik, email).Scan(&userID)
	if err == nil {
		ensureUserRoles(ctx, db, userID, roles)
		return userID
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("cek user %s: %v", nik, err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password %s: %v", nik, err)
	}
	if err := db.QueryRow(ctx, `
		INSERT INTO users (nik, email, full_name, password_hash)
		VALUES ($1, $2, $3, $4) RETURNING id::text`,
		nik, email, fullName, hash).Scan(&userID); err != nil {
		log.Fatalf("insert user %s: %v", nik, err)
	}
	ensureUserRoles(ctx, db, userID, roles)
	return userID
}

func ensureUserRoles(ctx context.Context, db *pgxpool.Pool, userID string, roles []string) {
	if len(roles) == 0 {
		return
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id
		FROM roles
		WHERE code = ANY($2)
		ON CONFLICT DO NOTHING`, userID, roles); err != nil {
		log.Fatalf("assign role: %v", err)
	}
}

func ensureCompany(ctx context.Context, db *pgxpool.Pool) string {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO companies (code, name)
		VALUES ('KSK', 'PT Kalimantan Sawit Kusuma')
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id::text`).Scan(&id)
	if err != nil {
		log.Fatalf("ensure company: %v", err)
	}
	return id
}

func seedOrganization(ctx context.Context, db *pgxpool.Pool, companyID string) map[string]string {
	ids := make(map[string]string, len(orgSeedUnits))
	for _, unit := range orgSeedUnits {
		var parentID *string
		if unit.ParentCode != "" {
			parent, ok := ids[unit.ParentCode]
			if !ok {
				log.Fatalf("parent org unit %s untuk %s belum dibuat", unit.ParentCode, unit.Code)
			}
			parentID = &parent
		}
		ids[unit.Code] = ensureOrgUnit(ctx, db, companyID, parentID, unit.Code, unit.Name, unit.Level, nullableString(unit.Region))
	}
	return ids
}

func seedPositions(ctx context.Context, db *pgxpool.Pool, orgUnitIDs map[string]string) seededPositions {
	presidentID := ensurePosition(ctx, db, orgUnitIDs["PRESIDIR"], "President Director", "president_director", nil, true)
	vpID := ensurePosition(ctx, db, orgUnitIDs["VPD"], "Vice President Director", "vp_director", &presidentID, true)
	ensurePosition(ctx, db, orgUnitIDs["INSPECT"], "Head of Inspectorate", "auditor", &presidentID, true)

	positions := seededPositions{
		PresidentID:     presidentID,
		VPID:            vpID,
		Directors:       map[string]string{},
		Secretaries:     map[string]string{},
		GMs:             map[string]string{},
		BiroSecretaries: map[string]string{},
		DeptHeads:       map[string]string{},
		SubDeptHeads:    map[string]string{},
		DivisionHeads:   map[string]string{},
	}
	unitsByCode := map[string]orgSeedUnit{}
	for _, unit := range orgSeedUnits {
		unitsByCode[unit.Code] = unit
	}

	for _, unit := range orgSeedUnits {
		if unit.Level != "directorate" || !strings.HasPrefix(unit.Code, "DIR-") {
			continue
		}
		title := "Director " + strings.TrimPrefix(unit.Name, "Directorate ")
		positions.Directors[unit.Code] = ensurePosition(ctx, db, orgUnitIDs[unit.Code], title, "director", &vpID, true)
	}

	for _, unit := range orgSeedUnits {
		if unit.Level != "directorate" || !strings.HasPrefix(unit.Code, "DIR-") {
			continue
		}
		directorID, ok := positions.Directors[unit.Code]
		if !ok {
			log.Fatalf("director untuk secretary %s tidak ditemukan", unit.Code)
		}
		title := "Secretary Director " + strings.TrimPrefix(unit.Name, "Directorate ")
		positions.Secretaries[unit.Code] = ensurePosition(ctx, db, orgUnitIDs[unit.Code], title, "secretary", &directorID, false)
	}

	for _, unit := range orgSeedUnits {
		if unit.Level != "biro" {
			continue
		}
		directorID, ok := positions.Directors[unit.ParentCode]
		if !ok {
			log.Fatalf("director untuk biro %s tidak ditemukan", unit.Code)
		}
		title := "GM " + strings.TrimPrefix(unit.Name, "Biro ")
		positions.GMs[unit.Code] = ensurePosition(ctx, db, orgUnitIDs[unit.Code], title, "gm", &directorID, true)
	}

	for _, unit := range orgSeedUnits {
		if unit.Level != "biro" {
			continue
		}
		gmID, ok := positions.GMs[unit.Code]
		if !ok {
			log.Fatalf("gm untuk secretary biro %s tidak ditemukan", unit.Code)
		}
		title := "Secretary GM " + strings.TrimPrefix(unit.Name, "Biro ")
		positions.BiroSecretaries[unit.Code] = ensurePosition(ctx, db, orgUnitIDs[unit.Code], title, "secretary", &gmID, false)
	}

	for _, unit := range orgSeedUnits {
		if unit.Level != "department" {
			continue
		}
		reportsToID, ok := reportsToForDepartment(unitsByCode, positions, unit)
		if !ok {
			log.Fatalf("atasan untuk department %s tidak ditemukan", unit.Code)
		}
		title := "Department Head " + strings.TrimPrefix(unit.Name, "Department ")
		positions.DeptHeads[unit.Code] = ensurePosition(ctx, db, orgUnitIDs[unit.Code], title, "dept_head", &reportsToID, true)
	}

	for _, unit := range orgSeedUnits {
		if unit.Level != "department" {
			continue
		}
		deptHeadID, ok := positions.DeptHeads[unit.Code]
		if !ok {
			log.Fatalf("department head untuk sub department %s tidak ditemukan", unit.Code)
		}
		title := "Sub Department Head " + strings.TrimPrefix(unit.Name, "Department ")
		positions.SubDeptHeads[unit.Code] = ensurePosition(ctx, db, orgUnitIDs[unit.Code], title, "sub_dept_head", &deptHeadID, true)
	}

	for _, unit := range orgSeedUnits {
		if unit.Level != "division" {
			continue
		}
		reportsToID, ok := reportsToForDivision(unitsByCode, positions, unit)
		if !ok {
			log.Fatalf("atasan untuk division %s tidak ditemukan", unit.Code)
		}
		title := "Division Head " + strings.TrimPrefix(unit.Name, "Division ")
		positions.DivisionHeads[unit.Code] = ensurePosition(ctx, db, orgUnitIDs[unit.Code], title, "division_head", &reportsToID, true)
	}

	return positions
}

func reportsToForDepartment(unitsByCode map[string]orgSeedUnit, positions seededPositions, unit orgSeedUnit) (string, bool) {
	parent, ok := unitsByCode[unit.ParentCode]
	if !ok {
		return "", false
	}
	if parent.Level == "biro" {
		gmID, ok := positions.GMs[parent.Code]
		return gmID, ok
	}
	if parent.Level == "directorate" {
		directorID, ok := positions.Directors[parent.Code]
		return directorID, ok
	}
	return "", false
}

func reportsToForDivision(unitsByCode map[string]orgSeedUnit, positions seededPositions, unit orgSeedUnit) (string, bool) {
	deptCode, ok := departmentCodeForUnit(unitsByCode, unit)
	if !ok {
		return "", false
	}
	subDeptHeadID, ok := positions.SubDeptHeads[deptCode]
	return subDeptHeadID, ok
}

func departmentCodeForUnit(unitsByCode map[string]orgSeedUnit, unit orgSeedUnit) (string, bool) {
	for parentCode := unit.ParentCode; parentCode != ""; {
		parent, ok := unitsByCode[parentCode]
		if !ok {
			return "", false
		}
		if parent.Level == "department" {
			return parent.Code, true
		}
		parentCode = parent.ParentCode
	}
	return "", false
}

func ensureOrgUnit(ctx context.Context, db *pgxpool.Pool, companyID string, parentID *string, code string, name string, level string, region *string) string {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO org_units (company_id, parent_id, code, name, unit_level, region)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (company_id, code)
		DO UPDATE SET
			name = EXCLUDED.name,
			parent_id = EXCLUDED.parent_id,
			unit_level = EXCLUDED.unit_level,
			region = EXCLUDED.region,
			valid_to = NULL,
			is_active = true
		RETURNING id::text`, companyID, parentID, code, name, level, region).Scan(&id)
	if err != nil {
		log.Fatalf("ensure org unit %s: %v", code, err)
	}
	return id
}

func ensurePosition(ctx context.Context, db *pgxpool.Pool, orgUnitID string, title string, positionType string, reportsTo *string, isApprover bool) string {
	var id string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM positions
		WHERE org_unit_id = $1 AND title = $2 AND position_type = $3 AND is_active
		LIMIT 1`, orgUnitID, title, positionType).Scan(&id)
	if err == nil {
		if _, err := db.Exec(ctx, `
			UPDATE positions
			SET reports_to = $2, is_approver = $3
			WHERE id = $1`, id, reportsTo, isApprover); err != nil {
			log.Fatalf("update position %s: %v", title, err)
		}
		return id
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("cek position %s: %v", title, err)
	}

	if err := db.QueryRow(ctx, `
		INSERT INTO positions (org_unit_id, title, position_type, reports_to, is_approver)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`, orgUnitID, title, positionType, reportsTo, isApprover).Scan(&id); err != nil {
		log.Fatalf("insert position %s: %v", title, err)
	}
	return id
}

func ensureUserPosition(ctx context.Context, db *pgxpool.Pool, userID string, positionID string) {
	var hasActivePosition bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_positions
			WHERE user_id = $1
			  AND position_id = $2
			  AND current_date >= valid_from
			  AND (valid_to IS NULL OR current_date <= valid_to)
		)`, userID, positionID).Scan(&hasActivePosition)
	if err != nil {
		log.Fatalf("cek penempatan admin: %v", err)
	}
	if hasActivePosition {
		return
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO user_positions (user_id, position_id, assignment_type)
		VALUES ($1, $2, 'definitive')`, userID, positionID); err != nil {
		log.Fatalf("assign admin position: %v", err)
	}
}

func removeLegacyDevSeed(ctx context.Context, db *pgxpool.Pool, companyID string) {
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatalf("begin remove legacy dev seed: %v", err)
	}
	defer tx.Rollback(ctx)

	codes := []string{"DEV-DIR", "DEV-DEPT"}
	if _, err := tx.Exec(ctx, `
		DELETE FROM user_positions up
		USING positions p, org_units ou
		WHERE up.position_id = p.id
		  AND p.org_unit_id = ou.id
		  AND ou.company_id = $1
		  AND ou.code = ANY($2)`, companyID, codes); err != nil {
		log.Fatalf("remove legacy user positions: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE positions p
		SET reports_to = NULL
		FROM org_units ou
		WHERE p.org_unit_id = ou.id
		  AND ou.company_id = $1
		  AND ou.code = ANY($2)`, companyID, codes); err != nil {
		log.Fatalf("clear legacy position reports_to: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM positions p
		USING org_units ou
		WHERE p.org_unit_id = ou.id
		  AND ou.company_id = $1
		  AND ou.code = ANY($2)`, companyID, codes); err != nil {
		log.Fatalf("remove legacy positions: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM org_units
		WHERE company_id = $1
		  AND code = ANY($2)`, companyID, codes); err != nil {
		log.Fatalf("remove legacy org units: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit remove legacy dev seed: %v", err)
	}
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
