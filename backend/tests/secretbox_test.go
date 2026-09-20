package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/portico/backend/internal/secretbox"
)

func TestSealOpenRoundTrip(t *testing.T) {
	secretbox.SetKey("test-key")
	raw := []byte(`{"host":"localhost","port":3306,"user":"root","password":"secret","api_key":"xyz"}`)

	sealed, err := secretbox.Seal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("secret")) || bytes.Contains(sealed, []byte("xyz")) {
		t.Fatalf("plaintext secret left in sealed config: %s", sealed)
	}
	if !bytes.Contains(sealed, []byte("localhost")) {
		t.Fatalf("non-secret field should stay readable: %s", sealed)
	}
	if !secretbox.NeedsSeal(raw) {
		t.Fatal("expected plaintext config to need sealing")
	}
	if secretbox.NeedsSeal(sealed) {
		t.Fatal("sealed config should not need sealing")
	}

	opened, err := secretbox.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(opened, []byte(`"password":"secret"`)) {
		t.Fatalf("opened config = %s", opened)
	}
	if !bytes.Contains(opened, []byte(`"port":3306`)) {
		t.Fatalf("port should stay a number: %s", opened)
	}
}

func TestRedactAndMerge(t *testing.T) {
	secretbox.SetKey("test-key")
	existing := []byte(`{"host":"db.internal","password":"secret","api_key":"xyz"}`)

	redacted, err := secretbox.Redact(existing)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(redacted, []byte("secret")) || bytes.Contains(redacted, []byte("xyz")) {
		t.Fatalf("redacted config leaked a secret: %s", redacted)
	}

	merged, err := secretbox.Merge(existing, []byte(`{"host":"other","password":"","api_key":"new-key"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(merged, []byte(`"password":"secret"`)) {
		t.Fatalf("blank password should keep the stored secret: %s", merged)
	}
	if !bytes.Contains(merged, []byte(`"api_key":"new-key"`)) {
		t.Fatalf("new api key should replace the stored one: %s", merged)
	}
	if !bytes.Contains(merged, []byte(`"host":"other"`)) {
		t.Fatalf("non-secret fields should come from the update: %s", merged)
	}
	if strings.Contains(string(merged), "db.internal") {
		t.Fatalf("old host should not survive a full config replace: %s", merged)
	}
}
