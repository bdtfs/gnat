package converters

import (
	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/server/dto"
	"sort"
	"time"
)

func RunToDTO(m *models.Run) *dto.Run {
	var stats *dto.Stats
	if m.Stats != nil {
		stats = StatsToDTO(m.Stats, m.StartedAt, m.EndedAt)
	}

	return &dto.Run{
		ID:        m.ID,
		SetupID:   m.SetupID,
		Status:    string(m.Status),
		StartedAt: m.StartedAt,
		EndedAt:   m.EndedAt,
		Error:     m.Error,
		Stats:     stats,
	}
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

	var lowest, highest float64
	if len(lat) > 0 {
		lowest = float64(lat[0].Milliseconds())
		highest = float64(lat[len(lat)-1].Milliseconds())
	}

	var average float64
	if len(lat) > 0 {
		var total time.Duration
		for _, v := range lat {
			total += v
		}
		average = float64(total.Milliseconds()) / float64(len(lat))
	}

	elapsed := endedAt.Sub(startedAt).Seconds()
	var rps float64
	if elapsed > 0 {
		rps = float64(m.TotalRequests) / elapsed
	}

	var successRate float64
	if m.TotalRequests > 0 {
		successRate = float64(m.SuccessRequests) / float64(m.TotalRequests)
	}

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
	}
}

func percentile(sorted []time.Duration, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx].Milliseconds())
}
