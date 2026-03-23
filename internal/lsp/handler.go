package lsp

import (
	"encoding/json"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/messages"
)

func handleNotification(method string, params json.RawMessage, language string, send func(tea.Msg)) {
	switch method {
	case "textDocument/publishDiagnostics":
		var p PublishDiagnosticsParams
		if err := json.Unmarshal(params, &p); err != nil {
			return
		}
		path := PathFromURI(p.URI)
		var diags []messages.Diagnostic
		for _, d := range p.Diagnostics {
			diags = append(diags, messages.Diagnostic{
				Line:     d.Range.Start.Line,
				Col:      d.Range.Start.Character,
				EndLine:  d.Range.End.Line,
				EndCol:   d.Range.End.Character,
				Severity: int(d.Severity),
				Message:  d.Message,
				Source:   d.Source,
			})
		}
		send(messages.DiagnosticsUpdatedMsg{Path: path, Diagnostics: diags})
	}
}
