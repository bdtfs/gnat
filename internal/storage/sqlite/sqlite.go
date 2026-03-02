package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/models"

	_ "modernc.org/sqlite"
)

// Repository implements storage.Repository backed by SQLite.
type Repository struct {
	db *sql.DB
	mu sync.RWMutex
}

// New creates a new SQLite repository. The dbPath is the file path for the
// SQLite database (e.g. "gnat.db"). Use ":memory:" for an in-memory database.
func New(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign keys.
	if _, err = db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	r := &Repository{db: db}

	if err = r.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return r, nil
}

// Close closes the underlying database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS setups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL,
			url TEXT NOT NULL,
			headers TEXT,
			body BLOB,
			rps INTEGER NOT NULL,
			duration INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			http_config TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			setup_id TEXT NOT NULL REFERENCES setups(id),
			status TEXT NOT NULL,
			started_at TEXT,
			ended_at TEXT,
			error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS stats (
			run_id TEXT PRIMARY KEY REFERENCES runs(id),
			total INTEGER NOT NULL DEFAULT 0,
			success INTEGER NOT NULL DEFAULT 0,
			failed INTEGER NOT NULL DEFAULT 0,
			avg_latency_ms REAL NOT NULL DEFAULT 0,
			p50_latency_ms REAL NOT NULL DEFAULT 0,
			p90_latency_ms REAL NOT NULL DEFAULT 0,
			p95_latency_ms REAL NOT NULL DEFAULT 0,
			p99_latency_ms REAL NOT NULL DEFAULT 0,
			max_latency_ms REAL NOT NULL DEFAULT 0,
			min_latency_ms REAL NOT NULL DEFAULT 0,
			success_rate REAL NOT NULL DEFAULT 0,
			rps REAL NOT NULL DEFAULT 0,
			bytes_read INTEGER NOT NULL DEFAULT 0,
			status_codes TEXT,
			errors TEXT
		)`,
	}

	for _, q := range queries {
		if _, err := r.db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q[:40], err)
		}
	}

	return nil
}

// --- Setup methods ---

func (r *Repository) CreateSetup(setup *models.Setup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	headersJSON, err := marshalJSON(setup.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}

	httpConfigJSON, err := marshalJSON(setup.HTTPConfig)
	if err != nil {
		return fmt.Errorf("marshal http_config: %w", err)
	}

	_, err = r.db.Exec(
		`INSERT INTO setups (id, name, description, method, url, headers, body, rps, duration, status, http_config, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		setup.ID,
		setup.Name,
		setup.Description,
		setup.Method,
		setup.URL,
		headersJSON,
		setup.Body,
		setup.RPS,
		int64(setup.Duration),
		string(setup.Status),
		httpConfigJSON,
		setup.CreatedAt.Format(time.RFC3339Nano),
		setup.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("setup with id %s already exists", setup.ID)
	}

	return nil
}

func (r *Repository) GetSetup(id string) (*models.Setup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getSetup(id)
}

func (r *Repository) getSetup(id string) (*models.Setup, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, method, url, headers, body, rps, duration, status, http_config, created_at, updated_at
		 FROM setups WHERE id = ?`, id)

	return scanSetup(row)
}

func (r *Repository) ListSetups() []*models.Setup {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.Query(
		`SELECT id, name, description, method, url, headers, body, rps, duration, status, http_config, created_at, updated_at
		 FROM setups ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var setups []*models.Setup
	for rows.Next() {
		s, err := scanSetupRows(rows)
		if err != nil {
			continue
		}
		setups = append(setups, s)
	}

	if setups == nil {
		return []*models.Setup{}
	}
	return setups
}

func (r *Repository) UpdateSetup(setup *models.Setup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check existence first.
	if _, err := r.getSetup(setup.ID); err != nil {
		return err
	}

	headersJSON, err := marshalJSON(setup.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}

	httpConfigJSON, err := marshalJSON(setup.HTTPConfig)
	if err != nil {
		return fmt.Errorf("marshal http_config: %w", err)
	}

	_, err = r.db.Exec(
		`UPDATE setups SET name=?, description=?, method=?, url=?, headers=?, body=?, rps=?, duration=?, status=?, http_config=?, updated_at=?
		 WHERE id=?`,
		setup.Name,
		setup.Description,
		setup.Method,
		setup.URL,
		headersJSON,
		setup.Body,
		setup.RPS,
		int64(setup.Duration),
		string(setup.Status),
		httpConfigJSON,
		setup.UpdatedAt.Format(time.RFC3339Nano),
		setup.ID,
	)
	return err
}

func (r *Repository) DeleteSetup(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.db.Exec(`DELETE FROM setups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete setup: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("setup with id %s not found", id)
	}

	return nil
}

// --- Run methods ---

func (r *Repository) CreateRun(run *models.Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	startedAt := ""
	if !run.StartedAt.IsZero() {
		startedAt = run.StartedAt.Format(time.RFC3339Nano)
	}

	endedAt := ""
	if !run.EndedAt.IsZero() {
		endedAt = run.EndedAt.Format(time.RFC3339Nano)
	}

	_, err := r.db.Exec(
		`INSERT INTO runs (id, setup_id, status, started_at, ended_at, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		run.ID,
		run.SetupID,
		string(run.Status),
		startedAt,
		endedAt,
		run.Error,
		time.Now().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("run with id %s already exists", run.ID)
	}

	return nil
}

func (r *Repository) GetRun(id string) (*models.Run, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getRun(id)
}

func (r *Repository) getRun(id string) (*models.Run, error) {
	row := r.db.QueryRow(
		`SELECT id, setup_id, status, started_at, ended_at, error
		 FROM runs WHERE id = ?`, id)

	run, err := scanRun(row)
	if err != nil {
		return nil, fmt.Errorf("run with id %s not found", id)
	}

	// Load stats if they exist.
	stats, err := r.getStats(id)
	if err == nil {
		run.Stats = stats
	}

	return run, nil
}

func (r *Repository) ListRuns() []*models.Run {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.listRuns("")
}

func (r *Repository) ListRunsBySetup(setupID string) []*models.Run {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.listRuns(setupID)
}

func (r *Repository) listRuns(setupID string) []*models.Run {
	var rows *sql.Rows
	var err error

	if setupID == "" {
		rows, err = r.db.Query(
			`SELECT id, setup_id, status, started_at, ended_at, error
			 FROM runs ORDER BY created_at DESC`)
	} else {
		rows, err = r.db.Query(
			`SELECT id, setup_id, status, started_at, ended_at, error
			 FROM runs WHERE setup_id = ? ORDER BY created_at DESC`, setupID)
	}
	if err != nil {
		return []*models.Run{}
	}
	defer rows.Close()

	var runs []*models.Run
	for rows.Next() {
		run, err := scanRunRows(rows)
		if err != nil {
			continue
		}

		// Load stats if they exist.
		stats, err := r.getStats(run.ID)
		if err == nil {
			run.Stats = stats
		}

		runs = append(runs, run)
	}

	if runs == nil {
		return []*models.Run{}
	}
	return runs
}

func (r *Repository) UpdateRun(run *models.Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check existence first.
	if _, err := r.getRun(run.ID); err != nil {
		return err
	}

	startedAt := ""
	if !run.StartedAt.IsZero() {
		startedAt = run.StartedAt.Format(time.RFC3339Nano)
	}

	endedAt := ""
	if !run.EndedAt.IsZero() {
		endedAt = run.EndedAt.Format(time.RFC3339Nano)
	}

	_, err := r.db.Exec(
		`UPDATE runs SET setup_id=?, status=?, started_at=?, ended_at=?, error=?
		 WHERE id=?`,
		run.SetupID,
		string(run.Status),
		startedAt,
		endedAt,
		run.Error,
		run.ID,
	)
	if err != nil {
		return err
	}

	// Persist stats if present.
	if run.Stats != nil {
		return r.saveStats(run.ID, run.Stats)
	}

	return nil
}

func (r *Repository) DeleteRun(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete stats first (foreign key).
	r.db.Exec(`DELETE FROM stats WHERE run_id = ?`, id)

	result, err := r.db.Exec(`DELETE FROM runs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("run with id %s not found", id)
	}

	return nil
}

// --- Stats methods ---

func (r *Repository) saveStats(runID string, stats *models.Stats) error {
	stats.StatusMu.RLock()
	statusCodes := make(map[int]uint64, len(stats.StatusCodes))
	for code, ptr := range stats.StatusCodes {
		if ptr != nil {
			statusCodes[code] = *ptr
		}
	}
	stats.StatusMu.RUnlock()

	statusCodesJSON, err := json.Marshal(statusCodes)
	if err != nil {
		return fmt.Errorf("marshal status_codes: %w", err)
	}

	stats.ErrorsMu.RLock()
	errorsCopy := append([]string(nil), stats.Errors...)
	stats.ErrorsMu.RUnlock()

	errorsJSON, err := json.Marshal(errorsCopy)
	if err != nil {
		return fmt.Errorf("marshal errors: %w", err)
	}

	// Compute latency percentiles.
	stats.LatenciesMu.Lock()
	lat := append([]time.Duration(nil), stats.Latencies...)
	stats.LatenciesMu.Unlock()

	var avgLatency, minLatency, maxLatency, p50, p90, p95, p99 float64
	if len(lat) > 0 {
		// Sort for percentile calculation.
		sortDurations(lat)
		minLatency = float64(lat[0].Milliseconds())
		maxLatency = float64(lat[len(lat)-1].Milliseconds())

		var total time.Duration
		for _, v := range lat {
			total += v
		}
		avgLatency = float64(total.Milliseconds()) / float64(len(lat))

		p50 = percentile(lat, 0.50)
		p90 = percentile(lat, 0.90)
		p95 = percentile(lat, 0.95)
		p99 = percentile(lat, 0.99)
	}

	var successRate, rps float64
	if stats.TotalRequests > 0 {
		successRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests)
	}

	_, err = r.db.Exec(
		`INSERT OR REPLACE INTO stats (run_id, total, success, failed, avg_latency_ms, p50_latency_ms, p90_latency_ms, p95_latency_ms, p99_latency_ms, max_latency_ms, min_latency_ms, success_rate, rps, bytes_read, status_codes, errors)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		stats.TotalRequests,
		stats.SuccessRequests,
		stats.FailedRequests,
		avgLatency,
		p50,
		p90,
		p95,
		p99,
		maxLatency,
		minLatency,
		successRate,
		rps,
		stats.TotalBytesRead,
		string(statusCodesJSON),
		string(errorsJSON),
	)

	return err
}

func (r *Repository) getStats(runID string) (*models.Stats, error) {
	row := r.db.QueryRow(
		`SELECT run_id, total, success, failed, avg_latency_ms, p50_latency_ms, p90_latency_ms, p95_latency_ms, p99_latency_ms, max_latency_ms, min_latency_ms, success_rate, rps, bytes_read, status_codes, errors
		 FROM stats WHERE run_id = ?`, runID)

	var (
		id                                                                         string
		total, success, failed, bytesRead                                          uint64
		avgLat, p50, p90, p95, p99, maxLat, minLat, successRate, rpsVal            float64
		statusCodesJSON, errorsJSON                                                sql.NullString
	)

	err := row.Scan(&id, &total, &success, &failed, &avgLat, &p50, &p90, &p95, &p99, &maxLat, &minLat, &successRate, &rpsVal, &bytesRead, &statusCodesJSON, &errorsJSON)
	if err != nil {
		return nil, err
	}

	stats := &models.Stats{
		TotalRequests:   total,
		SuccessRequests: success,
		FailedRequests:  failed,
		TotalBytesRead:  bytesRead,
		StatusCodes:     make(map[int]*uint64),
		Latencies:       make([]time.Duration, 0),
		Errors:          make([]string, 0),
	}

	// Reconstruct status codes map.
	if statusCodesJSON.Valid && statusCodesJSON.String != "" {
		var sc map[int]uint64
		if json.Unmarshal([]byte(statusCodesJSON.String), &sc) == nil {
			for code, count := range sc {
				c := count
				stats.StatusCodes[code] = &c
			}
		}
	}

	// Reconstruct errors slice.
	if errorsJSON.Valid && errorsJSON.String != "" {
		var errs []string
		if json.Unmarshal([]byte(errorsJSON.String), &errs) == nil {
			stats.Errors = errs
		}
	}

	return stats, nil
}

// --- Scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanSetup(row scannable) (*models.Setup, error) {
	var (
		id, name, description, method, url, status string
		createdAtStr, updatedAtStr                  string
		headersJSON, httpConfigJSON                  sql.NullString
		body                                        []byte
		rps                                         int
		durationNs                                  int64
	)

	err := row.Scan(&id, &name, &description, &method, &url, &headersJSON, &body, &rps, &durationNs, &status, &httpConfigJSON, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("setup with id not found: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339Nano, createdAtStr)
	updatedAt, _ := time.Parse(time.RFC3339Nano, updatedAtStr)

	setup := &models.Setup{
		ID:          id,
		Name:        name,
		Description: description,
		Method:      method,
		URL:         url,
		Body:        body,
		RPS:         rps,
		Duration:    time.Duration(durationNs),
		Status:      models.SetupStatus(status),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	if headersJSON.Valid && headersJSON.String != "" {
		var headers map[string]string
		if json.Unmarshal([]byte(headersJSON.String), &headers) == nil {
			setup.Headers = headers
		}
	}

	if httpConfigJSON.Valid && httpConfigJSON.String != "" {
		var httpConfig map[string]interface{}
		if json.Unmarshal([]byte(httpConfigJSON.String), &httpConfig) == nil {
			setup.HTTPConfig = httpConfig
		}
	}

	return setup, nil
}

func scanSetupRows(rows *sql.Rows) (*models.Setup, error) {
	return scanSetup(rows)
}

func scanRun(row scannable) (*models.Run, error) {
	var (
		id, setupID, status     string
		startedAtStr, endedAtStr sql.NullString
		runError                 string
	)

	err := row.Scan(&id, &setupID, &status, &startedAtStr, &endedAtStr, &runError)
	if err != nil {
		return nil, err
	}

	run := &models.Run{
		ID:      id,
		SetupID: setupID,
		Status:  models.RunStatus(status),
		Error:   runError,
	}

	if startedAtStr.Valid && startedAtStr.String != "" {
		t, _ := time.Parse(time.RFC3339Nano, startedAtStr.String)
		run.StartedAt = t
	}

	if endedAtStr.Valid && endedAtStr.String != "" {
		t, _ := time.Parse(time.RFC3339Nano, endedAtStr.String)
		run.EndedAt = t
	}

	return run, nil
}

func scanRunRows(rows *sql.Rows) (*models.Run, error) {
	return scanRun(rows)
}

// --- Utility helpers ---

func marshalJSON(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func sortDurations(d []time.Duration) {
	// Simple insertion sort, fine for the expected sizes.
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

func percentile(sorted []time.Duration, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx].Milliseconds())
}
