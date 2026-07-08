package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Company struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

func (h *Handler) ListCompanies(c *gin.Context) {
	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT id::text, code, name, is_active
		FROM companies
		WHERE is_active
		ORDER BY code`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat perusahaan"})
		return
	}
	defer rows.Close()

	companies := []Company{}
	for rows.Next() {
		var company Company
		if err := rows.Scan(&company.ID, &company.Code, &company.Name, &company.IsActive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca data perusahaan"})
			return
		}
		companies = append(companies, company)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membaca daftar perusahaan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"companies": companies})
}
