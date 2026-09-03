package stats

import (
	"encoding/json/v2"
	"testing"
)

func TestStatsJSONContract(t *testing.T) {
	data, err := json.Marshal(Stats{
		MemoryUsagePct: 12.5,
		GuestCount:     7,
	})
	if err != nil {
		t.Fatalf("marshal Stats: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal Stats JSON: %v", err)
	}

	if _, ok := got["memoryUsagePct"]; !ok {
		t.Fatalf("missing memoryUsagePct in JSON: %s", data)
	}
	if _, ok := got["guestCount"]; !ok {
		t.Fatalf("missing guestCount in JSON: %s", data)
	}
	if _, ok := got["MemoryUsagePct"]; ok {
		t.Fatalf("unexpected Go field name MemoryUsagePct in JSON: %s", data)
	}
	if _, ok := got["GuestCount"]; ok {
		t.Fatalf("unexpected Go field name GuestCount in JSON: %s", data)
	}
}

func TestCollectorPeakStatsTrackHistoricalMaximum(t *testing.T) {
	sc := &Collector{}

	first := Stats{MemoryUsage: 100, Goroutines: 20}
	sc.updatePeaks(&first)
	if first.MaxMemoryUsage != 100 || first.MaxGoroutines != 20 {
		t.Fatalf("first peak = (%v, %d), want (100, 20)", first.MaxMemoryUsage, first.MaxGoroutines)
	}

	lower := Stats{MemoryUsage: 80, Goroutines: 10}
	sc.updatePeaks(&lower)
	if lower.MaxMemoryUsage != 100 || lower.MaxGoroutines != 20 {
		t.Fatalf("lower sample peak = (%v, %d), want historical (100, 20)", lower.MaxMemoryUsage, lower.MaxGoroutines)
	}

	higher := Stats{MemoryUsage: 120, Goroutines: 30}
	sc.updatePeaks(&higher)
	if higher.MaxMemoryUsage != 120 || higher.MaxGoroutines != 30 {
		t.Fatalf("higher sample peak = (%v, %d), want (120, 30)", higher.MaxMemoryUsage, higher.MaxGoroutines)
	}
}
