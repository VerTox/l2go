package usecase

import (
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestMovementValidator_ValidateMovementRequest(t *testing.T) {
	validator := NewMovementValidator()

	tests := []struct {
		name       string
		charID     int32
		from       models.Position
		to         models.Position
		isRunning  bool
		wantErr    bool
		errorType  string
	}{
		{
			name:      "Valid walk movement",
			charID:    1,
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 1008, Y: 1000, Z: 0}, // 8 units distance (within 9.2 limit)
			isRunning: false,
			wantErr:   false,
		},
		{
			name:      "Valid run movement",
			charID:    1,
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 1013, Y: 1000, Z: 0}, // 13 units distance (within 13.8 limit)
			isRunning: true,
			wantErr:   false,
		},
		{
			name:      "Long distance movement (valid in L2J)",
			charID:    1,
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 6000, Y: 1000, Z: 0}, // 5000 units distance (valid in L2J < 9900)
			isRunning: false,
			wantErr:   false,
		},
		{
			name:      "Anti-teleport check - too far",
			charID:    1,
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 11000, Y: 1000, Z: 0}, // 10000 units distance (> 9900 = teleport)
			isRunning: true,
			wantErr:   true,
			errorType: ValidationErrorDistance,
		},
		{
			name:      "Out of world bounds - X too high",
			charID:    1,
			from:      models.Position{X: 200000, Y: 1000, Z: 0},
			to:        models.Position{X: 250000, Y: 1000, Z: 0}, // Beyond L2J world bounds (> 229376)
			isRunning: false,
			wantErr:   true,
			errorType: ValidationErrorBounds,
		},
		{
			name:      "Out of world bounds - Y too low",
			charID:    1,
			from:      models.Position{X: 1000, Y: -250000, Z: 0},
			to:        models.Position{X: 1000, Y: -280000, Z: 0}, // Beyond L2J world bounds (< -262144)
			isRunning: false,
			wantErr:   true,
			errorType: ValidationErrorBounds,
		},
		{
			name:      "Out of world bounds - Z too high",
			charID:    1,
			from:      models.Position{X: 1000, Y: 1000, Z: 16383},
			to:        models.Position{X: 1000, Y: 1000, Z: 16384}, // Beyond world bounds
			isRunning: false,
			wantErr:   true,
			errorType: ValidationErrorBounds,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMovementRequest(
				tt.charID,
				tt.from,
				tt.to,
				tt.isRunning,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateMovementRequest() expected error, got nil")
					return
				}

				// Check error type if specified
				if tt.errorType != "" {
					if mvErr, ok := err.(*MovementValidationError); ok {
						if mvErr.Type != tt.errorType {
							t.Errorf("ValidateMovementRequest() error type = %v, want %v", mvErr.Type, tt.errorType)
						}
					} else {
						t.Errorf("ValidateMovementRequest() expected MovementValidationError, got %T", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateMovementRequest() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestMovementValidator_ValidatePosition(t *testing.T) {
	validator := NewMovementValidator()

	tests := []struct {
		name        string
		charID      int32
		expectedPos models.Position
		clientPos   models.Position
		wantErr     bool
	}{
		{
			name:        "Position within L2J tolerance",
			charID:      1,
			expectedPos: models.Position{X: 1000, Y: 1000, Z: 0},
			clientPos:   models.Position{X: 1400, Y: 1300, Z: 0}, // ~500 units distance (within 600 limit)
			wantErr:     false,
		},
		{
			name:        "Position outside L2J tolerance (speed hack detection)",
			charID:      1,
			expectedPos: models.Position{X: 1000, Y: 1000, Z: 0},
			clientPos:   models.Position{X: 1500, Y: 1500, Z: 0}, // ~707 units distance (> 600 limit)
			wantErr:     true,
		},
		{
			name:        "Exact position match",
			charID:      1,
			expectedPos: models.Position{X: 1000, Y: 1000, Z: 0},
			clientPos:   models.Position{X: 1000, Y: 1000, Z: 0},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePosition(
				tt.charID,
				tt.expectedPos,
				tt.clientPos,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePosition() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePosition() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestMovementValidator_ShouldCorrectPosition(t *testing.T) {
	validator := NewMovementValidator()

	tests := []struct {
		name        string
		expectedPos models.Position
		clientPos   models.Position
		want        bool
	}{
		{
			name:        "Position within L2J correction threshold",
			expectedPos: models.Position{X: 1000, Y: 1000, Z: 0},
			clientPos:   models.Position{X: 1300, Y: 1000, Z: 0}, // 300 units distance
			want:        false, // Within L2J correction threshold of 500
		},
		{
			name:        "Position outside L2J correction threshold",
			expectedPos: models.Position{X: 1000, Y: 1000, Z: 0},
			clientPos:   models.Position{X: 1600, Y: 1000, Z: 0}, // 600 units distance
			want:        true, // Outside L2J correction threshold of 500
		},
		{
			name:        "Position at correction threshold boundary",
			expectedPos: models.Position{X: 1000, Y: 1000, Z: 0},
			clientPos:   models.Position{X: 1500, Y: 1000, Z: 0}, // 500 units distance
			want:        false, // Exactly at boundary (within threshold)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.ShouldCorrectPosition(tt.expectedPos, tt.clientPos)
			if got != tt.want {
				t.Errorf("ShouldCorrectPosition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateMovementTime(t *testing.T) {
	tests := []struct {
		name     string
		distance float64
		speed    float64
		want     time.Duration
	}{
		{
			name:     "80 units at speed 80",
			distance: 80.0,
			speed:    80.0,
			want:     time.Second, // 80 / 80 = 1 second
		},
		{
			name:     "120 units at speed 120",
			distance: 120.0,
			speed:    120.0,
			want:     time.Second, // 120 / 120 = 1 second
		},
		{
			name:     "40 units at speed 80",
			distance: 40.0,
			speed:    80.0,
			want:     500 * time.Millisecond, // 40 / 80 = 0.5 seconds
		},
		{
			name:     "240 units at speed 120",
			distance: 240.0,
			speed:    120.0,
			want:     2 * time.Second, // 240 / 120 = 2 seconds
		},
		{
			name:     "computed speed from stats (150 units/sec)",
			distance: 300.0,
			speed:    150.0,
			want:     2 * time.Second, // 300 / 150 = 2 seconds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateMovementTime(tt.distance, tt.speed)
			if got != tt.want {
				t.Errorf("CalculateMovementTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlayerMoveSpeed(t *testing.T) {
	computed := models.ComputedStats{RunSpd: 130, WalkSpd: 85}

	tests := []struct {
		name      string
		isRunning bool
		want      float64
	}{
		{name: "running uses RunSpd", isRunning: true, want: 130},
		{name: "walking uses WalkSpd", isRunning: false, want: 85},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlayerMoveSpeed(computed, tt.isRunning)
			if got != tt.want {
				t.Errorf("PlayerMoveSpeed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidMovementType(t *testing.T) {
	tests := []struct {
		name     string
		moveType int32
		want     bool
	}{
		{"Cursor keys movement", 0, true},
		{"Mouse movement", 1, true},
		{"Invalid negative type", -1, false},
		{"Invalid high type", 2, false},
		{"Invalid high type 2", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidMovementType(tt.moveType)
			if got != tt.want {
				t.Errorf("IsValidMovementType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSignificantMovement(t *testing.T) {
	tests := []struct {
		name      string
		from      models.Position
		to        models.Position
		threshold float64
		want      bool
	}{
		{
			name:      "Movement above threshold",
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 1020, Y: 1000, Z: 0}, // 20 units
			threshold: 10.0,
			want:      true,
		},
		{
			name:      "Movement below threshold",
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 1005, Y: 1000, Z: 0}, // 5 units
			threshold: 10.0,
			want:      false,
		},
		{
			name:      "Movement at threshold",
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 1010, Y: 1000, Z: 0}, // 10 units
			threshold: 10.0,
			want:      true, // >= threshold is significant
		},
		{
			name:      "No movement",
			from:      models.Position{X: 1000, Y: 1000, Z: 0},
			to:        models.Position{X: 1000, Y: 1000, Z: 0}, // 0 units
			threshold: 5.0,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSignificantMovement(tt.from, tt.to, tt.threshold)
			if got != tt.want {
				t.Errorf("IsSignificantMovement() = %v, want %v", got, tt.want)
			}
		})
	}
}