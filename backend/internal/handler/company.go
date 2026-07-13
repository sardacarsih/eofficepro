package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"

	"github.com/kskgroup/eofficepro/internal/middleware"
)

const (
	maxCompanyLogoSize      = 2 * 1024 * 1024
	minCompanyLogoDimension = 128
	maxCompanyLogoDimension = 4096
)

type companyLogoConfig struct {
	StorageKey     string `json:"storage_key"`
	MIMEType       string `json:"mime_type"`
	FileName       string `json:"file_name"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
}

type companyLetterheadConfig struct {
	Logo *companyLogoConfig `json:"logo,omitempty"`
}

type CompanyLogo struct {
	MIMEType       string `json:"mime_type"`
	FileName       string `json:"file_name"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
}

type Company struct {
	ID       string       `json:"id"`
	Code     string       `json:"code"`
	Name     string       `json:"name"`
	IsActive bool         `json:"is_active"`
	HasLogo  bool         `json:"has_logo"`
	Logo     *CompanyLogo `json:"logo,omitempty"`
	LogoURL  string       `json:"logo_url,omitempty"`
}

func (h *Handler) ListCompanies(c *gin.Context) {
	includeInactive := c.Query("include_inactive") == "true"
	if includeInactive {
		userID := c.GetString(middleware.CtxUserID)
		isSuperAdmin, err := h.userIsSuperAdmin(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
			return
		}
		companyRoles, err := h.companyRoles(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa akses perusahaan"})
			return
		}
		if !isSuperAdmin && len(companyRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "hanya admin yang dapat melihat perusahaan nonaktif"})
			return
		}
	}

	page, pageSize, offset, ok := parsePagination(c.Query("page"), c.Query("page_size"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page atau page_size tidak valid"})
		return
	}

	ctx := c.Request.Context()
	accessible, err := h.accessibleCompanies(ctx, c.GetString(middleware.CtxUserID), includeInactive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat perusahaan"})
		return
	}
	total := int64(len(accessible))
	if offset >= len(accessible) {
		c.JSON(http.StatusOK, gin.H{"data": []Company{}, "meta": newPageMeta(page, pageSize, total)})
		return
	}
	end := min(offset+pageSize, len(accessible))
	companyIDs := make([]string, 0, end-offset)
	for _, company := range accessible[offset:end] {
		companyIDs = append(companyIDs, company.ID)
	}

	query := `
		SELECT id::text, code, name, is_active, letterhead_config
		FROM companies
		WHERE id::text = ANY($1::text[])
		ORDER BY code`

	rows, err := h.DB.Query(ctx, query, companyIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat perusahaan"})
		return
	}
	defer rows.Close()

	companies := []Company{}
	for rows.Next() {
		var company Company
		var rawConfig []byte
		if err := rows.Scan(&company.ID, &company.Code, &company.Name, &company.IsActive, &rawConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data perusahaan"})
			return
		}

		config, err := parseCompanyLetterheadConfig(rawConfig)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "konfigurasi kop perusahaan tidak valid"})
			return
		}
		if config.Logo != nil && strings.TrimSpace(config.Logo.StorageKey) != "" {
			company.HasLogo = true
			company.Logo = &CompanyLogo{
				MIMEType:       config.Logo.MIMEType,
				FileName:       config.Logo.FileName,
				ChecksumSHA256: config.Logo.ChecksumSHA256,
				Width:          config.Logo.Width,
				Height:         config.Logo.Height,
			}
			if h.Minio != nil {
				company.LogoURL, _ = h.presignedGetURL(ctx, config.Logo.StorageKey)
			}
		}
		companies = append(companies, company)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar perusahaan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": companies, "meta": newPageMeta(page, pageSize, total)})
}

type companyRequest struct {
	Code     string `json:"code" binding:"required"`
	Name     string `json:"name" binding:"required"`
	IsActive *bool  `json:"is_active"`
}

func normalizeAndValidateCompanyRequest(req *companyRequest) error {
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" {
		return errors.New("kode perusahaan wajib diisi")
	}
	if utf8.RuneCountInString(req.Code) > 10 {
		return errors.New("kode perusahaan maksimal 10 karakter")
	}
	if req.Name == "" {
		return errors.New("nama perusahaan wajib diisi")
	}
	if utf8.RuneCountInString(req.Name) > 150 {
		return errors.New("nama perusahaan maksimal 150 karakter")
	}
	return nil
}

func (h *Handler) CreateCompany(c *gin.Context) {
	isSuperAdmin, err := h.userIsSuperAdmin(c.Request.Context(), c.GetString(middleware.CtxUserID))
	if err != nil || !isSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "hanya super admin yang dapat membuat perusahaan"})
		return
	}
	var req companyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kode dan nama perusahaan wajib diisi"})
		return
	}
	if err := normalizeAndValidateCompanyRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	ctx := c.Request.Context()
	var id string
	if err := h.DB.QueryRow(ctx, `
		INSERT INTO companies (code, name, is_active)
		VALUES ($1, $2, $3)
		RETURNING id::text`, req.Code, req.Name, isActive).Scan(&id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal membuat perusahaan (kode mungkin sudah digunakan)"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "company", &id, "create", &actor, map[string]any{
		"code": req.Code, "name": req.Name, "is_active": isActive,
	}, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) UpdateCompany(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if !h.requireAdminCompany(c, id) {
		return
	}
	var req companyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kode dan nama perusahaan wajib diisi"})
		return
	}
	if err := normalizeAndValidateCompanyRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tag, err := h.DB.Exec(ctx, `
		UPDATE companies
		SET code = $2,
		    name = $3,
		    is_active = COALESCE($4, is_active)
		WHERE id = $1`, id, req.Code, req.Name, req.IsActive)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "gagal memperbarui perusahaan (kode mungkin sudah digunakan)"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "perusahaan tidak ditemukan"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "company", &id, "update", &actor, map[string]any{
		"code": req.Code, "name": req.Name, "is_active": req.IsActive,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) DeactivateCompany(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if !h.requireAdminCompany(c, id) {
		return
	}
	ctx := c.Request.Context()
	tag, err := h.DB.Exec(ctx, `
		UPDATE companies
		SET is_active = false
		WHERE id = $1 AND is_active`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menonaktifkan perusahaan"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "perusahaan tidak ditemukan atau sudah nonaktif"})
		return
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "company", &id, "deactivate", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *Handler) UploadCompanyLogo(c *gin.Context) {
	if h.Minio == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "object storage belum tersedia"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCompanyLogoSize+1024*1024)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file logo wajib dikirim"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxCompanyLogoSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ukuran logo maksimal 2 MB"})
		return
	}

	data, err := readMultipartFile(fileHeader, maxCompanyLogoSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logo, extension, err := validateCompanyLogo(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logo.FileName = safeObjectFileName(fileHeader.Filename)

	companyID := strings.TrimSpace(c.Param("id"))
	if !h.requireAdminCompany(c, companyID) {
		return
	}
	ctx := c.Request.Context()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai penyimpanan logo"})
		return
	}
	defer tx.Rollback(ctx)

	var rawConfig []byte
	if err := tx.QueryRow(ctx, `
		SELECT letterhead_config
		FROM companies
		WHERE id = $1
		FOR UPDATE`, companyID).Scan(&rawConfig); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "perusahaan tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca konfigurasi perusahaan"})
		return
	}
	oldConfig, err := parseCompanyLetterheadConfig(rawConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "konfigurasi kop perusahaan tidak valid"})
		return
	}

	objectName := fmt.Sprintf("companies/%s/letterhead/logo-%s.%s", companyID, randomHex(8), extension)
	logo.StorageKey = objectName
	if _, err := h.Minio.PutObject(ctx, h.Bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: logo.MIMEType,
	}); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gagal mengunggah logo ke object storage"})
		return
	}

	logoJSON, err := json.Marshal(logo)
	if err != nil {
		_ = h.Minio.RemoveObject(ctx, h.Bucket, objectName, minio.RemoveObjectOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyiapkan metadata logo"})
		return
	}
	if _, err := tx.Exec(ctx, `
		UPDATE companies
		SET letterhead_config = jsonb_set(letterhead_config, '{logo}', $2::jsonb, true)
		WHERE id = $1`, companyID, logoJSON); err != nil {
		_ = h.Minio.RemoveObject(ctx, h.Bucket, objectName, minio.RemoveObjectOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan metadata logo"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		_ = h.Minio.RemoveObject(ctx, h.Bucket, objectName, minio.RemoveObjectOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyelesaikan penyimpanan logo"})
		return
	}

	if oldConfig.Logo != nil && oldConfig.Logo.StorageKey != "" && oldConfig.Logo.StorageKey != objectName {
		if err := h.Minio.RemoveObject(ctx, h.Bucket, oldConfig.Logo.StorageKey, minio.RemoveObjectOptions{}); err != nil {
			log.Printf("hapus logo lama perusahaan %s gagal: %v", companyID, err)
		}
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "company", &companyID, "upload_logo", &actor, map[string]any{
		"file_name": logo.FileName, "mime_type": logo.MIMEType,
		"width": logo.Width, "height": logo.Height, "checksum_sha256": logo.ChecksumSHA256,
	}, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": companyID})
}

func (h *Handler) DeleteCompanyLogo(c *gin.Context) {
	companyID := strings.TrimSpace(c.Param("id"))
	if !h.requireAdminCompany(c, companyID) {
		return
	}
	ctx := c.Request.Context()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memulai penghapusan logo"})
		return
	}
	defer tx.Rollback(ctx)

	var rawConfig []byte
	if err := tx.QueryRow(ctx, `
		SELECT letterhead_config
		FROM companies
		WHERE id = $1
		FOR UPDATE`, companyID).Scan(&rawConfig); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "perusahaan tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca konfigurasi perusahaan"})
		return
	}
	config, err := parseCompanyLetterheadConfig(rawConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "konfigurasi kop perusahaan tidak valid"})
		return
	}
	if _, err := tx.Exec(ctx, `
		UPDATE companies
		SET letterhead_config = letterhead_config - 'logo'
		WHERE id = $1`, companyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghapus metadata logo"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyelesaikan penghapusan logo"})
		return
	}

	if config.Logo != nil && config.Logo.StorageKey != "" && h.Minio != nil {
		if err := h.Minio.RemoveObject(ctx, h.Bucket, config.Logo.StorageKey, minio.RemoveObjectOptions{}); err != nil {
			log.Printf("hapus objek logo perusahaan %s gagal: %v", companyID, err)
		}
	}

	actor := c.GetString(middleware.CtxUserID)
	h.audit(ctx, "company", &companyID, "delete_logo", &actor, nil, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": companyID})
}

func parseCompanyLetterheadConfig(data []byte) (companyLetterheadConfig, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return companyLetterheadConfig{}, nil
	}
	var config companyLetterheadConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return companyLetterheadConfig{}, err
	}
	return config, nil
}

func readMultipartFile(fileHeader *multipart.FileHeader, maxSize int64) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("gagal membaca file logo")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return nil, errors.New("gagal membaca file logo")
	}
	if int64(len(data)) > maxSize {
		return nil, errors.New("ukuran logo maksimal 2 MB")
	}
	return data, nil
}

func validateCompanyLogo(data []byte) (companyLogoConfig, string, error) {
	contentType := normalizeMIMEType(http.DetectContentType(data))
	extension := ""
	switch contentType {
	case "image/png":
		extension = "png"
	case "image/jpeg":
		extension = "jpg"
	default:
		return companyLogoConfig{}, "", errors.New("logo harus berupa PNG atau JPEG")
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return companyLogoConfig{}, "", errors.New("isi file logo tidak valid")
	}
	if (contentType == "image/png" && format != "png") || (contentType == "image/jpeg" && format != "jpeg") {
		return companyLogoConfig{}, "", errors.New("format isi logo tidak sesuai")
	}
	if config.Width < minCompanyLogoDimension || config.Height < minCompanyLogoDimension {
		return companyLogoConfig{}, "", fmt.Errorf("dimensi logo minimal %dx%d piksel", minCompanyLogoDimension, minCompanyLogoDimension)
	}
	if config.Width > maxCompanyLogoDimension || config.Height > maxCompanyLogoDimension {
		return companyLogoConfig{}, "", fmt.Errorf("dimensi logo maksimal %dx%d piksel", maxCompanyLogoDimension, maxCompanyLogoDimension)
	}

	sum := sha256.Sum256(data)
	return companyLogoConfig{
		MIMEType:       contentType,
		ChecksumSHA256: hex.EncodeToString(sum[:]),
		Width:          config.Width,
		Height:         config.Height,
	}, extension, nil
}
