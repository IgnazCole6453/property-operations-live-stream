package property

import (
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	now := time.Date(2026, time.October, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   Operations
		want Snapshot
	}{
		{
			name: "counts work that needs attention in the next thirty days",
			in: Operations{
				AsOf: now,
				Maintenance: []MaintenanceRequest{
					{Status: "open"}, {Status: "scheduled"}, {Status: "closed"},
				},
				Documents: []TenantDocument{
					{ExpiresAt: now.Add(10 * 24 * time.Hour)},
					{ExpiresAt: now.Add(45 * 24 * time.Hour)},
					{ExpiresAt: now.Add(-24 * time.Hour)},
				},
				Inspections: []InspectionReminder{
					{DueAt: now.Add(5 * 24 * time.Hour)},
					{DueAt: now.Add(5 * 24 * time.Hour), Done: true},
					{DueAt: now.Add(31 * 24 * time.Hour)},
				},
			},
			want: Snapshot{OpenMaintenance: 2, ExpiringDocuments: 1, DueInspections: 1},
		},
		{
			name: "empty portfolio is all zeroes",
			in:   Operations{AsOf: now},
			want: Snapshot{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Summarize(tt.in); got != tt.want {
				t.Fatalf("Summarize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
