// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"testing"
)

func TestCdpURL_parseCoreURL_Good(t *testing.T) {
	u, err := parseCoreURL("http://localhost:9222/json")
	if err != nil {
		t.Fatalf("parseCoreURL returned error: %v", err)
	}
	if u.Scheme != "http" {
		t.Fatalf("Scheme = %q, want http", u.Scheme)
	}
	if u.Hostname() != "localhost" {
		t.Fatalf("Hostname = %q, want localhost", u.Hostname())
	}
	if u.Port() != "9222" {
		t.Fatalf("Port = %q, want 9222", u.Port())
	}
}

func TestCdpURL_parseCoreURL_Bad(t *testing.T) {
	// A control character in the URL forces core.URLParse to fail, exercising
	// the error-propagation branch of parseCoreURL.
	if _, err := parseCoreURL("http://exa\x7fmple.com"); err == nil {
		t.Fatal("parseCoreURL on malformed URL returned nil error")
	}
}

func TestCdpURL_cdpURLFromAny_Good(t *testing.T) {
	source := &cdpURL{Scheme: "http", Host: "localhost:9222"}

	fromPtr, err := cdpURLFromAny(source)
	if err != nil {
		t.Fatalf("cdpURLFromAny(*cdpURL) returned error: %v", err)
	}
	if fromPtr != source {
		t.Fatal("cdpURLFromAny(*cdpURL) should return the same pointer")
	}

	fromValue, err := cdpURLFromAny(*source)
	if err != nil {
		t.Fatalf("cdpURLFromAny(cdpURL) returned error: %v", err)
	}
	if fromValue == nil || fromValue.Host != "localhost:9222" {
		t.Fatalf("cdpURLFromAny(cdpURL) = %+v, want copy with Host localhost:9222", fromValue)
	}
}

func TestCdpURL_cdpURLFromAny_Bad(t *testing.T) {
	if _, err := cdpURLFromAny((*cdpURL)(nil)); err == nil {
		t.Fatal("cdpURLFromAny(nil *cdpURL) returned nil error")
	}
}

func TestCdpURL_cdpURLFromAny_Ugly(t *testing.T) {
	// An int satisfies neither *cdpURL, cdpURL, nor coreParsedURL — the
	// default branch must reject it through cdpURLFromParsed.
	if _, err := cdpURLFromAny(42); err == nil {
		t.Fatal("cdpURLFromAny(int) returned nil error")
	}
}

func TestCdpURL_cdpURLFromParsed_Bad(t *testing.T) {
	if _, err := cdpURLFromParsed("not a parsed url"); err == nil {
		t.Fatal("cdpURLFromParsed(string) returned nil error")
	}
}

func TestCdpURL_urlFieldString_Bad(t *testing.T) {
	// Non-struct input yields an invalid reflect.Value and an empty string
	// rather than a panic.
	if got := urlFieldString(42, "Scheme"); got != "" {
		t.Fatalf("urlFieldString(int) = %q, want empty", got)
	}
	// A real struct with a non-string field returns empty too.
	type sample struct{ Count int }
	if got := urlFieldString(sample{Count: 3}, "Count"); got != "" {
		t.Fatalf("urlFieldString(non-string field) = %q, want empty", got)
	}
}

func TestCdpURL_urlFieldString_Good(t *testing.T) {
	type sample struct{ Scheme string }
	if got := urlFieldString(sample{Scheme: "https"}, "Scheme"); got != "https" {
		t.Fatalf("urlFieldString = %q, want https", got)
	}
}

func TestCdpURL_urlFieldIsSet_Good(t *testing.T) {
	type sample struct {
		Name string
		Ptr  *int
	}
	value := 5
	if !urlFieldIsSet(sample{Name: "x"}, "Name") {
		t.Fatal("urlFieldIsSet on set string field = false, want true")
	}
	if urlFieldIsSet(sample{}, "Name") {
		t.Fatal("urlFieldIsSet on zero string field = true, want false")
	}
	if !urlFieldIsSet(sample{Ptr: &value}, "Ptr") {
		t.Fatal("urlFieldIsSet on non-nil pointer = false, want true")
	}
	if urlFieldIsSet(sample{}, "Ptr") {
		t.Fatal("urlFieldIsSet on nil pointer = true, want false")
	}
}

func TestCdpURL_urlFieldIsSet_Bad(t *testing.T) {
	// Missing field name yields an invalid reflect.Value, reported as not-set.
	type sample struct{ Name string }
	if urlFieldIsSet(sample{Name: "x"}, "Nope") {
		t.Fatal("urlFieldIsSet on absent field = true, want false")
	}
}

func TestCdpURL_urlField_Bad(t *testing.T) {
	if urlField(nil, "Scheme").IsValid() {
		t.Fatal("urlField(nil) returned a valid reflect.Value")
	}
	if urlField((*cdpURL)(nil), "Scheme").IsValid() {
		t.Fatal("urlField(nil *cdpURL) returned a valid reflect.Value")
	}
	if urlField(42, "Scheme").IsValid() {
		t.Fatal("urlField(int) returned a valid reflect.Value")
	}
}

func TestCdpURL_canonicalDebugURL_Good(t *testing.T) {
	got := canonicalDebugURL(&cdpURL{Scheme: "http", Host: "localhost:9222", Path: "/"})
	if got != "http://localhost:9222" {
		t.Fatalf("canonicalDebugURL = %q, want http://localhost:9222", got)
	}
}

func TestCdpURL_canonicalDebugURL_Bad(t *testing.T) {
	// An unsupported input type makes cdpURLFromAny fail; canonicalDebugURL
	// must collapse that to an empty string rather than propagate.
	if got := canonicalDebugURL(42); got != "" {
		t.Fatalf("canonicalDebugURL(int) = %q, want empty", got)
	}
}
