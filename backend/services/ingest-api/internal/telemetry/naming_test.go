package telemetry

import "testing"

func TestNormalizeOperationalIDs(t *testing.T) {
	siteID, rackID, minerID, err := NormalizeOperationalIDs("SITE-CL-1", "rack-a1", "ASIC_42")
	if err != nil {
		t.Fatalf("expected normalization success, got %v", err)
	}

	if siteID != "site-cl-01" {
		t.Fatalf("unexpected site_id %s", siteID)
	}

	if rackID != "rack-cl-01-01" {
		t.Fatalf("unexpected rack_id %s", rackID)
	}

	if minerID != "asic-000042" {
		t.Fatalf("unexpected miner_id %s", minerID)
	}
}

func TestNormalizeSiteIDRejectsInvalidCountry(t *testing.T) {
	_, err := NormalizeSiteID("site-123-1")
	if err == nil {
		t.Fatal("expected error for invalid country token")
	}
}

func TestNormalizeRackIDUsesSiteContext(t *testing.T) {
	rackID, err := NormalizeRackID("site-cl-03", "r-9")
	if err != nil {
		t.Fatalf("expected rack normalization success, got %v", err)
	}

	if rackID != "rack-cl-03-09" {
		t.Fatalf("unexpected rack_id %s", rackID)
	}
}

func TestNormalizeMinerIDRejectsMissingNumber(t *testing.T) {
	_, err := NormalizeMinerID("asic-foo")
	if err == nil {
		t.Fatal("expected error when miner id has no numeric segment")
	}
}
