package routes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountPoolUsageRoute(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	reset := now.Add(time.Hour)
	weeklyReset := now.AddDate(0, 0, 4)
	expiry := now.AddDate(0, 1, 0)
	cardExpiry := now.AddDate(0, 0, 2)
	secondCardExpiry := now.AddDate(0, 0, 3)
	parentID := int64(15)
	accounts := &accountPoolTestAccounts{accounts: []service.Account{
		{
			ID: 2, Name: "Backup", Priority: 20, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Concurrency: 3,
			Status: service.StatusError, Schedulable: true, ErrorMessage: "OAuth 401: private error description",
		},
		{
			ID: parentID, Name: "Paused parent", Priority: 10, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
			Status: service.StatusActive, Schedulable: false,
			Credentials: map[string]any{"plan_type": "plus", "access_token": "parent-secret"},
			Extra:       map[string]any{"privacy_mode": service.PrivacyModeTrainingOff},
		},
		{
			ID: 18, Name: "Overload", Priority: 20, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Concurrency: 2,
			Status: service.StatusActive, Schedulable: true, OverloadUntil: &reset,
		},
		{
			ID: 21, Name: "Cooldown", Priority: 30, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Concurrency: 2,
			Status: service.StatusActive, Schedulable: true, TempUnschedulableUntil: &reset,
			TempUnschedulableReason: `{"status_code":503,"error_message":"private upstream reason"}`,
		},
		{
			ID: 12, Priority: 10, Name: "upstream@example.test",
			Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Concurrency: 2,
			Status: service.StatusActive, Schedulable: true, RateLimitResetAt: &reset,
			ParentAccountID: &parentID,
			Credentials: map[string]any{
				"access_token": "upstream-secret", "plan_type": "team", "subscription_expires_at": expiry.Format(time.RFC3339),
			},
			Extra: map[string]any{
				"codex_5h_used_percent":  95.0,
				"codex_5h_reset_at":      reset.Format(time.RFC3339),
				"codex_7d_used_percent":  61.0,
				"codex_7d_reset_at":      weeklyReset.Format(time.RFC3339),
				"codex_usage_updated_at": now.Format(time.RFC3339),
				"internal_provider_note": "private",
				"codex_reset_credit_snapshot": map[string]any{
					"available_count": 3,
					"credits": []map[string]any{
						{"id": "credit-private", "expires_at": secondCardExpiry.Format(time.RFC3339)},
						{"expires_at": now.Add(-time.Minute).Format(time.RFC3339)},
						{"expires_at": cardExpiry.Format(time.RFC3339)},
					},
				},
			},
		},
	}}
	capacity := &accountPoolTestConcurrency{}
	stats := &accountPoolTestUsageStats{}
	usage := service.NewAccountUsageService(accounts, stats, nil, nil, nil, nil, nil, nil, service.NewUsageCache(), nil, nil)
	router := gin.New()
	RegisterUserRoutes(router.Group("/api/v1"), &handler.Handlers{
		AccountPoolUsage: handler.NewAccountPoolUsageHandler(usage, service.NewConcurrencyService(capacity)),
	}, middleware.JWTAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "Bearer test-user" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
		}
		c.Next()
	}), middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)

	request := func(method, query string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/usage/account-pool"+query, nil)
		if authenticated {
			req.Header.Set("Authorization", "Bearer test-user")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "", false).Code)
	require.Equal(t, http.StatusNotFound, request(http.MethodPost, "", true).Code)
	require.Zero(t, accounts.calls)

	scheduledAccounts := request(http.MethodGet, "?page=2&page_size=1&sort_by=name&sort_order=desc", true)
	require.Equal(t, http.StatusOK, scheduledAccounts.Code)
	require.JSONEq(t, fmt.Sprintf(`{
		"code": 0, "message": "success",
		"data": [
			{
				"id": 12, "name": "upstream@example.test", "platform": "openai", "type": "oauth",
				"plan_type": "team", "privacy_mode": "training_off",
				"status": "rateLimited", "status_code": 429, "recovers_at": %q, "expires_at": %q, "priority": 10,
				"concurrency": 2, "current_concurrency": 1,
				"reset_credits": {"available_count": 2, "expirations": [%q, %q]},
				"windows": [
					{"kind": "fiveHour", "utilization": 95, "resets_at": %q,
					 "window_stats": {"requests": 262, "tokens": 28900000, "cost": 18.13}},
					{"kind": "sevenDay", "utilization": 61, "resets_at": %q,
					 "window_stats": {"requests": 805, "tokens": 126400000, "cost": 74.59}}
				]
			},
			{
				"id": 2, "name": "Backup", "platform": "openai", "type": "apikey", "status": "error", "status_code": 401, "priority": 20,
				"concurrency": 3, "current_concurrency": 0
			},
			{
				"id": 18, "name": "Overload", "platform": "anthropic", "type": "apikey", "status": "overloaded", "status_code": 529,
				"recovers_at": %q, "priority": 20, "concurrency": 2
			},
			{
				"id": 21, "name": "Cooldown", "platform": "anthropic", "type": "apikey", "status": "tempUnschedulable", "status_code": 503,
				"recovers_at": %q, "priority": 30, "concurrency": 2
			}
		]
	}`, reset.Format(time.RFC3339), expiry.Format(time.RFC3339),
		cardExpiry.Format(time.RFC3339), secondCardExpiry.Format(time.RFC3339),
		reset.Format(time.RFC3339), weeklyReset.Format(time.RFC3339),
		reset.Format(time.RFC3339), reset.Format(time.RFC3339)), scheduledAccounts.Body.String())
	require.Equal(t, 1, accounts.calls)
	require.Equal(t, []int64{12, 2, 18, 21}, capacity.requested)
	require.Equal(t, []usagestats.AccountWindowStart{
		{AccountID: 12, Start: reset.Add(-5 * time.Hour)},
		{AccountID: 12, Start: weeklyReset.Add(-7 * 24 * time.Hour)},
	}, stats.requested)

	capacity.err = errors.New("concurrency cache unavailable")
	withoutCounts := request(http.MethodGet, "", true)
	require.Equal(t, http.StatusOK, withoutCounts.Code)
	require.Contains(t, withoutCounts.Body.String(), `"concurrency":2`)
	require.NotContains(t, withoutCounts.Body.String(), "current_concurrency")
}

type accountPoolTestAccounts struct {
	service.AccountRepository
	accounts []service.Account
	calls    int
}

func (r *accountPoolTestAccounts) ListAllWithFilters(_ context.Context, _, _, _, _ string, _ int64, _ string) ([]service.Account, error) {
	r.calls++
	return r.accounts, nil
}

type accountPoolTestConcurrency struct {
	service.ConcurrencyCache
	requested []int64
	err       error
}

func (r *accountPoolTestConcurrency) ReadAccountConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	r.requested = ids
	return map[int64]int{12: 1, 2: 0}, r.err
}

type accountPoolTestUsageStats struct {
	service.UsageLogRepository
	requested []usagestats.AccountWindowStart
}

func (r *accountPoolTestUsageStats) GetAccountWindowStatsByStarts(_ context.Context, windows []usagestats.AccountWindowStart) ([]usagestats.AccountStats, error) {
	r.requested = windows
	return []usagestats.AccountStats{
		{Requests: 262, Tokens: 28900000, Cost: 18.13, UserCost: 120},
		{Requests: 805, Tokens: 126400000, Cost: 74.59, UserCost: 551},
	}, nil
}
