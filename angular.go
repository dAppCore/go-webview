// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"time"

	core "dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// AngularHelper provides Angular-specific testing utilities.
type AngularHelper struct {
	wv      *Webview
	timeout time.Duration
}

// NewAngularHelper creates a new Angular helper for the webview.
func NewAngularHelper(wv *Webview) *AngularHelper {
	return &AngularHelper{
		wv:      wv,
		timeout: 30 * time.Second,
	}
}

// SetTimeout sets the default timeout for Angular operations.
func (ah *AngularHelper) SetTimeout(d time.Duration) {
	ah.timeout = d
}

// WaitForAngular waits for Angular to finish all pending operations.
// This includes HTTP requests, timers, and change detection.
func (ah *AngularHelper) WaitForAngular() error {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	return ah.waitForAngular(ctx)
}

// waitForAngular implements the Angular wait logic.
func (ah *AngularHelper) waitForAngular(ctx context.Context) error {
	// Check if Angular is present
	isAngular, err := ah.isAngularApp(ctx)
	if err != nil {
		return err
	}
	if !isAngular {
		return coreerr.E("AngularHelper.waitForAngular", "not an Angular application", nil)
	}

	// Wait for Zone.js stability
	return ah.waitForZoneStability(ctx)
}

// isAngularApp checks if the current page is an Angular application.
func (ah *AngularHelper) isAngularApp(ctx context.Context) (bool, error) {
	script := `
		(function() {
			// Check for Angular 2+
			if (window.getAllAngularRootElements && window.getAllAngularRootElements().length > 0) {
				return true;
			}
			// Check for Angular CLI generated apps
			if (document.querySelector('[ng-version]')) {
				return true;
			}
			// Check for Angular elements
			if (window.ng && typeof window.ng.probe === 'function') {
				return true;
			}
			// Check for AngularJS (1.x)
			if (window.angular && window.angular.element) {
				return true;
			}
			return false;
		})()
	`

	result, err := ah.wv.evaluate(ctx, script)
	if err != nil {
		return false, err
	}

	isAngular, ok := result.(bool)
	if !ok {
		return false, nil
	}

	return isAngular, nil
}

// waitForZoneStability waits for Zone.js to become stable.
func (ah *AngularHelper) waitForZoneStability(ctx context.Context) error {
	script := `
		new Promise((resolve, reject) => {
			const pollZone = () => {
				if (!window.Zone || !window.Zone.current) {
					resolve(true);
					return;
				}

				const inner = window.Zone.current._inner || window.Zone.current;
				if (!inner._hasPendingMicrotasks && !inner._hasPendingMacrotasks) {
					resolve(true);
					return;
				}

				setTimeout(pollZone, 50);
			};

			// Get the root elements
			const roots = window.getAllAngularRootElements ? window.getAllAngularRootElements() : [];
			if (roots.length === 0) {
				// Try to find root element directly
				const appRoot = document.querySelector('[ng-version]');
				if (appRoot) {
					roots.push(appRoot);
				}
			}

			if (roots.length === 0) {
				resolve(true); // No Angular roots found, nothing to wait for
				return;
			}

			// Get the Zone from any root element
			let zone = null;
			for (const root of roots) {
				try {
					const injector = window.ng.probe(root).injector;
					zone = injector.get(window.ng.coreTokens.NgZone || 'NgZone');
					if (zone) break;
				} catch (e) {
					// Continue to next root
				}
			}

			if (!zone) {
				pollZone();
				return;
			}

			// Use Angular's zone stability
			if (zone.isStable) {
				resolve(true);
				return;
			}

			// Wait for stability
			try {
				const sub = zone.onStable.subscribe(() => {
					sub.unsubscribe();
					resolve(true);
				});
			} catch (e) {
				pollZone();
			}
		})
	`

	result, err := ah.wv.evaluate(ctx, script)
	if err != nil {
		// If the script fails, fall back to simple polling
		return ah.pollForStability(ctx)
	}

	if stable, ok := result.(bool); ok && stable {
		return nil
	}

	return ah.pollForStability(ctx)
}

// pollForStability polls for Angular stability as a fallback.
func (ah *AngularHelper) pollForStability(ctx context.Context) error {
	script := `
		(function() {
			if (window.Zone && window.Zone.current) {
				const inner = window.Zone.current._inner || window.Zone.current;
				return !inner._hasPendingMicrotasks && !inner._hasPendingMacrotasks;
			}
			return true;
		})()
	`

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			result, err := ah.wv.evaluate(ctx, script)
			if err != nil {
				continue
			}
			if stable, ok := result.(bool); ok && stable {
				return nil
			}
		}
	}
}

// NavigateByRouter navigates using Angular Router.
func (ah *AngularHelper) NavigateByRouter(path string) error {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := core.Sprintf(`
		(function() {
			const roots = window.getAllAngularRootElements ? window.getAllAngularRootElements() : [];
			if (roots.length === 0) {
				throw new Error('No Angular root elements found');
			}

			for (const root of roots) {
				try {
					const injector = window.ng.probe(root).injector;
					const router = injector.get(window.ng.coreTokens.Router || 'Router');
					if (router) {
						router.navigateByUrl(%q);
						return true;
					}
				} catch (e) {
					continue;
				}
			}
			throw new Error('Could not find Angular Router');
		})()
	`, path)

	_, err := ah.wv.evaluate(ctx, script)
	if err != nil {
		return coreerr.E("AngularHelper.NavigateByRouter", "failed to navigate", err)
	}

	// Wait for navigation to complete
	return ah.waitForZoneStability(ctx)
}

// GetRouterState returns the current Angular router state.
func (ah *AngularHelper) GetRouterState() (*AngularRouterState, error) {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := `
		(function() {
			const roots = window.getAllAngularRootElements ? window.getAllAngularRootElements() : [];
			for (const root of roots) {
				try {
					const injector = window.ng.probe(root).injector;
					const router = injector.get(window.ng.coreTokens.Router || 'Router');
					if (router) {
						return {
							url: router.url,
							fragment: router.routerState.root.fragment,
							params: router.routerState.root.params,
							queryParams: router.routerState.root.queryParams
						};
					}
				} catch (e) {
					continue;
				}
			}
			return null;
		})()
	`

	result, err := ah.wv.evaluate(ctx, script)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, coreerr.E("AngularHelper.GetRouterState", "could not get router state", nil)
	}

	// Parse result
	resultMap, ok := result.(map[string]any)
	if !ok {
		return nil, coreerr.E("AngularHelper.GetRouterState", "invalid router state format", nil)
	}

	state := &AngularRouterState{
		URL: getString(resultMap, "url"),
	}

	if fragment, ok := resultMap["fragment"].(string); ok {
		state.Fragment = fragment
	}

	if params, ok := resultMap["params"].(map[string]any); ok {
		state.Params = make(map[string]string)
		for k, v := range params {
			if s, ok := v.(string); ok {
				state.Params[k] = s
			}
		}
	}

	if queryParams, ok := resultMap["queryParams"].(map[string]any); ok {
		state.QueryParams = make(map[string]string)
		for k, v := range queryParams {
			if s, ok := v.(string); ok {
				state.QueryParams[k] = s
			}
		}
	}

	return state, nil
}

// AngularRouterState represents Angular router state.
type AngularRouterState struct {
	URL         string            `json:"url"`
	Fragment    string            `json:"fragment,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
}

// GetComponentProperty gets a property from an Angular component.
func (ah *AngularHelper) GetComponentProperty(selector, propertyName string) (any, error) {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := core.Sprintf(`
			(function() {
				const selector = %s;
				const propertyName = %s;
				const element = document.querySelector(selector);
				if (!element) {
					throw new Error('Element not found: ' + selector);
				}
				const component = window.ng.probe(element).componentInstance;
				if (!component) {
					throw new Error('No Angular component found on element');
				}
				return component[propertyName];
			})()
		`, formatJSValue(selector), formatJSValue(propertyName))

	return ah.wv.evaluate(ctx, script)
}

// SetComponentProperty sets a property on an Angular component.
func (ah *AngularHelper) SetComponentProperty(selector, propertyName string, value any) error {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := core.Sprintf(`
			(function() {
				const selector = %s;
				const propertyName = %s;
				const element = document.querySelector(selector);
				if (!element) {
					throw new Error('Element not found: ' + selector);
				}
				const component = window.ng.probe(element).componentInstance;
				if (!component) {
					throw new Error('No Angular component found on element');
				}
				component[propertyName] = %s;

				// Trigger change detection
				const injector = window.ng.probe(element).injector;
				const appRef = injector.get(window.ng.coreTokens.ApplicationRef || 'ApplicationRef');
				if (appRef) {
				appRef.tick();
				}
				return true;
			})()
		`, formatJSValue(selector), formatJSValue(propertyName), formatJSValue(value))

	_, err := ah.wv.evaluate(ctx, script)
	return err
}

// CallComponentMethod calls a method on an Angular component.
func (ah *AngularHelper) CallComponentMethod(selector, methodName string, args ...any) (any, error) {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	argsStr := core.NewBuilder()
	for i, arg := range args {
		if i > 0 {
			argsStr.WriteString(", ")
		}
		argsStr.WriteString(formatJSValue(arg))
	}

	script := core.Sprintf(`
			(function() {
				const selector = %s;
				const methodName = %s;
				const element = document.querySelector(selector);
				if (!element) {
					throw new Error('Element not found: ' + selector);
				}
				const component = window.ng.probe(element).componentInstance;
				if (!component) {
					throw new Error('No Angular component found on element');
				}
				if (typeof component[methodName] !== 'function') {
					throw new Error('Method not found: ' + methodName);
				}
				const result = component[methodName](%s);

				// Trigger change detection
				const injector = window.ng.probe(element).injector;
				const appRef = injector.get(window.ng.coreTokens.ApplicationRef || 'ApplicationRef');
				if (appRef) {
				appRef.tick();
				}
				return result;
			})()
		`, formatJSValue(selector), formatJSValue(methodName), argsStr.String())

	return ah.wv.evaluate(ctx, script)
}

// TriggerChangeDetection manually triggers Angular change detection.
func (ah *AngularHelper) TriggerChangeDetection() error {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := `
		(function() {
			const roots = window.getAllAngularRootElements ? window.getAllAngularRootElements() : [];
			for (const root of roots) {
				try {
					const injector = window.ng.probe(root).injector;
					const appRef = injector.get(window.ng.coreTokens.ApplicationRef || 'ApplicationRef');
					if (appRef) {
						appRef.tick();
						return true;
					}
				} catch (e) {
					continue;
				}
			}
			return false;
		})()
	`

	_, err := ah.wv.evaluate(ctx, script)
	return err
}

// GetService gets an Angular service by token name.
func (ah *AngularHelper) GetService(serviceName string) (any, error) {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := core.Sprintf(`
		(function() {
			const roots = window.getAllAngularRootElements ? window.getAllAngularRootElements() : [];
			for (const root of roots) {
				try {
					const injector = window.ng.probe(root).injector;
					const service = injector.get(%q);
					if (service) {
						// Return a serializable representation
						return JSON.parse(JSON.stringify(service));
					}
				} catch (e) {
					continue;
				}
			}
			return null;
		})()
	`, serviceName)

	return ah.wv.evaluate(ctx, script)
}

// WaitForComponent waits for an Angular component to be present.
func (ah *AngularHelper) WaitForComponent(selector string) error {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := core.Sprintf(`
		(function() {
			const element = document.querySelector(%q);
			if (!element) return false;
			try {
				const component = window.ng.probe(element).componentInstance;
				return !!component;
			} catch (e) {
				return false;
			}
		})()
	`, selector)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			result, err := ah.wv.evaluate(ctx, script)
			if err != nil {
				continue
			}
			if found, ok := result.(bool); ok && found {
				return nil
			}
		}
	}
}

// DispatchEvent dispatches a custom event on an element.
func (ah *AngularHelper) DispatchEvent(selector, eventName string, detail any) error {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	detailStr := "null"
	if detail != nil {
		detailStr = formatJSValue(detail)
	}

	script := core.Sprintf(`
			(function() {
				const selector = %s;
				const eventName = %s;
				const element = document.querySelector(selector);
				if (!element) {
					throw new Error('Element not found: ' + selector);
				}
				const event = new CustomEvent(eventName, { bubbles: true, detail: %s });
				element.dispatchEvent(event);
				return true;
			})()
		`, formatJSValue(selector), formatJSValue(eventName), detailStr)

	_, err := ah.wv.evaluate(ctx, script)
	return err
}

// GetNgModel gets the value of an ngModel-bound input.
func (ah *AngularHelper) GetNgModel(selector string) (any, error) {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := core.Sprintf(`
		(function() {
			const element = document.querySelector(%q);
			if (!element) return null;

			// Try to get from component
			try {
				const debug = window.ng.probe(element);
				const component = debug.componentInstance;
				// Look for common ngModel patterns
				if (element.tagName === 'INPUT' || element.tagName === 'SELECT' || element.tagName === 'TEXTAREA') {
					return element.value;
				}
			} catch (e) {}

			return element.value || element.textContent;
		})()
	`, selector)

	return ah.wv.evaluate(ctx, script)
}

// SetNgModel sets the value of an ngModel-bound input.
func (ah *AngularHelper) SetNgModel(selector string, value any) error {
	ctx, cancel := context.WithTimeout(ah.wv.ctx, ah.timeout)
	defer cancel()

	script := core.Sprintf(`
			(function() {
				const selector = %s;
				const element = document.querySelector(selector);
				if (!element) {
					throw new Error('Element not found: ' + selector);
				}

				element.value = %s;
				element.dispatchEvent(new Event('input', { bubbles: true }));
				element.dispatchEvent(new Event('change', { bubbles: true }));

				// Trigger change detection
			const roots = window.getAllAngularRootElements ? window.getAllAngularRootElements() : [];
			for (const root of roots) {
				try {
					const injector = window.ng.probe(root).injector;
					const appRef = injector.get(window.ng.coreTokens.ApplicationRef || 'ApplicationRef');
					if (appRef) {
						appRef.tick();
						break;
					}
				} catch (e) {}
			}

				return true;
			})()
		`, formatJSValue(selector), formatJSValue(value))

	_, err := ah.wv.evaluate(ctx, script)
	return err
}

// Helper functions

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func formatJSValue(v any) string {
	r := core.JSONMarshal(v)
	if r.OK {
		return string(r.Value.([]byte))
	}

	r = core.JSONMarshal(core.Sprint(v))
	if r.OK {
		return string(r.Value.([]byte))
	}

	return "null"
}
