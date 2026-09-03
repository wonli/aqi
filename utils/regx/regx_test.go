package regx

import "testing"

func TestVerifyPhoneNumber(t *testing.T) {
	valid := []string{"13800138000", "19912345678", "16612345678"}
	for _, phone := range valid {
		if err := VerifyPhoneNumber(phone); err != nil {
			t.Fatalf("VerifyPhoneNumber(%q) returned error: %v", phone, err)
		}
	}

	invalid := []string{"", "12800138000", "1380013800", "138001380000", "1380013800a"}
	for _, phone := range invalid {
		if err := VerifyPhoneNumber(phone); err == nil {
			t.Fatalf("VerifyPhoneNumber(%q) expected error, got nil", phone)
		}
	}
}

func TestValidateDates(t *testing.T) {
	tests := []struct {
		name    string
		dates   []string
		wantErr bool
	}{
		{name: "same day", dates: []string{"2026-09-03", "2026-09-03"}},
		{name: "ordered range", dates: []string{"2026-09-01", "2026-09-03"}},
		{name: "missing end", dates: []string{"2026-09-03"}, wantErr: true},
		{name: "invalid start", dates: []string{"2026-02-30", "2026-09-03"}, wantErr: true},
		{name: "invalid end", dates: []string{"2026-09-01", "bad"}, wantErr: true},
		{name: "reversed", dates: []string{"2026-09-03", "2026-09-01"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDates(tt.dates)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateDates(%v) error = %v, wantErr %v", tt.dates, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimes(t *testing.T) {
	tests := []struct {
		name    string
		times   []string
		wantErr bool
	}{
		{name: "same time", times: []string{"09:30", "09:30"}},
		{name: "ordered range", times: []string{"09:30", "18:00"}},
		{name: "missing end", times: []string{"09:30"}, wantErr: true},
		{name: "invalid start", times: []string{"24:00", "18:00"}, wantErr: true},
		{name: "invalid end", times: []string{"09:30", "18:60"}, wantErr: true},
		{name: "reversed", times: []string{"18:00", "09:30"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimes(tt.times)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateTimes(%v) error = %v, wantErr %v", tt.times, err, tt.wantErr)
			}
		})
	}
}

func TestCheckID(t *testing.T) {
	if !CheckID("11010519491231002X", false) {
		t.Fatal("known valid 18-digit ID was rejected")
	}
	if !CheckID("11010519491231002x", false) {
		t.Fatal("lowercase checksum X should be accepted")
	}
	if CheckID("110105194912310021", false) {
		t.Fatal("invalid checksum was accepted")
	}
	if CheckID("99010519491231002X", false) {
		t.Fatal("invalid area code was accepted")
	}
	if CheckID("11010520990230002X", false) {
		t.Fatal("invalid/future birthday was accepted")
	}
	if !CheckID("123456789012345", true) {
		t.Fatal("15-digit ID should pass length-only validation")
	}
	if CheckID("123", true) {
		t.Fatal("invalid length passed length-only validation")
	}
}
