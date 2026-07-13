package lsp

import (
	"encoding/json"
	"testing"
)

func TestParseCompletionResultSupportsListDocumentationAndTextEdit(t *testing.T) {
	raw := json.RawMessage(`{
		"isIncomplete": false,
		"items": [{
			"label": "Println",
			"detail": "func(a ...any)",
			"documentation": {"kind": "markdown", "value": "Prints a line."},
			"insertTextFormat": 2,
			"textEdit": {
				"newText": "Println(${1:args})$0",
				"range": {
					"start": {"line": 2, "character": 4},
					"end": {"line": 2, "character": 7}
				}
			}
		}]
	}`)

	result, ok := parseCompletionResult(raw, 9, 4, "main.go", 2, 8)
	if !ok {
		t.Fatal("expected completion result to parse")
	}
	if result.BufferID != 9 || result.Generation != 4 || result.Path != "main.go" || result.Line != 2 || result.Col != 8 {
		t.Fatalf("result metadata = %+v", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Documentation != "Prints a line." || item.InsertTextFormat != 2 {
		t.Fatalf("item = %+v", item)
	}
	if item.TextEdit == nil || item.TextEdit.Col != 4 || item.TextEdit.EndCol != 7 ||
		item.TextEdit.NewText != "Println(${1:args})$0" {
		t.Fatalf("text edit = %+v", item.TextEdit)
	}
}

func TestParseCompletionResultSupportsInsertReplaceEdit(t *testing.T) {
	raw := json.RawMessage(`[{
		"label": "value",
		"textEdit": {
			"newText": "value",
			"insert": {
				"start": {"line": 0, "character": 1},
				"end": {"line": 0, "character": 3}
			},
			"replace": {
				"start": {"line": 0, "character": 1},
				"end": {"line": 0, "character": 5}
			}
		}
	}]`)

	result, ok := parseCompletionResult(raw, 1, 0, "main.go", 0, 3)
	if !ok || len(result.Items) != 1 || result.Items[0].TextEdit == nil {
		t.Fatalf("result = %+v, ok = %v", result, ok)
	}
	if result.Items[0].TextEdit.EndCol != 3 {
		t.Fatalf("insert range end = %d, want 3", result.Items[0].TextEdit.EndCol)
	}
}

func TestParseCompletionResultAppliesCompletionListItemDefaults(t *testing.T) {
	raw := json.RawMessage(`{
		"itemDefaults": {
			"insertTextFormat": 2,
			"editRange": {
				"start": {"line": 1, "character": 2},
				"end": {"line": 1, "character": 4}
			}
		},
		"items": [{
			"label": "call",
			"textEditText": "call($0)"
		}]
	}`)

	result, ok := parseCompletionResult(raw, 1, 0, "main.go", 1, 4)
	if !ok || len(result.Items) != 1 {
		t.Fatalf("result = %+v, ok = %v", result, ok)
	}
	item := result.Items[0]
	if item.InsertTextFormat != 2 || item.TextEdit == nil {
		t.Fatalf("item = %+v", item)
	}
	if item.TextEdit.Col != 2 || item.TextEdit.EndCol != 4 || item.TextEdit.NewText != "call($0)" {
		t.Fatalf("text edit = %+v", item.TextEdit)
	}
}
