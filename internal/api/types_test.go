package api

import (
	"encoding/json"
	"testing"
)

// TestWorkloadJSONIncludesZeroValuedCostFields is a regression test for a
// real bug found live: the public API can legitimately return
// costAnalysis.next30Days.potentialSavings: 0 for a workload that still has
// nonzero waste (confirmed via a live debug trace against a real cluster —
// waste and potentialSavings are independent metrics). historical_waste,
// cost_per_hour, potential_savings, cost_increase, and running_minutes all
// previously had `omitempty`, which made encoding/json drop the key
// entirely whenever the value was exactly zero. That's indistinguishable
// from a missing/null value to a consumer doing `jq '.potential_savings'`.
func TestWorkloadJSONIncludesZeroValuedCostFields(t *testing.T) {
	w := Workload{
		Cost:             2.96,
		Waste:            2.95,
		HistoricalWaste:  0,
		CostPerHour:      0,
		PotentialSavings: 0,
		CostIncrease:     0,
		RunningMinutes:   0,
	}

	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{"historical_waste", "cost_per_hour", "potential_savings", "cost_increase", "running_minutes"} {
		value, present := decoded[key]
		if !present {
			t.Errorf("key %q missing from JSON output for a zero value; want it present as 0", key)
			continue
		}
		if value != float64(0) {
			t.Errorf("key %q = %v, want 0", key, value)
		}
	}
}

// TestWorkloadContainerJSONIncludesZeroRunningMinutes covers the same
// omitempty-on-a-meaningful-zero bug as
// TestWorkloadJSONIncludesZeroValuedCostFields, at the per-container level —
// a container that never ran in the observed window (e.g. still Pending)
// legitimately has running_minutes: 0.
func TestWorkloadContainerJSONIncludesZeroRunningMinutes(t *testing.T) {
	c := WorkloadContainer{Name: "sidecar", RunningMinutes: 0}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	value, present := decoded["running_minutes"]
	if !present {
		t.Fatal("key \"running_minutes\" missing from JSON output for a zero value; want it present as 0")
	}
	if value != float64(0) {
		t.Fatalf("running_minutes = %v, want 0", value)
	}
}
