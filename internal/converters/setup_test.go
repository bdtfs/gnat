package converters

import (
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/server/dto"
)

func TestSetupToDTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model *models.Setup
	}{
		{
			name: "full setup",
			model: &models.Setup{
				ID:          "setup-123",
				Name:        "Test Setup",
				Description: "A test setup",
				Method:      "POST",
				URL:         "https://api.example.com",
				Body:        []byte(`{"key": "value"}`),
				Headers:     map[string]string{"Content-Type": "application/json"},
				RPS:         100,
				Duration:    30 * time.Second,
				Status:      models.SetupStatusActive,
				HTTPConfig:  map[string]interface{}{"timeout": 10},
				CreatedAt:   time.Date(2026, 1, 18, 12, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2026, 1, 18, 12, 30, 0, 0, time.UTC),
			},
		},
		{
			name: "minimal setup",
			model: &models.Setup{
				ID:     "setup-456",
				Name:   "Minimal",
				Method: "GET",
				URL:    "http://example.com",
				Status: models.SetupStatusInactive,
			},
		},
		{
			name: "setup with nil maps",
			model: &models.Setup{
				ID:         "setup-789",
				Name:       "No Maps",
				Method:     "GET",
				URL:        "http://example.com",
				Headers:    nil,
				HTTPConfig: nil,
				Status:     models.SetupStatusActive,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := SetupToDTO(tt.model)

			if d.ID != tt.model.ID {
				t.Errorf("expected ID %q, got %q", tt.model.ID, d.ID)
			}
			if d.Name != tt.model.Name {
				t.Errorf("expected Name %q, got %q", tt.model.Name, d.Name)
			}
			if d.Description != tt.model.Description {
				t.Errorf("expected Description %q, got %q", tt.model.Description, d.Description)
			}
			if d.Method != tt.model.Method {
				t.Errorf("expected Method %q, got %q", tt.model.Method, d.Method)
			}
			if d.URL != tt.model.URL {
				t.Errorf("expected URL %q, got %q", tt.model.URL, d.URL)
			}
			if d.RPS != tt.model.RPS {
				t.Errorf("expected RPS %d, got %d", tt.model.RPS, d.RPS)
			}
			if d.Duration != tt.model.Duration {
				t.Errorf("expected Duration %v, got %v", tt.model.Duration, d.Duration)
			}
			if d.Status != string(tt.model.Status) {
				t.Errorf("expected Status %q, got %q", tt.model.Status, d.Status)
			}
			if !d.CreatedAt.Equal(tt.model.CreatedAt) {
				t.Errorf("expected CreatedAt %v, got %v", tt.model.CreatedAt, d.CreatedAt)
			}
			if !d.UpdatedAt.Equal(tt.model.UpdatedAt) {
				t.Errorf("expected UpdatedAt %v, got %v", tt.model.UpdatedAt, d.UpdatedAt)
			}
		})
	}
}

func TestSetupFromDTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dto  *dto.Setup
	}{
		{
			name: "full setup",
			dto: &dto.Setup{
				ID:          "setup-123",
				Name:        "Test Setup",
				Description: "A test setup",
				Method:      "POST",
				URL:         "https://api.example.com",
				Body:        []byte(`{"key": "value"}`),
				Headers:     map[string]string{"Content-Type": "application/json"},
				RPS:         100,
				Duration:    30 * time.Second,
				Status:      "active",
				HTTPConfig:  map[string]interface{}{"timeout": 10},
				CreatedAt:   time.Date(2026, 1, 18, 12, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2026, 1, 18, 12, 30, 0, 0, time.UTC),
			},
		},
		{
			name: "minimal setup",
			dto: &dto.Setup{
				ID:     "setup-456",
				Name:   "Minimal",
				Method: "GET",
				URL:    "http://example.com",
				Status: "inactive",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := SetupFromDTO(tt.dto)

			if m.ID != tt.dto.ID {
				t.Errorf("expected ID %q, got %q", tt.dto.ID, m.ID)
			}
			if m.Name != tt.dto.Name {
				t.Errorf("expected Name %q, got %q", tt.dto.Name, m.Name)
			}
			if m.Description != tt.dto.Description {
				t.Errorf("expected Description %q, got %q", tt.dto.Description, m.Description)
			}
			if m.Method != tt.dto.Method {
				t.Errorf("expected Method %q, got %q", tt.dto.Method, m.Method)
			}
			if m.URL != tt.dto.URL {
				t.Errorf("expected URL %q, got %q", tt.dto.URL, m.URL)
			}
			if m.RPS != tt.dto.RPS {
				t.Errorf("expected RPS %d, got %d", tt.dto.RPS, m.RPS)
			}
			if m.Duration != tt.dto.Duration {
				t.Errorf("expected Duration %v, got %v", tt.dto.Duration, m.Duration)
			}
			if string(m.Status) != tt.dto.Status {
				t.Errorf("expected Status %q, got %q", tt.dto.Status, m.Status)
			}
		})
	}
}

func TestSetupRoundTrip(t *testing.T) {
	t.Parallel()

	original := &models.Setup{
		ID:          "round-trip-id",
		Name:        "Round Trip Test",
		Description: "Testing round trip conversion",
		Method:      "PUT",
		URL:         "https://api.example.com/resource",
		Body:        []byte(`{"update": true}`),
		Headers:     map[string]string{"Authorization": "Bearer token"},
		RPS:         200,
		Duration:    2 * time.Minute,
		Status:      models.SetupStatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	d := SetupToDTO(original)
	back := SetupFromDTO(d)

	if back.ID != original.ID {
		t.Errorf("round trip ID mismatch: expected %q, got %q", original.ID, back.ID)
	}
	if back.Name != original.Name {
		t.Errorf("round trip Name mismatch: expected %q, got %q", original.Name, back.Name)
	}
	if back.Method != original.Method {
		t.Errorf("round trip Method mismatch: expected %q, got %q", original.Method, back.Method)
	}
	if back.URL != original.URL {
		t.Errorf("round trip URL mismatch: expected %q, got %q", original.URL, back.URL)
	}
	if back.RPS != original.RPS {
		t.Errorf("round trip RPS mismatch: expected %d, got %d", original.RPS, back.RPS)
	}
	if back.Duration != original.Duration {
		t.Errorf("round trip Duration mismatch: expected %v, got %v", original.Duration, back.Duration)
	}
	if back.Status != original.Status {
		t.Errorf("round trip Status mismatch: expected %q, got %q", original.Status, back.Status)
	}
}
