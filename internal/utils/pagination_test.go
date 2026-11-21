package utils

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newContext(query string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/test?"+query, nil)
	return c
}

func TestParsePagination_Defaults(t *testing.T) {
	c := newContext("")
	page, pageSize, offset := ParsePagination(c, 20)

	if page != 1 {
		t.Errorf("Expected page=1, got %d", page)
	}
	if pageSize != 20 {
		t.Errorf("Expected pageSize=20, got %d", pageSize)
	}
	if offset != 0 {
		t.Errorf("Expected offset=0, got %d", offset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	c := newContext("page=3&page_size=10")
	page, pageSize, offset := ParsePagination(c, 20)

	if page != 3 {
		t.Errorf("Expected page=3, got %d", page)
	}
	if pageSize != 10 {
		t.Errorf("Expected pageSize=10, got %d", pageSize)
	}
	if offset != 20 {
		t.Errorf("Expected offset=20, got %d", offset)
	}
}

func TestParsePagination_NegativeValues(t *testing.T) {
	c := newContext("page=-1&page_size=-5")
	page, pageSize, offset := ParsePagination(c, 15)

	if page != 1 {
		t.Errorf("Expected page=1 (normalized), got %d", page)
	}
	if pageSize != 15 {
		t.Errorf("Expected pageSize=15 (default), got %d", pageSize)
	}
	if offset != 0 {
		t.Errorf("Expected offset=0, got %d", offset)
	}
}

func TestParsePagination_ZeroValues(t *testing.T) {
	c := newContext("page=0&page_size=0")
	page, pageSize, offset := ParsePagination(c, 25)

	if page != 1 {
		t.Errorf("Expected page=1, got %d", page)
	}
	if pageSize != 25 {
		t.Errorf("Expected pageSize=25 (default), got %d", pageSize)
	}
	if offset != 0 {
		t.Errorf("Expected offset=0, got %d", offset)
	}
}

func TestParsePagination_InvalidStrings(t *testing.T) {
	c := newContext("page=abc&page_size=xyz")
	page, pageSize, offset := ParsePagination(c, 10)

	if page != 1 {
		t.Errorf("Expected page=1 (fallback), got %d", page)
	}
	if pageSize != 10 {
		t.Errorf("Expected pageSize=10 (default), got %d", pageSize)
	}
	if offset != 0 {
		t.Errorf("Expected offset=0, got %d", offset)
	}
}

func TestParsePagination_OffsetCalculation(t *testing.T) {
	tests := []struct {
		page, pageSize, expectedOffset int
	}{
		{1, 10, 0},
		{2, 10, 10},
		{5, 20, 80},
		{10, 5, 45},
	}

	for _, tt := range tests {
		c := newContext("")
		// Manually set query values
		c.Request.URL.RawQuery = ""
		c.Request = c.Request.Clone(c.Request.Context())
		q := c.Request.URL.Query()
		q.Set("page", strconv.Itoa(tt.page))
		q.Set("page_size", strconv.Itoa(tt.pageSize))
		c.Request.URL.RawQuery = q.Encode()

		page, pageSize, offset := ParsePagination(c, 20)
		if page != tt.page || pageSize != tt.pageSize || offset != tt.expectedOffset {
			t.Errorf("ParsePagination(page=%d, pageSize=%d): got page=%d, pageSize=%d, offset=%d; want offset=%d",
				tt.page, tt.pageSize, page, pageSize, offset, tt.expectedOffset)
		}
	}
}
