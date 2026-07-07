package app

import (
	"fmt"
	"testing"
)

// TestGuestCookieScalesTo500 locks in the requirement that single-QR guest
// state stays well within the browser's ~4 KB per-cookie limit even for a
// 500-prompt challenge. Storing 500 raw 15-char IDs would blow past that
// (~10 KB after base64); the positional bitset keeps it tiny, and the round
// trip must preserve both the current prompt and the full seen set.
func TestGuestCookieScalesTo500(t *testing.T) {
	const n = 500
	ids := make([]string, n)
	seen := map[string]bool{}
	for i := range ids {
		ids[i] = fmt.Sprintf("prompt%09d", i) // 15 chars, like a real PB id
		if i%3 == 0 {
			seen[ids[i]] = true
		}
	}
	cur := ids[321]

	value := encodeGuestCookie(cur, ids, seen)

	// Budget generously but far below the 4096-byte cookie ceiling.
	if len(value) > 1024 {
		t.Fatalf("cookie value for %d prompts is %d bytes, want <= 1024", n, len(value))
	}

	gotCur, gotSeen := decodeGuestCookie(value, ids)
	if gotCur != cur {
		t.Errorf("cur round-trip = %q, want %q", gotCur, cur)
	}
	if len(gotSeen) != len(seen) {
		t.Errorf("seen size = %d, want %d", len(gotSeen), len(seen))
	}
	for i, id := range ids {
		want := i%3 == 0
		if gotSeen[id] != want {
			t.Fatalf("seen[%d]=%v, want %v", i, gotSeen[id], want)
		}
	}
}

// TestGuestCookieReconcilesShrunkPrompts checks that when the prompt list
// shrinks (the owner deleted prompts), decoding against the new, shorter list
// simply ignores the extra trailing bits rather than erroring or leaking IDs.
func TestGuestCookieReconcilesShrunkPrompts(t *testing.T) {
	full := []string{"a00000000000000", "b00000000000000", "c00000000000000", "d00000000000000"}
	seen := map[string]bool{full[0]: true, full[2]: true, full[3]: true}
	value := encodeGuestCookie(full[3], full, seen)

	// Owner deleted the last two prompts; decode against the surviving two.
	shrunk := full[:2]
	cur, gotSeen := decodeGuestCookie(value, shrunk)

	if cur != full[3] {
		t.Errorf("cur = %q, want %q (cur is validated by the caller, not here)", cur, full[3])
	}
	if !gotSeen[full[0]] {
		t.Errorf("expected surviving seen prompt %q to remain", full[0])
	}
	if _, ok := gotSeen[full[2]]; ok {
		t.Errorf("deleted prompt %q must not appear in seen", full[2])
	}
	if len(gotSeen) != 1 {
		t.Errorf("seen size = %d, want 1", len(gotSeen))
	}
}

// TestDecodeGuestCookieGarbage confirms tampered/garbage cookies degrade to a
// fresh guest rather than panicking.
func TestDecodeGuestCookieGarbage(t *testing.T) {
	ids := []string{"a00000000000000", "b00000000000000"}
	for _, v := range []string{"", "not-base64!!", "YWJj" /* "abc", not JSON */} {
		cur, seen := decodeGuestCookie(v, ids)
		if cur != "" || len(seen) != 0 {
			t.Errorf("garbage %q yielded cur=%q seen=%v, want empty", v, cur, seen)
		}
	}
}
