package database

import (
	"database/sql"
	"fmt"
	"time"
)

// RequestLog represents a request log entry for tracking API calls
type RequestLog struct {
	ID             int
	ClientID       *string
	Provider       string
	Model          string
	Endpoint       string
	RequestTokens  int
	ResponseTokens int
	LatencyMs      int
	StatusCode     *int
	ErrorMessage   *string
	CreatedAt      time.Time
}

// RequestLogFilter defines filtering options for listing request logs
type RequestLogFilter struct {
	ClientID   *string
	Model      *string
	Provider   *string
	Endpoint   *string
	StatusCode *int
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}

// RequestLogStats holds aggregated statistics for request logs
type RequestLogStats struct {
	TotalRequests    int
	TotalTokens      int64
	RequestTokens    int64
	ResponseTokens   int64
	AvgLatencyMs     float64
	SuccessCount     int
	ErrorCount       int
	SuccessRate      float64
	TokensByProvider map[string]int64
	RequestsByModel  map[string]int
	RequestsByClient map[string]int
}

// CreateRequestLog inserts a new request log entry
func (db *DB) CreateRequestLog(log *RequestLog) error {
	query := `
		INSERT INTO request_logs (client_id, provider, model, endpoint, request_tokens, response_tokens, latency_ms, status_code, error_message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := db.conn.Exec(query,
		log.ClientID, log.Provider, log.Model, log.Endpoint,
		log.RequestTokens, log.ResponseTokens, log.LatencyMs,
		log.StatusCode, log.ErrorMessage, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create request log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	log.ID = int(id)

	return nil
}

// ListRequestLogs retrieves request logs with optional filtering
func (db *DB) ListRequestLogs(filter *RequestLogFilter) ([]*RequestLog, error) {
	query := `SELECT id, client_id, provider, model, endpoint, request_tokens, response_tokens, latency_ms, status_code, error_message, created_at FROM request_logs WHERE 1=1`
	var args []interface{}

	if filter != nil {
		if filter.ClientID != nil {
			query += " AND client_id = ?"
			args = append(args, *filter.ClientID)
		}
		if filter.Model != nil {
			query += " AND model = ?"
			args = append(args, *filter.Model)
		}
		if filter.Provider != nil {
			query += " AND provider = ?"
			args = append(args, *filter.Provider)
		}
		if filter.Endpoint != nil {
			query += " AND endpoint = ?"
			args = append(args, *filter.Endpoint)
		}
		if filter.StatusCode != nil {
			query += " AND status_code = ?"
			args = append(args, *filter.StatusCode)
		}
		if filter.Since != nil {
			query += " AND created_at >= ?"
			args = append(args, *filter.Since)
		}
		if filter.Until != nil {
			query += " AND created_at <= ?"
			args = append(args, *filter.Until)
		}
	}

	query += " ORDER BY created_at DESC"

	if filter != nil && filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list request logs: %w", err)
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		log := &RequestLog{}
		err := rows.Scan(
			&log.ID, &log.ClientID, &log.Provider, &log.Model, &log.Endpoint,
			&log.RequestTokens, &log.ResponseTokens, &log.LatencyMs,
			&log.StatusCode, &log.ErrorMessage, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan request log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating request logs: %w", err)
	}

	return logs, nil
}

// GetRequestLogStats retrieves aggregated statistics for request logs
func (db *DB) GetRequestLogStats(filter *RequestLogFilter) (*RequestLogStats, error) {
	stats := &RequestLogStats{
		TokensByProvider: make(map[string]int64),
		RequestsByModel:  make(map[string]int),
		RequestsByClient: make(map[string]int),
	}

	// Build base WHERE clause
	whereClause := "WHERE 1=1"
	var args []interface{}

	if filter != nil {
		if filter.ClientID != nil {
			whereClause += " AND client_id = ?"
			args = append(args, *filter.ClientID)
		}
		if filter.Model != nil {
			whereClause += " AND model = ?"
			args = append(args, *filter.Model)
		}
		if filter.Provider != nil {
			whereClause += " AND provider = ?"
			args = append(args, *filter.Provider)
		}
		if filter.Endpoint != nil {
			whereClause += " AND endpoint = ?"
			args = append(args, *filter.Endpoint)
		}
		if filter.Since != nil {
			whereClause += " AND created_at >= ?"
			args = append(args, *filter.Since)
		}
		if filter.Until != nil {
			whereClause += " AND created_at <= ?"
			args = append(args, *filter.Until)
		}
	}

	// Get aggregate stats
	aggregateQuery := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(request_tokens + response_tokens), 0) as total_tokens,
			COALESCE(SUM(request_tokens), 0) as request_tokens,
			COALESCE(SUM(response_tokens), 0) as response_tokens,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
			COALESCE(SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END), 0) as success_count,
			COALESCE(SUM(CASE WHEN status_code IS NULL OR status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END), 0) as error_count
		FROM request_logs %s
	`, whereClause)

	var totalRequests int
	var totalTokens, requestTokens, responseTokens int64
	var avgLatencyMs float64
	var successCount, errorCount int

	err := db.conn.QueryRow(aggregateQuery, args...).Scan(
		&totalRequests, &totalTokens, &requestTokens, &responseTokens,
		&avgLatencyMs, &successCount, &errorCount,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get aggregate stats: %w", err)
	}

	stats.TotalRequests = totalRequests
	stats.TotalTokens = totalTokens
	stats.RequestTokens = requestTokens
	stats.ResponseTokens = responseTokens
	stats.AvgLatencyMs = avgLatencyMs
	stats.SuccessCount = successCount
	stats.ErrorCount = errorCount
	if totalRequests > 0 {
		stats.SuccessRate = float64(successCount) / float64(totalRequests)
	}

	// Get tokens by provider
	providerQuery := fmt.Sprintf(`
		SELECT provider, COALESCE(SUM(request_tokens + response_tokens), 0) as total_tokens
		FROM request_logs %s
		GROUP BY provider
	`, whereClause)

	providerRows, err := db.conn.Query(providerQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get tokens by provider: %w", err)
	}
	defer providerRows.Close()

	for providerRows.Next() {
		var provider string
		var tokens int64
		if err := providerRows.Scan(&provider, &tokens); err != nil {
			return nil, fmt.Errorf("failed to scan provider tokens: %w", err)
		}
		stats.TokensByProvider[provider] = tokens
	}
	if err := providerRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating provider tokens: %w", err)
	}

	// Get requests by model
	modelQuery := fmt.Sprintf(`
		SELECT model, COUNT(*) as request_count
		FROM request_logs %s
		GROUP BY model
	`, whereClause)

	modelRows, err := db.conn.Query(modelQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get requests by model: %w", err)
	}
	defer modelRows.Close()

	for modelRows.Next() {
		var model string
		var count int
		if err := modelRows.Scan(&model, &count); err != nil {
			return nil, fmt.Errorf("failed to scan model requests: %w", err)
		}
		stats.RequestsByModel[model] = count
	}
	if err := modelRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating model requests: %w", err)
	}

	// Get requests by client
	clientQuery := fmt.Sprintf(`
		SELECT COALESCE(client_id, 'anonymous'), COUNT(*) as request_count
		FROM request_logs %s
		GROUP BY client_id
	`, whereClause)

	clientRows, err := db.conn.Query(clientQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get requests by client: %w", err)
	}
	defer clientRows.Close()

	for clientRows.Next() {
		var clientID string
		var count int
		if err := clientRows.Scan(&clientID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan client requests: %w", err)
		}
		stats.RequestsByClient[clientID] = count
	}
	if err := clientRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating client requests: %w", err)
	}

	return stats, nil
}

// GetRequestLogByID retrieves a single request log by ID
func (db *DB) GetRequestLogByID(id int) (*RequestLog, error) {
	query := `SELECT id, client_id, provider, model, endpoint, request_tokens, response_tokens, latency_ms, status_code, error_message, created_at FROM request_logs WHERE id = ?`
	log := &RequestLog{}
	err := db.conn.QueryRow(query, id).Scan(
		&log.ID, &log.ClientID, &log.Provider, &log.Model, &log.Endpoint,
		&log.RequestTokens, &log.ResponseTokens, &log.LatencyMs,
		&log.StatusCode, &log.ErrorMessage, &log.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get request log: %w", err)
	}
	return log, nil
}

// DeleteOldRequestLogs removes request logs older than the specified time
func (db *DB) DeleteOldRequestLogs(before time.Time) (int64, error) {
	result, err := db.conn.Exec("DELETE FROM request_logs WHERE created_at < ?", before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old request logs: %w", err)
	}
	return result.RowsAffected()
}

// CountRequestLogs returns the total count of request logs matching the filter
func (db *DB) CountRequestLogs(filter *RequestLogFilter) (int, error) {
	query := "SELECT COUNT(*) FROM request_logs WHERE 1=1"
	var args []interface{}

	if filter != nil {
		if filter.ClientID != nil {
			query += " AND client_id = ?"
			args = append(args, *filter.ClientID)
		}
		if filter.Model != nil {
			query += " AND model = ?"
			args = append(args, *filter.Model)
		}
		if filter.Provider != nil {
			query += " AND provider = ?"
			args = append(args, *filter.Provider)
		}
		if filter.Endpoint != nil {
			query += " AND endpoint = ?"
			args = append(args, *filter.Endpoint)
		}
		if filter.StatusCode != nil {
			query += " AND status_code = ?"
			args = append(args, *filter.StatusCode)
		}
		if filter.Since != nil {
			query += " AND created_at >= ?"
			args = append(args, *filter.Since)
		}
		if filter.Until != nil {
			query += " AND created_at <= ?"
			args = append(args, *filter.Until)
		}
	}

	var count int
	err := db.conn.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count request logs: %w", err)
	}
	return count, nil
}

// GetClientStats retrieves stats for a specific client
func (db *DB) GetClientStats(clientID string) (*RequestLogStats, error) {
	return db.GetRequestLogStats(&RequestLogFilter{ClientID: &clientID})
}
