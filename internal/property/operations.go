package property

import "time"

type MaintenanceRequest struct {
	Status string `json:"status"`
}

type TenantDocument struct {
	ExpiresAt time.Time `json:"expires_at"`
}

type InspectionReminder struct {
	DueAt time.Time `json:"due_at"`
	Done  bool      `json:"done"`
}

type Operations struct {
	AsOf        time.Time            `json:"as_of"`
	Maintenance []MaintenanceRequest `json:"maintenance_requests"`
	Documents   []TenantDocument     `json:"tenant_documents"`
	Inspections []InspectionReminder `json:"inspection_reminders"`
}

type Snapshot struct {
	OpenMaintenance   int `json:"open_maintenance"`
	ExpiringDocuments int `json:"expiring_documents"`
	DueInspections    int `json:"due_inspections"`
}

func Summarize(in Operations) Snapshot {
	cutoff := in.AsOf.Add(30 * 24 * time.Hour)
	var out Snapshot
	for _, request := range in.Maintenance {
		if request.Status != "closed" {
			out.OpenMaintenance++
		}
	}
	for _, document := range in.Documents {
		if !document.ExpiresAt.Before(in.AsOf) && !document.ExpiresAt.After(cutoff) {
			out.ExpiringDocuments++
		}
	}
	for _, inspection := range in.Inspections {
		if !inspection.Done && !inspection.DueAt.After(cutoff) {
			out.DueInspections++
		}
	}
	return out
}
