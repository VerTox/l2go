package outclient

import (
	"math"
	"testing"
)

// percentFromCurrentLevel must return the level-progress as a 0.0–1.0 fraction
// (the client EXP bar fill), matching the UserInfo packet convention.
func TestPercentFromCurrentLevel(t *testing.T) {
	tests := []struct {
		name  string
		exp   int64
		level int32
		want  float64
	}{
		// Level 5 spans EXP [2884, 6038); halfway point is 4461.
		{name: "halfway through level 5", exp: 4461, level: 5, want: 0.5},
		// Exactly at the level threshold → empty bar.
		{name: "at level 5 threshold", exp: 2884, level: 5, want: 0.0},
		// Just below the next threshold → near full bar.
		{name: "near level 6 threshold", exp: 6037, level: 5, want: 0.99968},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentFromCurrentLevel(tt.exp, tt.level)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("percentFromCurrentLevel(%d, %d) = %f, want %f", tt.exp, tt.level, got, tt.want)
			}
		})
	}
}
