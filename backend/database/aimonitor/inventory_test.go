package aimonitor

import (
	"testing"
	"time"
)

func TestInventoryIndependentOfUsageFilters(t *testing.T) {
	s := fixture(t)
	now := time.Unix(1780000000, 0)
	r := validReport(now)
	if _, err := s.Ingest("node", r, now); err != nil {
		t.Fatal(err)
	}
	inventory, err := s.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range inventory.Selectors {
		if row.Model == r.Samples[0].Model {
			found = true
		}
	}
	if len(inventory.Tasks) != 1 || !found {
		t.Fatalf("missing inventory: %+v", inventory)
	}
}
