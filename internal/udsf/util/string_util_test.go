package util

import "testing"

func TestCreateBlockKey(t *testing.T) {
	tests := []struct {
		name                       string
		realm, storage, rec, block string
		want                       string
	}{
		{"all set", "r", "s", "rec", "b", "udsf:r:s:rec:b"},
		{"empty parts", "", "", "", "", "udsf::::"},
		{"separator in id is kept", "r:1", "s", "rec", "b", "udsf:r:1:s:rec:b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateBlockKey(tt.realm, tt.storage, tt.rec, tt.block); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateRecordBlockPattern(t *testing.T) {
	tests := []struct {
		name                string
		realm, storage, rec string
		want                string
	}{
		{"plain", "r", "s", "rec", "udsf:r:s:rec:*"},
		{"star escaped", "r*", "s", "rec", `udsf:r\*:s:rec:*`},
		{"question escaped", "r", "s?", "rec", `udsf:r:s\?:rec:*`},
		{"brackets escaped", "r", "s", "[a]", `udsf:r:s:\[a\]:*`},
		{"backslash escaped", `r\`, "s", "rec", `udsf:r\\:s:rec:*`},
		{"empty", "", "", "", "udsf::::*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateRecordBlockPattern(tt.realm, tt.storage, tt.rec); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
