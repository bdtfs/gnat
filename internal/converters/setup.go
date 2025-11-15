package converters

import (
	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/server/dto"
)

func SetupToDTO(m *models.Setup) *dto.Setup {
	return &dto.Setup{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Method:      m.Method,
		URL:         m.URL,
		Body:        m.Body,
		Headers:     m.Headers,
		RPS:         m.RPS,
		Duration:    m.Duration,
		Status:      string(m.Status),
		HTTPConfig:  m.HTTPConfig,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func SetupFromDTO(d *dto.Setup) *models.Setup {
	return &models.Setup{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		Method:      d.Method,
		URL:         d.URL,
		Body:        d.Body,
		Headers:     d.Headers,
		RPS:         d.RPS,
		Duration:    d.Duration,
		Status:      models.SetupStatus(d.Status),
		HTTPConfig:  d.HTTPConfig,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}
