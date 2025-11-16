package main

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CronWorkflow executes registered queries on a schedule
// This workflow replaces the dixcron binary functionality using Temporal's native cron support
func CronWorkflow(ctx workflow.Context, config CronWorkflowConfig) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("CronWorkflow started", "queries", len(config.RegisteredQueries))

	// Configure activity options with retries
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute, // Long timeout for expensive queries
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Determine if this is an hourly or daily execution
	// This is determined by the cron schedule used to trigger the workflow
	cronSchedule := workflow.GetInfo(ctx).CronSchedule
	isHourly := cronSchedule == config.HourlyCronSchedule

	logger.Info("Determining execution type",
		"cronSchedule", cronSchedule,
		"isHourly", isHourly)

	// Step 1: Get all indexed chains from database
	var chains []ChainInfo
	err := workflow.ExecuteActivity(ctx, "GetDatabaseInfoActivity").Get(ctx, &chains)
	if err != nil {
		logger.Error("Failed to get database info", "error", err)
		return fmt.Errorf("failed to get database info: %w", err)
	}

	logger.Info("Found indexed chains", "count", len(chains))

	// Step 2: Get current time for determining which months to process
	currentTime := workflow.Now(ctx)
	currentYear := currentTime.Year()
	currentMonth := int(currentTime.Month())

	if isHourly {
		// Hourly execution: only compute current month's block count
		err = executeCurrentMonthQueries(ctx, chains, currentYear, currentMonth, logger)
	} else {
		// Daily execution: compute all registered queries for all past months
		err = executeHistoricalQueries(ctx, config, chains, currentYear, currentMonth, logger)
	}

	if err != nil {
		logger.Error("Query execution failed", "error", err)
		return err
	}

	logger.Info("CronWorkflow completed successfully")
	return nil
}

// executeCurrentMonthQueries executes block count query for current month (hourly)
func executeCurrentMonthQueries(ctx workflow.Context, chains []ChainInfo,
	year, month int, logger log.Logger) error {

	logger.Info("Executing current month queries", "year", year, "month", month)

	for _, chain := range chains {
		logger.Info("Processing chain for current month",
			"relayChain", chain.RelayChain,
			"chain", chain.Chain)

		// Execute the total_blocks_in_month query for current month
		err := workflow.ExecuteActivity(ctx, "ExecuteAndStoreNamedQueryActivity",
			chain.RelayChain, chain.Chain, "total_blocks_in_month", year, month).Get(ctx, nil)

		if err != nil {
			logger.Error("Failed to execute current month query",
				"relayChain", chain.RelayChain,
				"chain", chain.Chain,
				"error", err)
			// Continue with other chains even if one fails
			continue
		}

		logger.Info("Current month query completed",
			"relayChain", chain.RelayChain,
			"chain", chain.Chain)
	}

	return nil
}

// executeHistoricalQueries executes all registered queries for all past months (daily)
func executeHistoricalQueries(ctx workflow.Context, config CronWorkflowConfig,
	chains []ChainInfo, currentYear, currentMonth int, logger log.Logger) error {

	logger.Info("Executing historical queries",
		"queries", len(config.RegisteredQueries),
		"chains", len(chains))

	// Process from 2019 (first year) to current month
	firstYear := 2019

	for _, chain := range chains {
		for _, queryName := range config.RegisteredQueries {
			// Process each year/month combination
			for year := firstYear; year <= currentYear; year++ {
				startMonth := 1
				endMonth := 12

				// For current year, only process up to current month
				if year == currentYear {
					endMonth = currentMonth
				}

				for month := startMonth; month <= endMonth; month++ {
					// Skip current/future months for registered queries
					if year == currentYear && month >= currentMonth {
						continue
					}

					logger.Info("Processing query",
						"query", queryName,
						"relayChain", chain.RelayChain,
						"chain", chain.Chain,
						"year", year,
						"month", month)

					// Check if result already exists
					var exists bool
					err := workflow.ExecuteActivity(ctx, "CheckQueryResultExistsActivity",
						chain.RelayChain, chain.Chain, queryName, year, month).Get(ctx, &exists)

					if err != nil {
						logger.Error("Failed to check query result existence",
							"error", err)
						continue
					}

					if exists {
						logger.Info("Query result already exists, skipping",
							"query", queryName,
							"year", year,
							"month", month)
						continue
					}

					// Execute and store the query
					err = workflow.ExecuteActivity(ctx, "ExecuteAndStoreNamedQueryActivity",
						chain.RelayChain, chain.Chain, queryName, year, month).Get(ctx, nil)

					if err != nil {
						logger.Error("Failed to execute query",
							"query", queryName,
							"relayChain", chain.RelayChain,
							"chain", chain.Chain,
							"year", year,
							"month", month,
							"error", err)
						// Continue with other queries even if one fails
						continue
					}

					logger.Info("Query executed successfully",
						"query", queryName,
						"relayChain", chain.RelayChain,
						"chain", chain.Chain,
						"year", year,
						"month", month)
				}
			}
		}
	}

	return nil
}

// ChainInfo represents information about an indexed chain
type ChainInfo struct {
	RelayChain string
	Chain      string
}
