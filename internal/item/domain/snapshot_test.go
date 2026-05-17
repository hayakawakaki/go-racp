package domain

import "testing"

func TestEmptySnapshot(t *testing.T) {
	t.Parallel()

	snap := EmptySnapshot()
	if snap == nil {
		t.Fatal("EmptySnapshot() returned nil")
	}
	if !snap.LoadedAt.IsZero() {
		t.Errorf("LoadedAt = %v, want zero time", snap.LoadedAt)
	}
	if snap.ByID == nil {
		t.Errorf("ByID is nil, want non-nil empty map (writable without panic)")
	}
	if len(snap.ByID) != 0 {
		t.Errorf("ByID len = %d, want 0", len(snap.ByID))
	}
	if snap.ByName == nil {
		t.Errorf("ByName is nil, want non-nil empty map")
	}
	if len(snap.ByName) != 0 {
		t.Errorf("ByName len = %d, want 0", len(snap.ByName))
	}
	if snap.Sorted != nil {
		t.Errorf("Sorted = %v, want nil", snap.Sorted)
	}
	if snap.SourceCount != 0 {
		t.Errorf("SourceCount = %d, want 0", snap.SourceCount)
	}
}

func TestEmptySnapshot_MapsAreWritable(t *testing.T) {
	t.Parallel()

	snap := EmptySnapshot()
	snap.ByID[1] = &Item{ID: 1}
	snap.ByName["Red_Potion"] = &Item{ID: 1, AegisName: "Red_Potion"}

	if snap.ByID[1] == nil {
		t.Errorf("write then read on ByID failed")
	}
	if snap.ByName["Red_Potion"] == nil {
		t.Errorf("write then read on ByName failed")
	}
}
