package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func performRequest(handler func(*gin.Context)) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		handler(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return resp
}

func TestSuccess(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Success(c, gin.H{"key": "value"})
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Code != 0 {
		t.Errorf("Expected code=0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("Expected message='success', got '%s'", resp.Message)
	}
}

func TestBadRequest(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		BadRequest(c, "invalid input")
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Code != 400 {
		t.Errorf("Expected code=400, got %d", resp.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Unauthorized(c, "not logged in")
	})

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestForbidden(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Forbidden(c, "no permission")
	})

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		NotFound(c, "resource not found")
	})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestTooManyRequests(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		TooManyRequests(c, "slow down")
	})

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Code != 429 {
		t.Errorf("Expected code=429, got %d", resp.Code)
	}
}

func TestServerErrorSafe(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		ServerErrorSafe(c, errors.New("database connection failed"))
	})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	// Should NOT expose internal error details
	if resp.Message == "database connection failed" {
		t.Error("ServerErrorSafe should not expose internal error details")
	}
	if resp.Message != "internal server error" {
		t.Errorf("Expected generic message, got '%s'", resp.Message)
	}
}

func TestPaginated(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		items := []string{"a", "b", "c"}
		Paginated(c, items, 100, 1, 10)
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Code != 0 {
		t.Errorf("Expected code=0, got %d", resp.Code)
	}

	// Verify paginated data structure
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}
	if dataMap["total"].(float64) != 100 {
		t.Errorf("Expected total=100, got %v", dataMap["total"])
	}
	if dataMap["page"].(float64) != 1 {
		t.Errorf("Expected page=1, got %v", dataMap["page"])
	}
}

func TestErrorWithErr(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		ErrorWithErr(c, errors.New("validation failed"))
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Message != "validation failed" {
		t.Errorf("Expected 'validation failed', got '%s'", resp.Message)
	}
}

func TestErrorWithErr_CustomStatus(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		ErrorWithErr(c, errors.New("conflict"), http.StatusConflict)
	})

	if w.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d", w.Code)
	}
}
