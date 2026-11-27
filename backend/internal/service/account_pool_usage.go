package service

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// AccountPoolUsage is the account data visible to signed-in users.
type AccountPoolUsage struct {
	ID                 int64                    `json:"id"`
	Name               string                   `json:"name"`
	Platform           string                   `json:"platform"`
	Type               string                   `json:"type"`
	PlanType           string                   `json:"plan_type,omitempty"`
	PrivacyMode        string                   `json:"privacy_mode,omitempty"`
	ExpiresAt          string                   `json:"expires_at,omitempty"`
	Concurrency        int                      `json:"concurrency"`
	CurrentConcurrency *int                     `json:"current_concurrency,omitempty"`
	Status             string                   `json:"status"`
	StatusCode         int                      `json:"status_code,omitempty"`
	RecoversAt         *time.Time               `json:"recovers_at,omitempty"`
	Priority           int                      `json:"priority"`
	ResetCredits       *AccountPoolResetCredits `json:"reset_credits,omitempty"`
	Windows            []AccountPoolUsageWindow `json:"windows,omitempty"`
}

type AccountPoolResetCredits struct {
	AvailableCount int         `json:"available_count"`
	Expirations    []time.Time `json:"expirations,omitempty"`
}

type AccountPoolUsageWindow struct {
	Kind               string                  `json:"kind"`
	Utilization        float64                 `json:"utilization"`
	UtilizationUnknown bool                    `json:"utilization_unknown,omitempty"`
	ResetsAt           *time.Time              `json:"resets_at,omitempty"`
	WindowStats        *AccountPoolWindowStats `json:"window_stats,omitempty"`
	CostLimit          *AccountPoolCostLimit   `json:"cost_limit,omitempty"`
	start              time.Time
}

type AccountPoolWindowStats struct {
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

type AccountPoolCostLimit struct {
	Used  float64 `json:"used"`
	Limit float64 `json:"limit"`
}

func (s *AccountUsageService) ListPoolUsage(ctx context.Context) ([]AccountPoolUsage, error) {
	accounts, err := s.accountRepo.ListAllWithFilters(ctx, "", "", "", "", 0, "")
	if err != nil {
		return nil, fmt.Errorf("list account pool: %w", err)
	}
	// Match the admin table's ascending priority order and ID tie-breaker.
	slices.SortFunc(accounts, func(a, b Account) int {
		return cmp.Or(cmp.Compare(a.Priority, b.Priority), cmp.Compare(a.ID, b.ID))
	})

	accountsByID := make(map[int64]*Account, len(accounts))
	for i := range accounts {
		accountsByID[accounts[i].ID] = &accounts[i]
	}

	items := make([]AccountPoolUsage, 0, len(accounts))
	now := time.Now()
	for i := range accounts {
		account := &accounts[i]
		if !account.Schedulable {
			continue
		}

		var parent *Account
		if account.ParentAccountID != nil {
			parent = accountsByID[*account.ParentAccountID]
		}
		item := AccountPoolUsage{
			ID:          account.ID,
			Name:        account.Name,
			Platform:    account.Platform,
			Type:        account.Type,
			PlanType:    strings.TrimSpace(account.GetCredential("plan_type")),
			PrivacyMode: strings.TrimSpace(account.GetExtraString("privacy_mode")),
			ExpiresAt:   accountPoolExpiry(account, parent),
			Concurrency: account.Concurrency,
			Priority:    account.Priority,
		}
		if parent != nil {
			item.PlanType = firstNonEmpty(item.PlanType, parent.GetCredential("plan_type"))
			item.PrivacyMode = firstNonEmpty(item.PrivacyMode, parent.GetExtraString("privacy_mode"))
		}
		item.Status, item.StatusCode, item.RecoversAt = accountPoolStatus(account, now)
		if account.IsOpenAIOAuth() {
			item.ResetCredits = readPoolOpenAIResetCredits(account, now)
			if item.ResetCredits == nil && parent != nil {
				item.ResetCredits = readPoolOpenAIResetCredits(parent, now)
			}
		}
		s.addPoolQuotaSnapshot(account, &item, now)
		if account.SupportsAccountQuota() {
			item.addLocalQuotaWindows(account, now)
		}
		items = append(items, item)
	}
	s.populatePoolWindowStats(ctx, items)
	return items, nil
}

func accountPoolExpiry(account, parent *Account) string {
	for _, source := range []*Account{account, parent} {
		if source == nil {
			continue
		}
		if expires := source.GetCredentialAsTime("subscription_expires_at"); expires != nil && expires.Year() >= 1 && expires.Year() <= 9999 {
			return expires.Format(time.RFC3339Nano)
		}
		date := strings.TrimSpace(source.GetCredential("subscription_expires_at"))
		if _, err := time.Parse(time.DateOnly, date); err == nil {
			return date
		}
	}
	if account.ExpiresAt != nil {
		return account.ExpiresAt.Format(time.RFC3339Nano)
	}
	return ""
}

var poolHTTPStatusPattern = regexp.MustCompile(`(?i)\(([1-5]\d{2})\)|\b(?:HTTP(?:/\d(?:\.\d)?)?|status(?:[_\s-]*code)?|(?:error[_\s-]*)?code|OAuth)\s*[:=]?\s*([1-5]\d{2})\b`)

func accountPoolHTTPStatus(message string) int {
	match := poolHTTPStatusPattern.FindStringSubmatch(message)
	if match == nil {
		return 0
	}
	code, _ := strconv.Atoi(firstNonEmpty(match[1], match[2]))
	return code
}

func accountPoolStatus(account *Account, now time.Time) (string, int, *time.Time) {
	cooldown := account.TempUnschedulableUntil
	if cooldown != nil && !cooldown.After(now) {
		cooldown = nil
	}
	switch {
	case account.RateLimitResetAt != nil && account.RateLimitResetAt.After(now):
		return "rateLimited", 429, account.RateLimitResetAt
	case account.OverloadUntil != nil && account.OverloadUntil.After(now):
		return "overloaded", 529, account.OverloadUntil
	case account.Status == StatusError:
		return "error", accountPoolHTTPStatus(account.ErrorMessage), cooldown
	case cooldown != nil:
		var state TempUnschedState
		if err := json.Unmarshal([]byte(account.TempUnschedulableReason), &state); err == nil &&
			state.StatusCode >= 100 && state.StatusCode <= 599 {
			return "tempUnschedulable", state.StatusCode, cooldown
		}
		return "tempUnschedulable", accountPoolHTTPStatus(account.TempUnschedulableReason), cooldown
	case account.Status == StatusExpired, account.AutoPauseOnExpired && account.ExpiresAt != nil && !account.ExpiresAt.After(now):
		return "expired", 0, nil
	case !account.IsActive():
		return "inactive", 0, nil
	case account.SupportsAccountQuota() && account.IsQuotaExceeded():
		return "quotaExceeded", 0, nil
	default:
		return "active", 0, nil
	}
}

func readPoolOpenAIResetCredits(account *Account, now time.Time) *AccountPoolResetCredits {
	cached := account.Extra[openaiQuotaResetCreditsKey]
	if cached == nil {
		return nil
	}
	raw, err := json.Marshal(cached)
	if err != nil {
		return nil
	}
	var snapshot struct {
		AvailableCount *int                               `json:"available_count"`
		Credits        []OpenAIRateLimitResetCreditDetail `json:"credits"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil || snapshot.AvailableCount == nil {
		return nil
	}
	if *snapshot.AvailableCount <= 0 {
		return &AccountPoolResetCredits{}
	}
	var expirations []time.Time
	for _, credit := range snapshot.Credits {
		if expiry, err := parseTime(strings.TrimSpace(credit.ExpiresAt)); err == nil && expiry.After(now) {
			expirations = append(expirations, expiry)
		}
	}
	if len(expirations) == 0 {
		return nil
	}
	slices.SortFunc(expirations, time.Time.Compare)
	count := min(*snapshot.AvailableCount, len(expirations))
	return &AccountPoolResetCredits{AvailableCount: count, Expirations: expirations[:count]}
}

// Read observations already stored by upstream traffic and the quota services.
// The admin usage methods may probe upstream and persist account changes.
func (s *AccountUsageService) addPoolQuotaSnapshot(account *Account, item *AccountPoolUsage, now time.Time) {
	switch {
	case account.IsOpenAIOAuth():
		var usage UsageInfo
		applyExtraToUsage(&usage, account.Extra, now)
		if usesOpenAIMonthlyUsageWindow(account) {
			item.addOpenAIWindow("thirtyDay", usage.SevenDay, openAIMonthlyWindowDuration(account), now)
		} else {
			item.addOpenAIWindow("fiveHour", usage.FiveHour, 5*time.Hour, now)
			item.addOpenAIWindow("sevenDay", usage.SevenDay, 7*24*time.Hour, now)
		}
	case supportsAnthropicPassiveUsage(account):
		usage := s.buildAnthropicPassiveUsage(account)
		if account.SessionWindowEnd != nil {
			item.addProgress("fiveHour", usage.FiveHour, activePoolWindowStart(usage.FiveHour, 5*time.Hour, now))
		}
		item.addProgress("sevenDay", usage.SevenDay, activePoolWindowStart(usage.SevenDay, 7*24*time.Hour, now))
		item.addProgress("sevenDayFable", usage.SevenDayFable, time.Time{})
	case account.IsGrokOAuth():
		if billing, _ := grokBillingSnapshotFromExtra(account.Extra); billing != nil {
			var usage UsageInfo
			applyGrokBillingProgressWindows(&usage, billing, now)
			weeklyStart, _ := currentGrokBillingWindow(billing, true, now)
			monthlyStart, _ := currentGrokBillingWindow(billing, false, now)
			item.addProgress("sevenDay", usage.SevenDay, weeklyStart)
			item.addProgress("thirtyDay", usage.ThirtyDay, monthlyStart)
		}
	default:
		item.addProviderQuotaSnapshot(account, now)
	}
}

func (item *AccountPoolUsage) addProviderQuotaSnapshot(account *Account, now time.Time) {
	if snapshot := decodeOpenCodeGoUsageSnapshot(account.Extra); snapshot != nil && snapshot.Data != nil {
		item.addObservedWindow("fiveHour", snapshot.Data.Rolling.Percent, poolUsageTime(snapshot.Data.Rolling.ResetsAt), 5*time.Hour, now)
		item.addObservedWindow("sevenDay", snapshot.Data.Weekly.Percent, poolUsageTime(snapshot.Data.Weekly.ResetsAt), 7*24*time.Hour, now)
		item.addObservedWindow("thirtyDay", snapshot.Data.Monthly.Percent, poolUsageTime(snapshot.Data.Monthly.ResetsAt), 0, now)
		return
	}
	if snapshot := decodeOllamaCloudUsageSnapshot(account.Extra); snapshot != nil && snapshot.Data != nil {
		if window := snapshot.Data.FiveHour; window != nil {
			item.addObservedWindow("fiveHour", window.UsedPercent, window.ResetAt, 5*time.Hour, now)
		}
		if window := snapshot.Data.SevenDay; window != nil {
			item.addObservedWindow("sevenDay", window.UsedPercent, window.ResetAt, 7*24*time.Hour, now)
		}
		return
	}
	if provider := account.GetCodingPlanProvider(); provider != "" {
		for _, window := range []struct {
			kind, used, reset string
			duration          time.Duration
		}{
			{"fiveHour", cnExtraSuffix5hUsed, cnExtraSuffix5hReset, 5 * time.Hour},
			{"sevenDay", cnExtraSuffixWeeklyUsed, cnExtraSuffixWeeklyReset, 7 * 24 * time.Hour},
			{"thirtyDay", cnExtraSuffixMonthlyUsed, cnExtraSuffixMonthlyReset, 0},
		} {
			if used := account.Extra[cnExtraKey(provider, window.used)]; used != nil {
				resetAt := poolUsageTime(account.getExtraTime(cnExtraKey(provider, window.reset)))
				item.addObservedWindow(window.kind, parseExtraFloat64(used), resetAt, window.duration, now)
			}
		}
	}
}

func (item *AccountPoolUsage) addLocalQuotaWindows(account *Account, now time.Time) {
	for _, period := range []struct {
		kind, key, resetMode string
		used, limit          float64
		duration             time.Duration
		expired              func() bool
	}{
		{"localDaily", "quota_daily", account.GetQuotaDailyResetMode(), account.GetQuotaDailyUsed(), account.GetQuotaDailyLimit(), 24 * time.Hour, account.IsDailyQuotaPeriodExpired},
		{"localWeekly", "quota_weekly", account.GetQuotaWeeklyResetMode(), account.GetQuotaWeeklyUsed(), account.GetQuotaWeeklyLimit(), 7 * 24 * time.Hour, account.IsWeeklyQuotaPeriodExpired},
	} {
		if period.limit <= 0 {
			continue
		}
		startAt := account.getExtraTime(period.key + "_start")
		var resetAt time.Time
		if period.resetMode == "fixed" {
			resetAt = account.getExtraTime(period.key + "_reset_at")
		} else if !startAt.IsZero() {
			resetAt = startAt.Add(period.duration)
		}
		if period.expired() {
			period.used = 0
			startAt = time.Time{}
		}
		if !resetAt.After(now) {
			resetAt = time.Time{}
		}
		item.addCostLimit(period.kind, period.used, period.limit, poolUsageTime(resetAt), startAt)
	}
	if limit := account.GetQuotaLimit(); limit > 0 {
		item.addCostLimit("localTotal", account.GetQuotaUsed(), limit, nil, time.Time{})
	}
}

func (item *AccountPoolUsage) addCostLimit(kind string, used, limit float64, resetAt *time.Time, start time.Time) {
	if math.IsInf(limit, 0) || math.IsInf(used, 0) || math.IsNaN(used) {
		return
	}
	item.addWindow(AccountPoolUsageWindow{
		Kind: kind, Utilization: used / limit * 100, ResetsAt: resetAt, start: start,
		CostLimit: &AccountPoolCostLimit{Used: used, Limit: limit},
	})
}

func activePoolWindowStart(progress *UsageProgress, duration time.Duration, now time.Time) time.Time {
	if duration > 0 && progress != nil && progress.ResetsAt != nil && progress.ResetsAt.After(now) {
		if start := progress.ResetsAt.Add(-duration); !start.After(now) {
			return start
		}
	}
	return time.Time{}
}

func (item *AccountPoolUsage) addObservedWindow(kind string, percent float64, resetAt *time.Time, duration time.Duration, now time.Time) {
	progress := &UsageProgress{Utilization: percent, ResetsAt: resetAt}
	item.addProgress(kind, progress, activePoolWindowStart(progress, duration, now))
}

func (item *AccountPoolUsage) addOpenAIWindow(kind string, progress *UsageProgress, duration time.Duration, now time.Time) {
	if progress == nil {
		progress = &UsageProgress{UtilizationUnknown: true}
	}
	item.addProgress(kind, progress, codexWindowStatsStart(progress, duration, now))
}

func (item *AccountPoolUsage) addProgress(kind string, progress *UsageProgress, start time.Time) {
	if progress != nil {
		item.addWindow(AccountPoolUsageWindow{
			Kind: kind, Utilization: progress.Utilization, UtilizationUnknown: progress.UtilizationUnknown,
			ResetsAt: progress.ResetsAt, start: start,
		})
	}
}

func (item *AccountPoolUsage) addWindow(window AccountPoolUsageWindow) {
	if window.Utilization < 0 || math.IsNaN(window.Utilization) || math.IsInf(window.Utilization, 0) {
		return
	}
	item.Windows = append(item.Windows, window)
}

type accountWindowStatsByStartReader interface {
	GetAccountWindowStatsByStarts(context.Context, []usagestats.AccountWindowStart) ([]usagestats.AccountStats, error)
}

func (s *AccountUsageService) populatePoolWindowStats(ctx context.Context, items []AccountPoolUsage) {
	reader, ok := s.usageLogRepo.(accountWindowStatsByStartReader)
	if !ok {
		return
	}
	var starts []usagestats.AccountWindowStart
	var windows []*AccountPoolUsageWindow
	for _, item := range items {
		for i := range item.Windows {
			window := &item.Windows[i]
			if !window.start.IsZero() {
				starts = append(starts, usagestats.AccountWindowStart{AccountID: item.ID, Start: window.start})
				windows = append(windows, window)
			}
		}
	}
	if len(starts) == 0 {
		return
	}
	stats, err := reader.GetAccountWindowStatsByStarts(ctx, starts)
	if err != nil {
		slog.Warn("account_pool_window_stats_failed", "error", err)
		return
	}
	if len(stats) != len(windows) {
		slog.Warn("account_pool_window_stats_incomplete", "got", len(stats), "expected", len(windows))
		return
	}
	for i, stat := range stats {
		if !math.IsNaN(stat.Cost) && !math.IsInf(stat.Cost, 0) {
			windows[i].WindowStats = &AccountPoolWindowStats{Requests: stat.Requests, Tokens: stat.Tokens, Cost: stat.Cost}
		}
	}
}

func poolUsageTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
