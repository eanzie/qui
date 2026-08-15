// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/auth"
	"github.com/autobrr/qui/internal/database"
	"github.com/autobrr/qui/internal/domain"
)

// failDeleteStore is an scs.Store that always fails on Delete.
// Find returns valid session data so that RenewToken sees a non-empty token.
type failDeleteStore struct {
	sessionData []byte
}

func newFailDeleteStore() *failDeleteStore {
	// Encode minimal session data with a future deadline.
	data, err := scs.GobCodec{}.Encode(
		time.Now().Add(time.Hour),
		map[string]any{},
	)
	if err != nil {
		panic("failed to encode test session data: " + err.Error())
	}
	return &failDeleteStore{sessionData: data}
}

func (s *failDeleteStore) Find(token string) ([]byte, bool, error) {
	return s.sessionData, true, nil
}

func (s *failDeleteStore) Delete(_ string) error {
	return errors.New("simulated store failure")
}

func (s *failDeleteStore) Commit(_ string, _ []byte, _ time.Time) error {
	return nil
}

func TestSetupReturns500WhenRenewTokenFails(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	authService := auth.NewService(db)
	sm := scs.New()
	sm.Store = newFailDeleteStore()

	handler := &AuthHandler{
		authService:    authService,
		sessionManager: sm,
		config:         &domain.Config{},
	}

	body := `{"username":"alice","password":"password1234"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "session=existing-token")

	resp := httptest.NewRecorder()

	// Wrap in LoadAndSave so SCS injects session context with a non-empty token.
	sm.LoadAndSave(http.HandlerFunc(handler.Setup)).ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	assert.Contains(t, respBody["error"], "Authentication failed")
}

func TestLoginReturns500WhenRenewTokenFails(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	authService := auth.NewService(db)

	// Create a user first so Login can succeed up to RenewToken.
	_, err := authService.SetupUser(t.Context(), "bob", "securepass123")
	require.NoError(t, err)

	sm := scs.New()
	sm.Store = newFailDeleteStore()

	handler := &AuthHandler{
		authService:    authService,
		sessionManager: sm,
		config:         &domain.Config{},
	}

	body := `{"username":"bob","password":"securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "session=existing-token")

	resp := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(handler.Login)).ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	assert.Contains(t, respBody["error"], "Authentication failed")
}

func TestLoginDoesNotSetAuthenticatedWhenRenewTokenFails(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	authService := auth.NewService(db)
	_, err := authService.SetupUser(t.Context(), "carol", "securepass123")
	require.NoError(t, err)

	sm := scs.New()
	sm.Store = newFailDeleteStore()

	handler := &AuthHandler{
		authService:    authService,
		sessionManager: sm,
		config:         &domain.Config{},
	}

	body := `{"username":"carol","password":"securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "session=existing-token")

	resp := httptest.NewRecorder()

	var authenticated bool
	// Use a custom handler to inspect session state after the Login handler runs.
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.Login(w, r)
		authenticated = sm.GetBool(r.Context(), "authenticated")
	})).ServeHTTP(resp, req)

	assert.False(t, authenticated, "authenticated must not be set when RenewToken fails")
}

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.New(dbPath)
	require.NoError(t, err)
	return db, func() { require.NoError(t, db.Close()) }
}
