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

func TestCreateRecordKey(t *testing.T) {
	tests := []struct{ name, realm, storage, rec, want string }{
		{"all set", "r", "s", "rec", "udsf:r:s:rec"},
		{"empty parts", "", "", "", "udsf:::"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateRecordKey(tt.realm, tt.storage, tt.rec); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A record key must never be matched by the record's own block pattern, otherwise listing
// blocks (and deleting them with the record) would pick up the record itself.
func TestRecordKeyIsNotMatchedByBlockPattern(t *testing.T) {
	key := CreateRecordKey("r", "s", "rec")
	pat := CreateRecordBlockPattern("r", "s", "rec")
	if len(key) >= len(pat)-1 && key[:len(pat)-1] == pat[:len(pat)-1] {
		t.Fatalf("record key %q falls under block pattern %q", key, pat)
	}
	if blk := CreateBlockKey("r", "s", "rec", "b"); blk[:len(pat)-1] != pat[:len(pat)-1] {
		t.Fatalf("block key %q should fall under pattern %q", blk, pat)
	}
}

func TestCreateBlockIndexKey(t *testing.T) {
	if got := CreateBlockIndexKey("r", "s", "rec"); got != "udsfidx:r:s:rec" {
		t.Fatalf("got %q", got)
	}
	// must never be matched by the block pattern or equal the record key
	pat := CreateRecordBlockPattern("r", "s", "rec")
	idx := CreateBlockIndexKey("r", "s", "rec")
	if len(idx) >= len(pat)-1 && idx[:len(pat)-1] == pat[:len(pat)-1] {
		t.Fatalf("index key %q collides with pattern %q", idx, pat)
	}
	if idx == CreateRecordKey("r", "s", "rec") {
		t.Fatal("index key equals record key")
	}
}
