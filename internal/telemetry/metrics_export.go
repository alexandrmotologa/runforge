package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alexandrmotologa/runforge/internal/durable"
)

// PrometheusHandler returns an HTTP handler serving standard Prometheus metrics.
func PrometheusHandler(ledger *durable.Ledger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := ledger.GetAggregateStats(context.Background())
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to gather stats: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		fmt.Fprintf(w, "# HELP runforge_runs_total Total number of agent runs executed.\n")
		fmt.Fprintf(w, "# TYPE runforge_runs_total counter\n")
		fmt.Fprintf(w, "runforge_runs_total %d\n\n", stats.TotalRuns)

		fmt.Fprintf(w, "# HELP runforge_active_runs Current active or running executions.\n")
		fmt.Fprintf(w, "# TYPE runforge_active_runs gauge\n")
		fmt.Fprintf(w, "runforge_active_runs %d\n\n", stats.ActiveRuns)

		fmt.Fprintf(w, "# HELP runforge_completed_runs Total completed executions.\n")
		fmt.Fprintf(w, "# TYPE runforge_completed_runs counter\n")
		fmt.Fprintf(w, "runforge_completed_runs %d\n\n", stats.CompletedRuns)

		fmt.Fprintf(w, "# HELP runforge_failed_runs Total failed executions.\n")
		fmt.Fprintf(w, "# TYPE runforge_failed_runs counter\n")
		fmt.Fprintf(w, "runforge_failed_runs %d\n\n", stats.FailedRuns)

		fmt.Fprintf(w, "# HELP runforge_tokens_in_total Total prompt tokens ingested.\n")
		fmt.Fprintf(w, "# TYPE runforge_tokens_in_total counter\n")
		fmt.Fprintf(w, "runforge_tokens_in_total %d\n\n", stats.TotalTokensIn)

		fmt.Fprintf(w, "# HELP runforge_tokens_out_total Total completion tokens generated.\n")
		fmt.Fprintf(w, "# TYPE runforge_tokens_out_total counter\n")
		fmt.Fprintf(w, "runforge_tokens_out_total %d\n\n", stats.TotalTokensOut)

		fmt.Fprintf(w, "# HELP runforge_tokens_saved_total Total tokens conserved via compiled deterministic execution.\n")
		fmt.Fprintf(w, "# TYPE runforge_tokens_saved_total counter\n")
		fmt.Fprintf(w, "runforge_tokens_saved_total %d\n\n", stats.TotalTokensSaved)

		fmt.Fprintf(w, "# HELP runforge_cost_usd_total Total USD spent on LLM provider APIs.\n")
		fmt.Fprintf(w, "# TYPE runforge_cost_usd_total counter\n")
		fmt.Fprintf(w, "runforge_cost_usd_total %.4f\n\n", stats.TotalCostUSD)

		fmt.Fprintf(w, "# HELP runforge_cost_saved_usd_total Total USD saved compared to repetitive frontier runs.\n")
		fmt.Fprintf(w, "# TYPE runforge_cost_saved_usd_total counter\n")
		fmt.Fprintf(w, "runforge_cost_saved_usd_total %.4f\n\n", stats.TotalCostSaved)

		fmt.Fprintf(w, "# HELP runforge_savings_percentage Average net token reduction percentage.\n")
		fmt.Fprintf(w, "# TYPE runforge_savings_percentage gauge\n")
		fmt.Fprintf(w, "runforge_savings_percentage %.2f\n", stats.AverageSavingsPct)
	}
}
