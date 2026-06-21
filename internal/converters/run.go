package converters

import (
	"sort"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/server/dto"
)

func RunToDTO(m *models.Run) *dto.Run {
	var stats *dto.Stats
	if m.Stats != nil {
		stats = StatsToDTO(m.Stats, m.StartedAt, m.EndedAt)
	}

	out := &dto.Run{
		ID:        m.ID,
		SetupID:   m.SetupID,
		Status:    string(m.Status),
		StartedAt: m.StartedAt,
		Elapsed:   time.Since(m.StartedAt).String(),
		Error:     m.Error,
		Stats:     stats,
	}

	if !m.EndedAt.IsZero() {
		out.EndedAt = &m.EndedAt
		out.Elapsed = m.EndedAt.Sub(m.StartedAt).String()
	}

	return out
}

func StatsToDTO(m *models.Stats, startedAt, endedAt time.Time) *dto.Stats {
	if m == nil {
		return nil
	}

	m.StatusMu.RLock()
	statusCodes := make(map[int]uint64, len(m.StatusCodes))
	for code, ptr := range m.StatusCodes {
		if ptr != nil {
			statusCodes[code] = *ptr
		}
	}
	m.StatusMu.RUnlock()

	m.ErrorsMu.RLock()
	errorsCopy := append([]string(nil), m.Errors...)
	m.ErrorsMu.RUnlock()

	m.LatenciesMu.Lock()
	lat := append([]time.Duration(nil), m.Latencies...)
	m.LatenciesMu.Unlock()

	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	lowest, highest, average := aggregateLatencies(lat)

	elapsed := endedAt.Sub(startedAt).Seconds()
	var rps float64
	if elapsed > 0 {
		rps = float64(m.TotalRequests) / elapsed
	}

	var successRate float64
	if m.TotalRequests > 0 {
		successRate = float64(m.SuccessRequests) / float64(m.TotalRequests)
	}

	m.TimeSeriesMu.RLock()
	timeSeries := copyTimeSeries(m.TimeSeries)
	m.TimeSeriesMu.RUnlock()

	return &dto.Stats{
		Total:       m.TotalRequests,
		Success:     m.SuccessRequests,
		Failed:      m.FailedRequests,
		AvgLatency:  average,
		MinLatency:  lowest,
		MaxLatency:  highest,
		P50Latency:  percentile(lat, 0.50),
		P90Latency:  percentile(lat, 0.90),
		P95Latency:  percentile(lat, 0.95),
		P99Latency:  percentile(lat, 0.99),
		SuccessRate: successRate,
		RPS:         rps,
		BytesRead:   m.TotalBytesRead,
		StatusCodes: statusCodes,
		Errors:      errorsCopy,
		TimeSeries:  timeSeries,
	}
}

func aggregateLatencies(sorted []time.Duration) (lowest, highest, average float64) {
	if len(sorted) == 0 {
		return 0, 0, 0
	}

	lowest = float64(sorted[0].Milliseconds())
	highest = float64(sorted[len(sorted)-1].Milliseconds())

	var total time.Duration
	for _, v := range sorted {
		total += v
	}
	average = float64(total.Milliseconds()) / float64(len(sorted))

	return lowest, highest, average
}

func copyTimeSeries(points []models.TimeSeriesPoint) []dto.TimeSeriesPoint {
	if len(points) == 0 {
		return nil
	}

	out := make([]dto.TimeSeriesPoint, len(points))
	for i, p := range points {
		out[i] = dto.TimeSeriesPoint{
			TimestampMs:  p.TimestampMs,
			P50Latency:   p.P50Latency,
			P90Latency:   p.P90Latency,
			P95Latency:   p.P95Latency,
			P99Latency:   p.P99Latency,
			RPS:          p.RPS,
			ErrorRate:    p.ErrorRate,
			SuccessCount: p.SuccessCount,
			FailedCount:  p.FailedCount,
		}
	}

	return out
}

func percentile(sorted []time.Duration, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx].Milliseconds())
}
