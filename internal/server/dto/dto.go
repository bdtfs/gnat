package dto

import "time"

type Setup struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Method      string                 `json:"method"`
	URL         string                 `json:"url"`
	Body        []byte                 `json:"body,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	RPS         int                    `json:"rps"`
	Duration    time.Duration          `json:"duration"`
	Status      string                 `json:"status"`
	HTTPConfig  map[string]interface{} `json:"http_config,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Run struct {
	ID        string    `json:"id"`
	SetupID   string    `json:"setup_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Error     string    `json:"error,omitempty"`
	Stats     *Stats    `json:"stats"`
}

type Stats struct {
	Total       uint64         `json:"total"`
	Success     uint64         `json:"success"`
	Failed      uint64         `json:"failed"`
	AvgLatency  float64        `json:"avg_latency_ms"`
	MinLatency  float64        `json:"min_latency_ms"`
	MaxLatency  float64        `json:"max_latency_ms"`
	P50Latency  float64        `json:"p50_latency_ms"`
	P90Latency  float64        `json:"p90_latency_ms"`
	P95Latency  float64        `json:"p95_latency_ms"`
	P99Latency  float64        `json:"p99_latency_ms"`
	SuccessRate float64        `json:"success_rate"`
	RPS         float64        `json:"rps"`
	BytesRead   uint64         `json:"bytes_read"`
	StatusCodes map[int]uint64 `json:"status_codes"`
	Errors      []string       `json:"errors,omitempty"`
}
