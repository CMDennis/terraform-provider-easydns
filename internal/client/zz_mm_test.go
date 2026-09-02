//go:build livecheck

package client

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveListMailmaps(t *testing.T) {
	token, key, domain := os.Getenv("EASYDNS_API_TOKEN"), os.Getenv("EASYDNS_API_KEY"), os.Getenv("FIXTURE_DOMAIN")
	if token == "" || key == "" || domain == "" {
		t.Skip("need credentials")
	}
	c, _ := New(Config{BaseURL: "https://sandbox.rest.easydns.net", Token: token, Key: key, HTTPTimeout: 30 * time.Second})
	maps, err := c.ListMailmaps(context.Background(), domain)
	if err != nil {
		t.Fatalf("ListMailmaps: %v", err)
	}
	t.Logf("mailmaps remaining: %d", len(maps))
	for _, m := range maps {
		t.Logf("  %+v", m)
	}
}
