// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"testing"

	core "dappco.re/go"
)

// TestAngularCoverage_waitForZoneStability_Good_PromiseResolvesStable
// covers the path where the Zone-stability promise resolves true directly,
// so no polling fallback is needed.
func TestAngularCoverage_waitForZoneStability_Good_PromiseResolvesStable(t *testing.T) {
	var promiseCalls int
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Errorf("unexpected method %q", msg.Method)
			return
		}
		expr, _ := msg.Params["expression"].(string)
		if core.Contains(expr, "new Promise") {
			promiseCalls++
		}
		target.replyValue(msg.ID, true)
	})

	if err := ah.waitForZoneStability(context.Background()); err != nil {
		t.Fatalf("waitForZoneStability returned error: %v", err)
	}
	if promiseCalls != 1 {
		t.Fatalf("zone-stability promise evaluated %d times, want exactly 1 (no polling fallback)", promiseCalls)
	}
}

// TestAngularCoverage_waitForZoneStability_Ugly_NonBoolFallsBackToPolling
// covers the path where the promise resolves a non-boolean value, forcing
// the polling fallback that ultimately succeeds.
func TestAngularCoverage_waitForZoneStability_Ugly_NonBoolFallsBackToPolling(t *testing.T) {
	var calls int
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		if msg.Method != "Runtime.evaluate" {
			t.Errorf("unexpected method %q", msg.Method)
			return
		}
		calls++
		expr, _ := msg.Params["expression"].(string)
		if core.Contains(expr, "new Promise") {
			// Non-bool result triggers the polling fallback.
			target.replyValue(msg.ID, "not-a-bool")
			return
		}
		// Polling probe reports stable.
		target.replyValue(msg.ID, true)
	})

	if err := ah.waitForZoneStability(context.Background()); err != nil {
		t.Fatalf("waitForZoneStability returned error: %v", err)
	}
	if calls < 2 {
		t.Fatalf("waitForZoneStability made %d evaluate calls, want at least 2 (promise + polling)", calls)
	}
}

// TestAngularCoverage_isAngularApp_Bad_EvaluateError covers the
// evaluate-failure branch of isAngularApp.
func TestAngularCoverage_isAngularApp_Bad_EvaluateError(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		target.replyError(msg.ID, "evaluation crashed")
	})

	isAngular, err := ah.isAngularApp(context.Background())
	if err == nil {
		t.Fatal("isAngularApp returned nil error on evaluate failure")
	}
	if isAngular {
		t.Fatal("isAngularApp returned true despite evaluate failure")
	}
}

// TestAngularCoverage_isAngularApp_Ugly_NonBoolResult covers the path
// where evaluate succeeds but returns a non-boolean value, which the
// helper treats as "not Angular" without erroring.
func TestAngularCoverage_isAngularApp_Ugly_NonBoolResult(t *testing.T) {
	ah, _, _ := newAngularTestHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		target.replyValue(msg.ID, "maybe")
	})

	isAngular, err := ah.isAngularApp(context.Background())
	if err != nil {
		t.Fatalf("isAngularApp returned error on non-bool result: %v", err)
	}
	if isAngular {
		t.Fatal("isAngularApp returned true for a non-bool result")
	}
}
