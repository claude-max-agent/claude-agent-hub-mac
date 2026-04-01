package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleUpgradeRejectsReloadForNonTmux(t *testing.T) {
	h := NewUpgradeHandler(".")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upgrade", bytes.NewBufferString(`{"layer":"api","reload":true}`))
	w := httptest.NewRecorder()

	h.HandleUpgrade(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
