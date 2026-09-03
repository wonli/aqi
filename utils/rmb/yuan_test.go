package rmb

import "testing"

func TestYuanToFen(t *testing.T) {
	tests := []struct {
		name    string
		yuan    string
		ratio   float64
		want    int64
		wantErr bool
	}{
		{name: "integer", yuan: "12", ratio: 1, want: 1200},
		{name: "one decimal", yuan: "12.3", ratio: 1, want: 1230},
		{name: "two decimals", yuan: "12.34", ratio: 1, want: 1234},
		{name: "extra decimals truncated", yuan: "12.349", ratio: 1, want: 1234},
		{name: "percentage", yuan: "12.34", ratio: 0.5, want: 617},
		{name: "invalid", yuan: "abc", ratio: 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := YuanToFen(tt.yuan, tt.ratio)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("YuanToFen(%q) expected error", tt.yuan)
				}
				return
			}
			if err != nil {
				t.Fatalf("YuanToFen(%q) returned error: %v", tt.yuan, err)
			}
			if got != tt.want {
				t.Fatalf("YuanToFen(%q, %v) = %d, want %d", tt.yuan, tt.ratio, got, tt.want)
			}
		})
	}
}

func TestFloatYuanToFen(t *testing.T) {
	tests := []struct {
		name   string
		yuan   float64
		places int
		want   int64
	}{
		{name: "two decimals", yuan: 12.34, places: 2, want: 1234},
		{name: "rounding", yuan: 12.345, places: 2, want: 1235},
		{name: "one decimal", yuan: 12.34, places: 1, want: 123},
		{name: "zero decimals", yuan: 12.6, places: 0, want: 13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FloatYuanToFen(tt.yuan, tt.places)
			if err != nil {
				t.Fatalf("FloatYuanToFen(%v, %d) returned error: %v", tt.yuan, tt.places, err)
			}
			if got != tt.want {
				t.Fatalf("FloatYuanToFen(%v, %d) = %d, want %d", tt.yuan, tt.places, got, tt.want)
			}
		})
	}
}
