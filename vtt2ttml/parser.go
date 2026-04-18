package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ---- Data model ----

// CueStyle holds rendering attributes for a voice.
type CueStyle struct {
	Color  string // "#RRGGBB" or other CSS colour
	Bold   bool
	Italic bool
}

// CueSettings holds the parsed cue header settings.
type CueSettings struct {
	Line        float64 // percentage; -1 = not set
	Position    float64
	PositionSet bool
	Size        float64
	SizeSet     bool
	Align       string // "left"|"center"|"right"|"start"|"end"|""
}

// Node is the interface for cue text AST nodes.
type Node interface{ nodeKind() string }

// TextNode holds a plain text run (may contain literal newlines).
type TextNode struct{ Text string }

func (t *TextNode) nodeKind() string { return "text" }

// TagNode holds a parsed inline tag and its children.
type TagNode struct {
	Tag      string   // "v", "i", "b", "u", "c", "ruby", "rt", "lang"
	Voice    string   // populated for Tag=="v"
	Classes  []string // populated for Tag=="c"
	Children []Node
}

func (t *TagNode) nodeKind() string { return "tag" }

// Cue represents one subtitle entry.
type Cue struct {
	ID       string
	Start    time.Duration
	End      time.Duration
	Settings CueSettings
	Nodes    []Node
}

// VTTFile is the complete parsed representation.
type VTTFile struct {
	Lang    string
	Styles  map[string]CueStyle // voice name → style
	Classes map[string]CueStyle // class name → style (for ::cue(.className))
	Notes   []string            // NOTE block bodies
	Cues    []Cue
}

// ---- Regexps ----

var (
	reCueVoice = regexp.MustCompile(`(?s)::cue\(\s*v\[voice="([^"]+)"\]\s*\)\s*\{([^}]+)\}`)
	reCueClass = regexp.MustCompile(`(?s)::cue\(\s*\.([A-Za-z0-9_\-]+)\s*\)\s*\{([^}]+)\}`)
	reHexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{3,8}$`)
	reBlank    = regexp.MustCompile(`\n{2,}`)
)

var cssNamedColors = map[string]string{
	"white":   "#FFFFFF",
	"black":   "#000000",
	"red":     "#FF0000",
	"green":   "#008000",
	"blue":    "#0000FF",
	"yellow":  "#FFFF00",
	"cyan":    "#00FFFF",
	"aqua":    "#00FFFF",
	"magenta": "#FF00FF",
	"fuchsia": "#FF00FF",
	"gray":    "#808080",
	"grey":    "#808080",
	"silver":  "#C0C0C0",
	"lime":    "#00FF00",
	"orange":  "#FFA500",
	"purple":  "#800080",
	"pink":    "#FFC0CB",
	"brown":   "#A52A2A",
	"navy":    "#000080",
	"teal":    "#008080",
	"olive":   "#808000",
	"maroon":  "#800000",
	"gold":    "#FFD700",
	"indigo":  "#4B0082",
	"violet":  "#EE82EE",
	"coral":   "#FF7F50",
	"salmon":  "#FA8072",
	"khaki":   "#F0E68C",
}

// ---- Public entry point ----

// ParseVTT parses a WebVTT document and returns the structured representation.
func ParseVTT(data string) (*VTTFile, error) {
	// Strip UTF-8 BOM.
	data = strings.TrimPrefix(data, "\xEF\xBB\xBF")
	// Normalise line endings.
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")

	blocks := splitBlocks(data)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	// Verify header.
	header := strings.TrimSpace(blocks[0])
	if !strings.HasPrefix(header, "WEBVTT") {
		return nil, fmt.Errorf("not a WebVTT file: missing WEBVTT header")
	}

	vtt := &VTTFile{
		Lang:    "en",
		Styles:  make(map[string]CueStyle),
		Classes: make(map[string]CueStyle),
	}

	for _, block := range blocks[1:] {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		switch {
		case strings.HasPrefix(block, "NOTE"):
			// Preserve note body (everything after the first line).
			body := strings.TrimPrefix(block, "NOTE")
			body = strings.TrimSpace(body)
			if body != "" {
				vtt.Notes = append(vtt.Notes, body)
			}
		case strings.HasPrefix(block, "STYLE"):
			css := strings.TrimPrefix(block, "STYLE")
			css = strings.TrimSpace(css)
			parseStyleBlock(css, vtt.Styles, vtt.Classes)
		default:
			// Check if this block contains a timing arrow.
			if strings.Contains(block, " --> ") {
				if err := parseCue(block, &vtt.Cues); err != nil {
					// Skip malformed cues rather than aborting.
					continue
				}
			}
			// Otherwise it's an unknown metadata block; ignore.
		}
	}

	return vtt, nil
}

// splitBlocks splits the document on one or more consecutive blank lines.
func splitBlocks(data string) []string {
	return reBlank.Split(data, -1)
}

// ---- Style parsing ----

func parseStyleBlock(css string, styles, classes map[string]CueStyle) {
	for _, m := range reCueVoice.FindAllStringSubmatch(css, -1) {
		styles[m[1]] = parseCSSProperties(m[2])
	}
	for _, m := range reCueClass.FindAllStringSubmatch(css, -1) {
		classes[m[1]] = parseCSSProperties(m[2])
	}
}

func parseCSSProperties(css string) CueStyle {
	var s CueStyle
	for _, decl := range strings.Split(css, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		idx := strings.IndexByte(decl, ':')
		if idx < 0 {
			continue
		}
		prop := strings.TrimSpace(decl[:idx])
		val := strings.TrimSpace(decl[idx+1:])
		switch strings.ToLower(prop) {
		case "color":
			s.Color = normalizeColor(val)
		case "font-weight":
			if strings.ToLower(val) == "bold" {
				s.Bold = true
			}
		case "font-style":
			if strings.ToLower(val) == "italic" {
				s.Italic = true
			}
		}
	}
	return s
}

func normalizeColor(c string) string {
	lower := strings.ToLower(strings.TrimSpace(c))
	if named, ok := cssNamedColors[lower]; ok {
		return named
	}
	return c // pass through hex or other values as-is
}

// ---- Cue parsing ----

func parseCue(block string, cues *[]Cue) error {
	lines := strings.Split(block, "\n")
	i := 0

	var cueID string
	// If the first line doesn't contain " --> ", it's a cue ID.
	if !strings.Contains(lines[i], " --> ") {
		cueID = strings.TrimSpace(lines[i])
		i++
		if i >= len(lines) {
			return fmt.Errorf("cue block has ID but no timing line")
		}
	}

	start, end, settings, err := parseTimingLine(lines[i])
	if err != nil {
		return fmt.Errorf("parsing timing: %w", err)
	}
	i++

	// Join remaining lines as cue text.
	cueText := strings.Join(lines[i:], "\n")

	nodes := parseCueText(cueText)

	*cues = append(*cues, Cue{
		ID:       cueID,
		Start:    start,
		End:      end,
		Settings: settings,
		Nodes:    nodes,
	})
	return nil
}

func parseTimingLine(line string) (start, end time.Duration, settings CueSettings, err error) {
	settings.Line = -1

	parts := strings.SplitN(line, " --> ", 2)
	if len(parts) != 2 {
		err = fmt.Errorf("no --> found in: %q", line)
		return
	}

	startStr := strings.TrimSpace(parts[0])
	rest := strings.TrimSpace(parts[1])

	// rest may be "HH:MM:SS.mmm [settings...]"
	endAndSettings := strings.Fields(rest)
	if len(endAndSettings) == 0 {
		err = fmt.Errorf("no end timestamp in: %q", line)
		return
	}
	endStr := endAndSettings[0]
	settingsStr := ""
	if len(endAndSettings) > 1 {
		settingsStr = strings.Join(endAndSettings[1:], " ")
	}

	start, err = parseTimestamp(startStr)
	if err != nil {
		return
	}
	end, err = parseTimestamp(endStr)
	if err != nil {
		return
	}
	settings = parseCueSettings(settingsStr)
	return
}

func parseTimestamp(ts string) (time.Duration, error) {
	parts := strings.Split(ts, ":")
	var h, m int
	var secStr string

	switch len(parts) {
	case 2:
		// MM:SS.mmm
		var err error
		m, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp minutes %q: %w", parts[0], err)
		}
		secStr = parts[1]
	case 3:
		// HH:MM:SS.mmm
		var err error
		h, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp hours %q: %w", parts[0], err)
		}
		m, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp minutes %q: %w", parts[1], err)
		}
		secStr = parts[2]
	default:
		return 0, fmt.Errorf("unexpected timestamp format: %q", ts)
	}

	secParts := strings.SplitN(secStr, ".", 2)
	sec, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp seconds %q: %w", secParts[0], err)
	}
	ms := 0
	if len(secParts) == 2 {
		msStr := secParts[1]
		// Pad or truncate to 3 digits.
		for len(msStr) < 3 {
			msStr += "0"
		}
		if len(msStr) > 3 {
			msStr = msStr[:3]
		}
		ms, err = strconv.Atoi(msStr)
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp milliseconds %q: %w", secParts[1], err)
		}
	}

	d := time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(ms)*time.Millisecond
	return d, nil
}

func parseCueSettings(s string) CueSettings {
	cs := CueSettings{Line: -1}
	if s == "" {
		return cs
	}
	for _, token := range strings.Fields(s) {
		idx := strings.IndexByte(token, ':')
		if idx < 0 {
			continue
		}
		key := token[:idx]
		val := token[idx+1:]
		switch key {
		case "line":
			// Strip snap-to-lines suffix (e.g. "90%,start").
			val = strings.SplitN(val, ",", 2)[0]
			if strings.HasSuffix(val, "%") {
				f, err := strconv.ParseFloat(val[:len(val)-1], 64)
				if err == nil {
					cs.Line = f
				}
			}
			// Integer line numbers are ignored (no direct TTML equivalent).
		case "position":
			val = strings.SplitN(val, ",", 2)[0]
			if strings.HasSuffix(val, "%") {
				f, err := strconv.ParseFloat(val[:len(val)-1], 64)
				if err == nil {
					cs.Position = f
					cs.PositionSet = true
				}
			}
		case "size":
			if strings.HasSuffix(val, "%") {
				f, err := strconv.ParseFloat(val[:len(val)-1], 64)
				if err == nil {
					cs.Size = f
					cs.SizeSet = true
				}
			}
		case "align":
			cs.Align = val
		}
	}
	return cs
}

// ---- Cue text tokeniser ----

func parseCueText(text string) []Node {
	pos := 0
	nodes := parseNodes(text, &pos)
	return stripOuterNewlines(nodes)
}

// parseNodes parses a sequence of nodes until EOF or a closing tag.
// closingTag is the tag name we're looking for to stop (empty = stop at EOF only).
func parseNodes(text string, pos *int) []Node {
	var nodes []Node
	var textBuf strings.Builder

	flushText := func() {
		if textBuf.Len() > 0 {
			nodes = append(nodes, &TextNode{Text: unescapeHTML(textBuf.String())})
			textBuf.Reset()
		}
	}

	for *pos < len(text) {
		ch := text[*pos]
		if ch != '<' {
			textBuf.WriteByte(ch)
			*pos++
			continue
		}

		// Find the end of this tag.
		end := strings.IndexByte(text[*pos:], '>')
		if end < 0 {
			// No closing >; treat rest as text.
			textBuf.WriteString(text[*pos:])
			*pos = len(text)
			break
		}
		tagBody := text[*pos+1 : *pos+end] // contents between < and >
		*pos += end + 1

		if strings.HasPrefix(tagBody, "/") {
			// Closing tag — flush text and return to caller.
			flushText()
			return nodes
		}

		// Self-closing tags (e.g. <br/>)
		if strings.HasSuffix(tagBody, "/") {
			tagBody = tagBody[:len(tagBody)-1]
			tagName := strings.Fields(tagBody)
			if len(tagName) > 0 && strings.ToLower(tagName[0]) == "br" {
				flushText()
				nodes = append(nodes, &TextNode{Text: "\n"})
			}
			continue
		}

		// Timestamp tags like <00:00:05.000> — skip.
		if len(tagBody) > 0 && (tagBody[0] >= '0' && tagBody[0] <= '9') {
			continue
		}

		// Opening tag.
		flushText()
		tag, voice, classes := parseTagHead(tagBody)
		children := parseNodes(text, pos) // recurse; stops at matching </tag>
		nodes = append(nodes, &TagNode{
			Tag:      tag,
			Voice:    voice,
			Classes:  classes,
			Children: children,
		})
	}

	flushText()
	return nodes
}

// parseTagHead parses a tag body string like "v Sam", "c.class1.class2", "i", etc.
func parseTagHead(body string) (tag, voice string, classes []string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", "", nil
	}

	// Determine the base tag name (up to first space or dot).
	first := strings.FieldsFunc(body, func(r rune) bool {
		return r == ' ' || r == '\t'
	})
	if len(first) == 0 {
		return "", "", nil
	}
	tagPart := first[0]

	// Class tags: "c.class1.class2"
	if strings.HasPrefix(tagPart, "c") && strings.Contains(tagPart, ".") {
		tag = "c"
		parts := strings.SplitN(tagPart, ".", -1)[1:]
		for _, p := range parts {
			if p != "" {
				classes = append(classes, p)
			}
		}
		return
	}

	// Lang tag: "lang en-US"
	if tagPart == "lang" {
		tag = "lang"
		return
	}

	// Voice tag: "v VoiceName" (voice name is everything after "v ")
	if tagPart == "v" {
		tag = "v"
		if len(first) > 1 {
			voice = strings.TrimSpace(strings.Join(first[1:], " "))
		} else {
			// Try the rest of body after "v"
			rest := strings.TrimPrefix(body, "v")
			rest = strings.TrimLeft(rest, " \t")
			voice = rest
		}
		return
	}

	// Simple tags: i, b, u, ruby, rt
	tag = strings.ToLower(tagPart)
	return
}

// stripOuterNewlines removes leading/trailing TextNodes that are purely whitespace.
func stripOuterNewlines(nodes []Node) []Node {
	// Trim leading.
	for len(nodes) > 0 {
		tn, ok := nodes[0].(*TextNode)
		if ok && strings.TrimSpace(tn.Text) == "" {
			nodes = nodes[1:]
		} else {
			break
		}
	}
	// Trim trailing.
	for len(nodes) > 0 {
		tn, ok := nodes[len(nodes)-1].(*TextNode)
		if ok && strings.TrimSpace(tn.Text) == "" {
			nodes = nodes[:len(nodes)-1]
		} else {
			break
		}
	}
	return nodes
}

// ---- HTML entity unescaping ----

func unescapeHTML(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '&' {
			b.WriteByte(s[i])
			i++
			continue
		}
		semi := strings.IndexByte(s[i:], ';')
		if semi < 0 {
			b.WriteByte(s[i])
			i++
			continue
		}
		entity := s[i : i+semi+1] // includes & and ;
		switch entity {
		case "&amp;":
			b.WriteByte('&')
		case "&lt;":
			b.WriteByte('<')
		case "&gt;":
			b.WriteByte('>')
		case "&nbsp;":
			b.WriteRune('\u00A0')
		case "&apos;":
			b.WriteByte('\'')
		case "&quot;":
			b.WriteByte('"')
		default:
			inner := entity[1 : len(entity)-1] // strip & and ;
			if strings.HasPrefix(inner, "#x") || strings.HasPrefix(inner, "#X") {
				n, err := strconv.ParseInt(inner[2:], 16, 32)
				if err == nil {
					b.WriteRune(rune(n))
				} else {
					b.WriteString(entity)
				}
			} else if strings.HasPrefix(inner, "#") {
				n, err := strconv.ParseInt(inner[1:], 10, 32)
				if err == nil {
					b.WriteRune(rune(n))
				} else {
					b.WriteString(entity)
				}
			} else {
				b.WriteString(entity)
			}
		}
		i += semi + 1
	}
	return b.String()
}

// voiceToXMLID sanitises a voice name for use as an XML ID component.
func voiceToXMLID(voice string) string {
	voice = strings.TrimSpace(voice)
	var b strings.Builder
	b.Grow(len(voice))
	for _, r := range voice {
		switch {
		case r == ' ' || r == '\t':
			b.WriteByte('_')
		case utf8.ValidRune(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
