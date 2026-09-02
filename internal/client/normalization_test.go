package client

import "testing"

func TestNormalizeDomainAndRecordValues(t *testing.T) {
	t.Parallel()

	domain, err := NormalizeDomain("BÜCHER.Example.")
	if err != nil || domain != "xn--bcher-kva.example" {
		t.Fatalf("NormalizeDomain=%q error=%v", domain, err)
	}
	for _, invalid := range []string{" example.invalid", "example.invalid..", "-bad.example", "bad_.example"} {
		if _, err := NormalizeDomain(invalid); err == nil {
			t.Errorf("NormalizeDomain(%q) succeeded", invalid)
		}
	}

	if got, err := NormalizeRecordRdata("AAAA", "2001:0db8:0:0::1"); err != nil || got != "2001:db8::1" {
		t.Fatalf("AAAA=%q error=%v", got, err)
	}
	if got, err := NormalizeRecordRdata("SRV", "0 5 443 Target.Example."); err != nil || got != "0 5 443 target.example" {
		t.Fatalf("SRV=%q error=%v", got, err)
	}
	if got, err := NormalizeRecordRdata("TXT", "Case And Space. "); err != nil || got != "Case And Space. " {
		t.Fatalf("TXT=%q error=%v", got, err)
	}
}

func TestRecordsEquivalentUsesTypeSpecificNormalization(t *testing.T) {
	t.Parallel()

	desired := CreateRecordRequest{
		Domain: "Example.Invalid.", Host: "WWW", Type: "CNAME", Rdata: "Target.Example.Invalid.", TTL: 300,
	}
	record := Record{
		Domain: "example.invalid", Host: "www", Type: "cname", Rdata: "target.example.invalid", TTL: 300,
	}
	if !RecordsEquivalent(record, desired) {
		t.Fatal("semantically equivalent CNAME records did not match")
	}
	record.Type = "TXT"
	record.Rdata = "Value"
	desired.Type = "TXT"
	desired.Rdata = "value"
	if RecordsEquivalent(record, desired) {
		t.Fatal("opaque TXT values were compared case-insensitively")
	}
}
