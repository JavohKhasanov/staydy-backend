package intervention

import "testing"

func TestSuggestedActions_MapsReasons(t *testing.T) {
	got := SuggestedActions([]string{"Davomat juda past (<50%)", "To'lov qarzi bor"})
	if len(got) != 2 {
		t.Fatalf("want 2 actions, got %d: %v", len(got), got)
	}
}

func TestSuggestedActions_DedupesAndFallback(t *testing.T) {
	if a := SuggestedActions([]string{"Davomat past (<70%)", "Davomat juda past (<50%)"}); len(a) != 1 {
		t.Fatalf("want 1 deduped action, got %d", len(a))
	}
	if a := SuggestedActions([]string{"noma'lum sabab"}); len(a) != 1 {
		t.Fatalf("want fallback action, got %v", a)
	}
}
