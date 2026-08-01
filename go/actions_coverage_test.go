// SPDX-License-Identifier: EUPL-1.2
package webview

import (
	"context"
	"testing"
)

// uploadDragHarness drives the real UploadFileAction / DragAndDropAction
// Execute methods against a fake CDP target, mirroring newActionHarness.
//
//	wv, target := uploadDragHarness(t, func(tgt *fakeCDPTarget, msg cdpMessage) { ... })
func uploadDragHarness(t *testing.T, onMessage func(*fakeCDPTarget, cdpMessage)) (*Webview, *fakeCDPTarget) {
	t.Helper()
	return newActionHarness(t, onMessage)
}

func TestActionsCoverage_UploadFileAction_Execute_Good(t *testing.T) {
	wv, _ := uploadDragHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(22)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "INPUT"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-5"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			target.reply(msg.ID, map[string]any{"model": map[string]any{"content": []any{float64(0), float64(0), float64(1), float64(0), float64(1), float64(1), float64(0), float64(1)}}})
		case "DOM.setFileInputFiles":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Errorf("unexpected method %q", msg.Method)
		}
	})

	action := UploadFileAction{Selector: "#file", FilePaths: []string{"/tmp/a.txt"}}
	if err := action.Execute(context.Background(), wv); err != nil {
		t.Fatalf("UploadFileAction.Execute returned error: %v", err)
	}
}

func TestActionsCoverage_UploadFileAction_Execute_Bad(t *testing.T) {
	action := UploadFileAction{Selector: "#file", FilePaths: []string{"/tmp/a.txt"}}
	err := action.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("UploadFileAction.Execute with nil webview returned nil error")
	}
}

func TestActionsCoverage_UploadFileAction_Execute_Ugly(t *testing.T) {
	// querySelector resolves the document but never finds the file input —
	// the action must surface the lookup failure rather than panic.
	wv, _ := uploadDragHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.replyError(msg.ID, "no such node")
		default:
			target.replyError(msg.ID, "unexpected method")
		}
	})

	action := UploadFileAction{Selector: "#missing", FilePaths: []string{"/tmp/a.txt"}}
	if err := action.Execute(context.Background(), wv); err == nil {
		t.Fatal("UploadFileAction.Execute on missing input returned nil error")
	}
}

func TestActionsCoverage_DragAndDropAction_Execute_Good(t *testing.T) {
	wv, _ := uploadDragHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			sel, _ := msg.Params["selector"].(string)
			switch sel {
			case "#source":
				target.reply(msg.ID, map[string]any{"nodeId": float64(31)})
			case "#target":
				target.reply(msg.ID, map[string]any{"nodeId": float64(32)})
			default:
				t.Errorf("unexpected selector %q", sel)
			}
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-6"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			nodeID := int(msg.Params["nodeId"].(float64))
			box := []any{float64(nodeID), float64(nodeID), float64(nodeID + 10), float64(nodeID), float64(nodeID + 10), float64(nodeID + 10), float64(nodeID), float64(nodeID + 10)}
			target.reply(msg.ID, map[string]any{"model": map[string]any{"content": box}})
		case "Input.dispatchMouseEvent":
			target.reply(msg.ID, map[string]any{})
		default:
			t.Errorf("unexpected method %q", msg.Method)
		}
	})

	action := DragAndDropAction{SourceSelector: "#source", TargetSelector: "#target"}
	if err := action.Execute(context.Background(), wv); err != nil {
		t.Fatalf("DragAndDropAction.Execute returned error: %v", err)
	}
}

func TestActionsCoverage_DragAndDropAction_Execute_Bad(t *testing.T) {
	action := DragAndDropAction{SourceSelector: "#source", TargetSelector: "#target"}
	err := action.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("DragAndDropAction.Execute with nil webview returned nil error")
	}
}

func TestActionsCoverage_DragAndDropAction_Execute_Ugly(t *testing.T) {
	// Source element resolves but reports no bounding box — drag must abort
	// with an error rather than dereferencing a nil box.
	wv, _ := uploadDragHarness(t, func(target *fakeCDPTarget, msg cdpMessage) {
		switch msg.Method {
		case "DOM.getDocument":
			target.reply(msg.ID, map[string]any{"root": map[string]any{"nodeId": float64(1)}})
		case "DOM.querySelector":
			target.reply(msg.ID, map[string]any{"nodeId": float64(31)})
		case "DOM.describeNode":
			target.reply(msg.ID, map[string]any{"node": map[string]any{"nodeName": "DIV"}})
		case "DOM.resolveNode":
			target.reply(msg.ID, map[string]any{"object": map[string]any{"objectId": "obj-6"}})
		case "Runtime.callFunctionOn":
			target.reply(msg.ID, map[string]any{"result": map[string]any{"value": map[string]any{"innerHTML": "", "innerText": ""}}})
		case "DOM.getBoxModel":
			target.replyError(msg.ID, "could not compute box model")
		default:
			target.replyError(msg.ID, "unexpected method")
		}
	})

	action := DragAndDropAction{SourceSelector: "#source", TargetSelector: "#target"}
	if err := action.Execute(context.Background(), wv); err == nil {
		t.Fatal("DragAndDropAction.Execute with no bounding box returned nil error")
	}
}

func TestActionsCoverage_ActionSequence_UploadFile_Good(t *testing.T) {
	seq := NewActionSequence().UploadFile("#file", []string{"/tmp/a.txt", "/tmp/b.txt"})

	if len(seq.actions) != 1 {
		t.Fatalf("sequence has %d actions, want 1", len(seq.actions))
	}
	action, ok := seq.actions[0].(UploadFileAction)
	if !ok {
		t.Fatalf("queued action is %T, want UploadFileAction", seq.actions[0])
	}
	if action.Selector != "#file" || len(action.FilePaths) != 2 {
		t.Fatalf("queued UploadFileAction = %+v, want #file with 2 paths", action)
	}
}

func TestActionsCoverage_ActionSequence_UploadFile_Ugly(t *testing.T) {
	// The builder copies the supplied slice; mutating the caller's slice
	// afterwards must not change the queued action.
	paths := []string{"/tmp/a.txt"}
	seq := NewActionSequence().UploadFile("#file", paths)
	paths[0] = "/tmp/mutated.txt"

	action := seq.actions[0].(UploadFileAction)
	if action.FilePaths[0] != "/tmp/a.txt" {
		t.Fatalf("queued path = %q, want /tmp/a.txt (builder must copy)", action.FilePaths[0])
	}
}

func TestActionsCoverage_ActionSequence_DragAndDrop_Good(t *testing.T) {
	seq := NewActionSequence().DragAndDrop("#source", "#target")

	if len(seq.actions) != 1 {
		t.Fatalf("sequence has %d actions, want 1", len(seq.actions))
	}
	action, ok := seq.actions[0].(DragAndDropAction)
	if !ok {
		t.Fatalf("queued action is %T, want DragAndDropAction", seq.actions[0])
	}
	if action.SourceSelector != "#source" || action.TargetSelector != "#target" {
		t.Fatalf("queued DragAndDropAction = %+v, want #source/#target", action)
	}
}
