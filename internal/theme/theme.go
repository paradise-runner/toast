package theme

import "encoding/json"

type Theme struct {
	Name    string                 `json:"name"`
	Variant string                 `json:"variant"`
	UI      map[string]string      `json:"ui"`
	Syntax  map[string]SyntaxStyle `json:"syntax"`
	Git     map[string]string      `json:"git"`
}

type SyntaxStyle struct {
	FG     string `json:"fg,omitempty"`
	BG     string `json:"bg,omitempty"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
}

func parse(data []byte) (*Theme, error) {
	var t Theme
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
