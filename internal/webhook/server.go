package webhook

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/xruins/external-dns-technitium-dns/internal/config"
	"github.com/xruins/external-dns-technitium-dns/internal/technitium"
)

const (
	// contentTypeWebhook is the media type expected by the external-dns webhook protocol.
	contentTypeWebhook = "application/external.dns.webhook+json;version=1"
)

// Server is the HTTP handler for the external-dns webhook provider API.
type Server struct {
	cfg    *config.Config
	client *technitium.Client
	mux    *http.ServeMux
}

// NewServer creates and configures a Server with all required routes.
func NewServer(cfg *config.Config, client *technitium.Client) *Server {
	s := &Server{cfg: cfg, client: client}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", s.handleNegotiate)
	mux.HandleFunc("GET /records", s.handleGetRecords)
	mux.HandleFunc("POST /adjustendpoints", s.handleAdjustEndpoints)
	mux.HandleFunc("POST /records", s.handleApplyChanges)
	mux.HandleFunc("GET /healthz", s.handleHealth)

	s.mux = mux
	return s
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Incoming request", "method", r.Method, "path", r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

// handleNegotiate responds to the domain-filter negotiation request from external-dns.
// external-dns calls GET / to discover which domains this provider manages.
func (s *Server) handleNegotiate(w http.ResponseWriter, r *http.Request) {
	filters := s.cfg.DomainFilters
	if len(filters) == 0 {
		filters = []string{s.cfg.Zone}
	}
	writeJSON(w, http.StatusOK, DomainFilter{Filters: filters})
}

// handleGetRecords returns all DNS records currently managed in the Technitium zone.
// Records are grouped by (dnsName, recordType) to match the external-dns Endpoint model.
func (s *Server) handleGetRecords(w http.ResponseWriter, r *http.Request) {
	records, err := s.client.GetAllRecords(r.Context())
	if err != nil {
		slog.Error("Failed to retrieve records from Technitium", "error", err)
		http.Error(w, "failed to retrieve records", http.StatusInternalServerError)
		return
	}

	endpoints := recordsToEndpoints(records)
	writeJSON(w, http.StatusOK, endpoints)
}

// handleAdjustEndpoints is a pass-through; external-dns uses it to validate endpoints
// before applying changes. We return the input unchanged.
func (s *Server) handleAdjustEndpoints(w http.ResponseWriter, r *http.Request) {
	var endpoints []*Endpoint
	if err := json.NewDecoder(r.Body).Decode(&endpoints); err != nil {
		http.Error(w, fmt.Sprintf("decoding request body: %v", err), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, endpoints)
}

// handleApplyChanges processes DNS record creates, updates, and deletes from external-dns.
func (s *Server) handleApplyChanges(w http.ResponseWriter, r *http.Request) {
	var changes Changes
	if err := json.NewDecoder(r.Body).Decode(&changes); err != nil {
		http.Error(w, fmt.Sprintf("decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	handler := &changeHandler{client: s.client}

	// Delete first, then create — avoids conflicts during updates.
	for _, ep := range changes.Delete {
		if err := handler.deleteEndpoint(ctx, ep); err != nil {
			slog.Error("Failed to delete endpoint", "dnsName", ep.DNSName, "type", ep.RecordType, "error", err)
		}
	}

	// For updates: delete the old records, then create the new ones.
	for _, ep := range changes.UpdateOld {
		if err := handler.deleteEndpoint(ctx, ep); err != nil {
			slog.Error("Failed to delete old endpoint during update", "dnsName", ep.DNSName, "type", ep.RecordType, "error", err)
		}
	}

	for _, ep := range changes.Create {
		if err := handler.createEndpoint(ctx, ep, false); err != nil {
			slog.Error("Failed to create endpoint", "dnsName", ep.DNSName, "type", ep.RecordType, "error", err)
		}
	}

	for _, ep := range changes.UpdateNew {
		// Use overwrite=true because we just deleted the corresponding UpdateOld records.
		if err := handler.createEndpoint(ctx, ep, true); err != nil {
			slog.Error("Failed to create new endpoint during update", "dnsName", ep.DNSName, "type", ep.RecordType, "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleHealth responds to Kubernetes liveness/readiness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// recordsToEndpoints converts a flat list of Technitium records into grouped external-dns Endpoints.
// Records sharing the same dnsName and recordType are merged into a single Endpoint with multiple Targets.
func recordsToEndpoints(records []technitium.Record) []*Endpoint {
	type key struct{ name, rtype string }
	index := make(map[key]*Endpoint)
	var order []key

	for _, r := range records {
		// Skip disabled records and internal zone-apex system records.
		if r.Disabled {
			continue
		}
		rtype := strings.ToUpper(r.Type)
		if rtype == "SOA" {
			continue
		}

		target, err := technitium.ExtractTarget(r.Type, r.RData)
		if err != nil {
			slog.Debug("Skipping record with unsupported type", "name", r.Name, "type", r.Type)
			continue
		}

		k := key{name: r.Name, rtype: rtype}
		ep, exists := index[k]
		if !exists {
			ep = &Endpoint{
				DNSName:    r.Name,
				RecordType: rtype,
				RecordTTL:  r.TTL,
			}
			index[k] = ep
			order = append(order, k)
		}
		ep.Targets = append(ep.Targets, target)
	}

	endpoints := make([]*Endpoint, 0, len(order))
	for _, k := range order {
		endpoints = append(endpoints, index[k])
	}
	return endpoints
}

// writeJSON serialises v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", contentTypeWebhook)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}
