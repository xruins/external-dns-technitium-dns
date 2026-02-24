package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/xruins/external-dns-technitium-dns/internal/technitium"
)

// changeHandler processes individual endpoint create/delete operations.
type changeHandler struct {
	client *technitium.Client
}

// buildComment constructs the record comment that identifies this webhook as the record owner.
// The "resource" field is populated from the external-dns/resource label when available.
func buildComment(ep *Endpoint) string {
	resource := ep.DNSName
	if ep.Labels != nil {
		if r, ok := ep.Labels["external-dns/resource"]; ok && r != "" {
			resource = r
		}
	}
	return fmt.Sprintf("%s (resource: %s)", technitium.CommentPrefix, resource)
}

// isOwnedRecord returns true when the record's comment indicates it was created by this webhook.
func isOwnedRecord(r technitium.Record) bool {
	return strings.HasPrefix(r.Comments, technitium.CommentPrefix)
}

// isAddressRecord returns true for A and AAAA record types.
func isAddressRecord(recordType string) bool {
	t := strings.ToUpper(recordType)
	return t == "A" || t == "AAAA"
}

// createEndpoint adds all targets of the endpoint as individual DNS records.
// When overwrite is false the Technitium server will reject any target that already exists.
func (h *changeHandler) createEndpoint(ctx context.Context, ep *Endpoint, overwrite bool) error {
	comment := buildComment(ep)
	var errs []string
	for _, target := range ep.Targets {
		if err := h.client.AddRecord(ctx, ep.DNSName, ep.RecordType, target, ep.RecordTTL, comment, overwrite); err != nil {
			errs = append(errs, err.Error())
		} else {
			slog.Info("Created DNS record", "name", ep.DNSName, "type", ep.RecordType, "target", target)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("creating records for %s %s: %s", ep.RecordType, ep.DNSName, strings.Join(errs, "; "))
	}
	return nil
}

// deleteEndpoint removes all targets of the endpoint from Technitium.
// Before each deletion the existing record is fetched and validated:
//   - Records whose type is neither A nor AAAA are skipped with a warning.
//   - Records that were not created by this webhook (missing comment prefix) are skipped.
func (h *changeHandler) deleteEndpoint(ctx context.Context, ep *Endpoint) error {
	var errs []string
	for _, target := range ep.Targets {
		if err := h.deleteSingleRecord(ctx, ep.DNSName, ep.RecordType, target); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("deleting records for %s %s: %s", ep.RecordType, ep.DNSName, strings.Join(errs, "; "))
	}
	return nil
}

// deleteSingleRecord validates ownership and record type before deleting one DNS record.
func (h *changeHandler) deleteSingleRecord(ctx context.Context, domain, recordType, target string) error {
	// Fetch the existing records so we can inspect their metadata.
	existing, err := h.client.GetRecords(ctx, domain, recordType)
	if err != nil {
		return fmt.Errorf("fetching existing %s record %q before deletion: %w", recordType, domain, err)
	}

	// Find the specific record matching this target.
	matched := findRecord(existing, recordType, target)
	if matched == nil {
		slog.Warn("Record not found in Technitium; skipping deletion",
			"name", domain, "type", recordType, "target", target)
		return nil
	}

	// Safety: only delete A and AAAA records through this path; other types require
	// explicit operator action to prevent accidental data loss.
	if !isAddressRecord(matched.Type) {
		slog.Warn("Skipping deletion: record type is not A or AAAA",
			"name", domain, "type", matched.Type, "target", target)
		return nil
	}

	// Ownership: only delete records that this webhook created.
	if !isOwnedRecord(*matched) {
		slog.Warn("Skipping deletion: record was not created by this webhook",
			"name", domain, "type", matched.Type, "comment", matched.Comments)
		return nil
	}

	if err := h.client.DeleteRecord(ctx, domain, recordType, target); err != nil {
		return err
	}
	slog.Info("Deleted DNS record", "name", domain, "type", recordType, "target", target)
	return nil
}

// findRecord searches a list of Technitium records for one matching the given type and target value.
func findRecord(records []technitium.Record, recordType, target string) *technitium.Record {
	for i := range records {
		r := &records[i]
		if !strings.EqualFold(r.Type, recordType) {
			continue
		}
		extractedTarget, err := technitium.ExtractTarget(r.Type, r.RData)
		if err != nil {
			continue
		}
		if extractedTarget == target {
			return r
		}
	}
	return nil
}
