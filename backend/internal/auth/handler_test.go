package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/testutil"
)

func TestRegister(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)

	body := `{"email":"test@example.com","display_name":"Test User","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp auth.AuthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.User.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", resp.User.Email)
	}
	if resp.Tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.Tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)

	body := `{"email":"dup@example.com","display_name":"User","password":"password123"}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("first register failed: %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("expected status 409 for duplicate, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestLogin(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)

	// Register first
	regBody := `{"email":"login@example.com","display_name":"Login User","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: %d", w.Code)
	}

	// Login
	loginBody := `{"email":"login@example.com","password":"password123"}`
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp auth.AuthResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)

	regBody := `{"email":"wrong@example.com","display_name":"Wrong","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	loginBody := `{"email":"wrong@example.com","password":"wrongpassword"}`
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w2.Code)
	}
}

func TestRefreshToken(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)

	// Register
	regBody := `{"email":"refresh@example.com","display_name":"Refresh","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var regResp auth.AuthResponse
	json.Unmarshal(w.Body.Bytes(), &regResp)

	// Refresh
	refreshBody, _ := json.Marshal(map[string]string{"refresh_token": regResp.Tokens.RefreshToken})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBuffer(refreshBody))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var tokens auth.TokenPair
	if err := json.Unmarshal(w2.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("expected non-empty new access token")
	}

	// Old refresh token should be invalid (rotation)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBuffer(refreshBody))
	req3.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusUnauthorized {
		t.Errorf("expected old refresh token to be invalid (401), got %d", w3.Code)
	}
}

func TestLogout(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)

	// Register
	regBody := `{"email":"logout@example.com","display_name":"Logout","password":"password123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var regResp auth.AuthResponse
	json.Unmarshal(w.Body.Bytes(), &regResp)

	// Logout
	logoutBody, _ := json.Marshal(map[string]string{"refresh_token": regResp.Tokens.RefreshToken})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/logout", bytes.NewBuffer(logoutBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+regResp.Tokens.AccessToken)
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}
}
