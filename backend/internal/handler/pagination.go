package handler

import "strconv"

const (
	paginationDefaultPage     = 1
	paginationDefaultPageSize = 20
	paginationMaxPageSize     = 100
)

// pageMeta menyertai setiap respons list yang dipaginasi server-side.
type pageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func newPageMeta(page, pageSize int, total int64) pageMeta {
	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}
	return pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
}

// parsePagination membaca query param `page` dan `page_size`, dengan default
// dan batas atas, lalu mengembalikan offset SQL siap pakai.
func parsePagination(pageRaw, pageSizeRaw string) (page, pageSize, offset int, ok bool) {
	page = paginationDefaultPage
	if pageRaw != "" {
		parsed, err := strconv.Atoi(pageRaw)
		if err != nil || parsed < 1 {
			return 0, 0, 0, false
		}
		page = parsed
	}

	pageSize = paginationDefaultPageSize
	if pageSizeRaw != "" {
		parsed, err := strconv.Atoi(pageSizeRaw)
		if err != nil || parsed < 1 {
			return 0, 0, 0, false
		}
		pageSize = min(parsed, paginationMaxPageSize)
	}

	offset = (page - 1) * pageSize
	return page, pageSize, offset, true
}
