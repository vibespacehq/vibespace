package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRequestDeviceCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login/device/code" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing Accept: application/json header")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("client_id") != "test-client-id" {
			t.Errorf("unexpected client_id: %s", r.FormValue("client_id"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "dc-123",
			UserCode:        "ABCD-1234",
			VerificationURI: "https://github.com/login/device",
			ExpiresIn:       900,
			Interval:        5,
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	resp, err := RequestDeviceCode(context.Background(), "test-client-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeviceCode != "dc-123" {
		t.Errorf("got device code %q, want %q", resp.DeviceCode, "dc-123")
	}
	if resp.UserCode != "ABCD-1234" {
		t.Errorf("got user code %q, want %q", resp.UserCode, "ABCD-1234")
	}
	if resp.Interval != 5 {
		t.Errorf("got interval %d, want 5", resp.Interval)
	}
}

func TestRequestDeviceCode_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad_request"}`))
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	_, err := RequestDeviceCode(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPollForToken_ImmediateSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "ghu_test123",
			RefreshToken: "ghr_test456",
			TokenType:    "bearer",
			ExpiresIn:    28800,
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token, err := PollForToken(ctx, "test-id", "dc-123", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "ghu_test123" {
		t.Errorf("got access token %q, want %q", token.AccessToken, "ghu_test123")
	}
	if token.RefreshToken != "ghr_test456" {
		t.Errorf("got refresh token %q, want %q", token.RefreshToken, "ghr_test456")
	}
}

func TestPollForToken_PendingThenSuccess(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := calls.Add(1)
		if n < 3 {
			json.NewEncoder(w).Encode(TokenResponse{
				Error: "authorization_pending",
			})
			return
		}
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "ghu_success",
			RefreshToken: "ghr_success",
			TokenType:    "bearer",
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := PollForToken(ctx, "test-id", "dc-123", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "ghu_success" {
		t.Errorf("got %q, want %q", token.AccessToken, "ghu_success")
	}
	if calls.Load() < 3 {
		t.Errorf("expected at least 3 poll attempts, got %d", calls.Load())
	}
}

func TestPollForToken_AccessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			Error:            "access_denied",
			ErrorDescription: "user denied",
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := PollForToken(ctx, "test-id", "dc-123", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPollForToken_ExpiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			Error: "expired_token",
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := PollForToken(ctx, "test-id", "dc-123", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPollForToken_SlowDown(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := calls.Add(1)
		if n == 1 {
			json.NewEncoder(w).Encode(TokenResponse{
				Error: "slow_down",
			})
			return
		}
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "ghu_slowed",
			RefreshToken: "ghr_slowed",
			TokenType:    "bearer",
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	token, err := PollForToken(ctx, "test-id", "dc-123", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "ghu_slowed" {
		t.Errorf("got %q, want %q", token.AccessToken, "ghu_slowed")
	}
}

func TestRefreshAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login/oauth/access_token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("unexpected grant_type: %s", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "ghr_old" {
			t.Errorf("unexpected refresh_token: %s", r.FormValue("refresh_token"))
		}
		if r.FormValue("client_id") != "test-id" {
			t.Errorf("unexpected client_id: %s", r.FormValue("client_id"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "ghu_new",
			RefreshToken: "ghr_new",
			TokenType:    "bearer",
			ExpiresIn:    28800,
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	token, err := RefreshAccessToken(context.Background(), "test-id", "ghr_old")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "ghu_new" {
		t.Errorf("got %q, want %q", token.AccessToken, "ghu_new")
	}
	if token.RefreshToken != "ghr_new" {
		t.Errorf("got %q, want %q", token.RefreshToken, "ghr_new")
	}
}

func TestRefreshAccessToken_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			Error:            "bad_refresh_token",
			ErrorDescription: "token expired",
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	_, err := RefreshAccessToken(context.Background(), "test-id", "ghr_expired")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPollForToken_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			Error: "authorization_pending",
		})
	}))
	defer srv.Close()

	origBase := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = origBase }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := PollForToken(ctx, "test-id", "dc-123", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
