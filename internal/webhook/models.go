// Package webhook implements the external-dns webhook provider HTTP API.
package webhook

// Endpoint represents a DNS record as understood by external-dns.
// Field names use camelCase JSON tags to match the external-dns wire format.
type Endpoint struct {
	DNSName          string            `json:"dnsName"`
	Targets          []string          `json:"targets"`
	RecordType       string            `json:"recordType"`
	RecordTTL        int64             `json:"recordTTL,omitempty"`
	SetIdentifier    string            `json:"setIdentifier,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	ProviderSpecific []ProviderSpecific `json:"providerSpecific,omitempty"`
}

// ProviderSpecific holds an arbitrary name/value pair attached to an Endpoint.
type ProviderSpecific struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Changes describes a set of DNS record operations requested by external-dns.
type Changes struct {
	// Create holds new endpoints to add.
	Create []*Endpoint `json:"create"`
	// UpdateOld holds the old values of endpoints being updated (to be deleted).
	UpdateOld []*Endpoint `json:"updateOld"`
	// UpdateNew holds the new values of endpoints being updated (to be created).
	UpdateNew []*Endpoint `json:"updateNew"`
	// Delete holds endpoints to remove.
	Delete []*Endpoint `json:"delete"`
}

// DomainFilter is the response body for the GET / (negotiation) endpoint.
type DomainFilter struct {
	Filters []string `json:"filters"`
	Exclude []string `json:"exclude,omitempty"`
}
