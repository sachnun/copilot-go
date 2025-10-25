package app

import (
	"context"
	"fmt"
	"math"
	"net/http"

	"internal/logger"
	"internal/paths"
	"internal/services/github"
	"internal/state"
	"internal/token"
)

func RunCheckUsage(ctx context.Context) error {
	if err := paths.EnsurePaths(paths.Default); err != nil {
		return err
	}

	if err := token.SetupGitHubToken(ctx, state.Shared, paths.Default, token.SetupGitHubTokenOptions{}, http.DefaultClient); err != nil {
		return err
	}

	usage, err := github.GetCopilotUsage(ctx, state.Shared, http.DefaultClient)
	if err != nil {
		return err
	}

	premium := usage.QuotaSnapshots.PremiumInteractions
	premiumTotal := premium.Entitlement
	premiumUsed := premiumTotal - premium.Remaining
	premiumPercentUsed := percentage(premiumUsed, premiumTotal)
	premiumPercentRemaining := premium.PercentRemaining

	logger.Info("Copilot Usage (plan: %s)", usage.CopilotPlan)
	logger.Info("Quota resets: %s", usage.QuotaResetDate)
	logger.Info("\nQuotas:")
	logger.Info(
		"  Premium: %s/%s used (%.1f%% used, %.1f%% remaining)",
		formatCount(premiumUsed),
		formatCount(premiumTotal),
		premiumPercentUsed,
		premiumPercentRemaining,
	)
	logQuota("Chat", usage.QuotaSnapshots.Chat)
	logQuota("Completions", usage.QuotaSnapshots.Completions)

	return nil
}

func logQuota(name string, detail github.QuotaDetail) {
	total := detail.Entitlement
	used := total - detail.Remaining
	logger.Info(
		"  %s: %s/%s used (%.1f%% used, %.1f%% remaining)",
		name,
		formatCount(used),
		formatCount(total),
		percentage(used, total),
		detail.PercentRemaining,
	)
}

func percentage(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return (value / total) * 100
}

func formatCount(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "-"
	}
	if math.Abs(value-math.Round(value)) < 1e-6 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}
