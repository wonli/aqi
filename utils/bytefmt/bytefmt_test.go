package bytefmt

import "testing"

func TestByteSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		want  string
	}{
		{name: "zero", bytes: 0, want: "0B"},
		{name: "bytes", bytes: 512, want: "512B"},
		{name: "kilobyte", bytes: KILOBYTE, want: "1K"},
		{name: "fractional kilobyte", bytes: 1536, want: "1.5K"},
		{name: "megabyte", bytes: 2 * MEGABYTE, want: "2M"},
		{name: "gigabyte", bytes: 3 * GIGABYTE, want: "3G"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ByteSize(tt.bytes); got != tt.want {
				t.Fatalf("ByteSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestToBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "bytes", input: "512B", want: 512},
		{name: "kilobytes", input: "1K", want: KILOBYTE},
		{name: "binary suffix", input: "1.5MiB", want: MEGABYTE + MEGABYTE/2},
		{name: "lowercase and spaces", input: " 2gb ", want: 2 * GIGABYTE},
		{name: "missing unit", input: "1024", wantErr: true},
		{name: "negative", input: "-1K", wantErr: true},
		{name: "unknown unit", input: "1XB", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToBytes(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ToBytes(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ToBytes(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ToBytes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestByteSizeRoundTripForExactValues(t *testing.T) {
	values := []uint64{0, 512, KILOBYTE, 1536, 2 * MEGABYTE, 3 * GIGABYTE}
	for _, value := range values {
		formatted := ByteSize(value)
		got, err := ToBytes(formatted)
		if err != nil {
			t.Fatalf("ToBytes(ByteSize(%d)=%q) returned error: %v", value, formatted, err)
		}
		if got != value {
			t.Fatalf("round trip for %d produced %d via %q", value, got, formatted)
		}
	}
}
