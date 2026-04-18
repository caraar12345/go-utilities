package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	assPlayResX = 1920
	assPlayResY = 1080
)

// ConvertToASS converts a parsed VTTFile to an ASS (Advanced SubStation Alpha) document.
func ConvertToASS(vtt *VTTFile) ([]byte, error) {
	var buf bytes.Buffer

	// [Script Info]
	buf.WriteString("[Script Info]\n")
	buf.WriteString("; Converted from WebVTT\n")
	buf.WriteString("ScriptType: v4.00+\n")
	buf.WriteString(fmt.Sprintf("PlayResX: %d\n", assPlayResX))
	buf.WriteString(fmt.Sprintf("PlayResY: %d\n", assPlayResY))
	buf.WriteString("WrapStyle: 0\n")
	buf.WriteString("ScaledBorderAndShadow: yes\n")
	for _, note := range vtt.Notes {
		for _, line := range strings.Split(note, "\n") {
			buf.WriteString("; " + line + "\n")
		}
	}
	buf.WriteString("\n")

	// [V4+ Styles]
	buf.WriteString("[V4+ Styles]\n")
	buf.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")

	writeASSStyle(&buf, "Default", "&H00FFFFFF&", false, false)

	voices := make([]string, 0, len(vtt.Styles))
	for v := range vtt.Styles {
		voices = append(voices, v)
	}
	sort.Strings(voices)
	for _, voice := range voices {
		style := vtt.Styles[voice]
		writeASSStyle(&buf, voice, rgbToASS(style.Color), style.Bold, style.Italic)
	}
	buf.WriteString("\n")

	// [Events]
	buf.WriteString("[Events]\n")
	buf.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	for _, cue := range vtt.Cues {
		start := formatDurationASS(cue.Start)
		end := formatDurationASS(cue.End)

		dominant := dominantVoice(cue.Nodes)
		styleName := "Default"
		if dominant != "" {
			if _, ok := vtt.Styles[dominant]; ok {
				styleName = dominant
			}
		}

		text := renderNodesASS(cue.Nodes, dominant, vtt.Styles, vtt.Classes, cue.Settings)
		fmt.Fprintf(&buf, "Dialogue: 0,%s,%s,%s,,0,0,0,,%s\n", start, end, styleName, text)
	}

	return buf.Bytes(), nil
}

// writeASSStyle writes one Style line. All styles use the same font/border defaults;
// only colour, bold, and italic vary.
func writeASSStyle(buf *bytes.Buffer, name, primaryColour string, bold, italic bool) {
	boldVal := 0
	if bold {
		boldVal = -1
	}
	italicVal := 0
	if italic {
		italicVal = -1
	}
	// Format: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,
	//         Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,
	//         BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding
	fmt.Fprintf(buf,
		"Style: %s,Arial,48,%s,&H000000FF&,&H00000000&,&H80000000&,%d,%d,0,0,100,100,0,0,1,2,0,2,10,10,30,1\n",
		name, primaryColour, boldVal, italicVal,
	)
}

// dominantVoice returns the first voice tag name at the top level of the node list.
func dominantVoice(nodes []Node) string {
	for _, n := range nodes {
		if tn, ok := n.(*TagNode); ok && tn.Tag == "v" {
			return tn.Voice
		}
	}
	return ""
}

// renderNodesASS produces the ASS Text field for a cue, including any leading
// positioning/alignment override block derived from cue settings.
func renderNodesASS(nodes []Node, dominant string, styles, classes map[string]CueStyle, settings CueSettings) string {
	var b strings.Builder

	if pre := cueSettingsOverride(settings); pre != "" {
		b.WriteString(pre)
	}

	state := formatState{color: lineStyleColor(dominant, styles)}
	renderNodesASSInner(&b, nodes, dominant, styles, classes, state)
	return b.String()
}

// formatState tracks the currently-active formatting so a span can restore the
// exact outer state when it closes, instead of emitting broad resets like {\r}
// that wipe surrounding <b>/<i>/<u>/colour state.
type formatState struct {
	color  string // active ASS primary colour, e.g. "&H00FFFFFF&"
	bold   bool
	italic bool
}

// cueSettingsOverride renders the ASS override block for the cue's line/position/align
// settings. Returns "" when no override is needed.
func cueSettingsOverride(s CueSettings) string {
	if s.Line < 0 && !s.PositionSet && s.Align == "" {
		return ""
	}

	anH := alignHorizontal(s.Align) // 1=left, 2=centre, 3=right

	if s.Line < 0 {
		// No explicit vertical position: keep the style's MarginV (bottom alignment).
		if s.Align == "" {
			return ""
		}
		return fmt.Sprintf("{\\an%d}", anH) // 1/2/3 — bottom-{left,centre,right}
	}

	// Vertical line is set → anchor at top of text, position at (x, y).
	y := int(s.Line / 100.0 * float64(assPlayResY))
	x := assPlayResX / 2
	if s.PositionSet {
		x = int(s.Position / 100.0 * float64(assPlayResX))
	} else {
		switch anH {
		case 1:
			x = 0
		case 3:
			x = assPlayResX
		}
	}
	// Top-row alignment: 7=top-left, 8=top-centre, 9=top-right.
	an := anH + 6
	// TODO: settings.Size is parsed but not applied — would need \pos + margins.
	return fmt.Sprintf("{\\an%d\\pos(%d,%d)}", an, x, y)
}

func alignHorizontal(a string) int {
	switch strings.ToLower(a) {
	case "start", "left":
		return 1
	case "end", "right":
		return 3
	default:
		return 2
	}
}

// lineStyleColor returns the ASS primary colour for the Dialogue line's style,
// used to restore colour after a voice/class override.
func lineStyleColor(dominant string, styles map[string]CueStyle) string {
	if dominant != "" {
		if s, ok := styles[dominant]; ok {
			return rgbToASS(s.Color)
		}
	}
	return "&H00FFFFFF&"
}

// renderNodesASSInner walks the cue AST emitting ASS override tags. The state
// struct tracks active colour/bold/italic so a closing span can restore exactly
// the outer formatting instead of issuing a broad {\r} reset.
func renderNodesASSInner(
	b *strings.Builder,
	nodes []Node,
	dominant string,
	styles, classes map[string]CueStyle,
	state formatState,
) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			b.WriteString(escapeASSText(n.Text))

		case *TagNode:
			switch n.Tag {
			case "v":
				if n.Voice == dominant {
					renderNodesASSInner(b, n.Children, dominant, styles, classes, state)
				} else if style, ok := styles[n.Voice]; ok {
					writeStyleSpan(b, style, state, func(inner formatState) {
						renderNodesASSInner(b, n.Children, dominant, styles, classes, inner)
					})
				} else {
					renderNodesASSInner(b, n.Children, dominant, styles, classes, state)
				}
			case "c":
				style, ok := mergeClassStyles(n.Classes, classes)
				if !ok {
					renderNodesASSInner(b, n.Children, dominant, styles, classes, state)
					break
				}
				writeStyleSpan(b, style, state, func(inner formatState) {
					renderNodesASSInner(b, n.Children, dominant, styles, classes, inner)
				})
			case "i":
				if state.italic {
					renderNodesASSInner(b, n.Children, dominant, styles, classes, state)
				} else {
					b.WriteString("{\\i1}")
					renderNodesASSInner(b, n.Children, dominant, styles, classes, formatState{color: state.color, bold: state.bold, italic: true})
					b.WriteString("{\\i0}")
				}
			case "b":
				if state.bold {
					renderNodesASSInner(b, n.Children, dominant, styles, classes, state)
				} else {
					b.WriteString("{\\b1}")
					renderNodesASSInner(b, n.Children, dominant, styles, classes, formatState{color: state.color, bold: true, italic: state.italic})
					b.WriteString("{\\b0}")
				}
			case "u":
				b.WriteString("{\\u1}")
				renderNodesASSInner(b, n.Children, dominant, styles, classes, state)
				b.WriteString("{\\u0}")
			default:
				// ruby, rt, lang — pass children through.
				renderNodesASSInner(b, n.Children, dominant, styles, classes, state)
			}
		}
	}
}

// mergeClassStyles folds the styles for each named class (later wins) into one.
// Returns false if none of the classes have a defined style.
func mergeClassStyles(classNames []string, classes map[string]CueStyle) (CueStyle, bool) {
	var merged CueStyle
	any := false
	for _, name := range classNames {
		s, ok := classes[name]
		if !ok {
			continue
		}
		any = true
		if s.Color != "" {
			merged.Color = s.Color
		}
		if s.Bold {
			merged.Bold = true
		}
		if s.Italic {
			merged.Italic = true
		}
	}
	return merged, any
}

// writeStyleSpan emits the open override for a voice or class span, runs body
// with the updated format state, then emits the close override that restores
// exactly the attributes this span changed.
func writeStyleSpan(b *strings.Builder, style CueStyle, outer formatState, body func(inner formatState)) {
	inner := outer
	var open strings.Builder
	open.WriteByte('{')
	if style.Color != "" {
		newColor := rgbToASS(style.Color)
		if newColor != outer.color {
			open.WriteString("\\c")
			open.WriteString(newColor)
			inner.color = newColor
		}
	}
	if style.Bold && !outer.bold {
		open.WriteString("\\b1")
		inner.bold = true
	}
	if style.Italic && !outer.italic {
		open.WriteString("\\i1")
		inner.italic = true
	}
	if open.Len() > 1 {
		open.WriteByte('}')
		b.WriteString(open.String())
	}

	body(inner)

	var close strings.Builder
	close.WriteByte('{')
	if inner.color != outer.color {
		close.WriteString("\\c")
		close.WriteString(outer.color)
	}
	if inner.bold != outer.bold {
		close.WriteString("\\b0")
	}
	if inner.italic != outer.italic {
		close.WriteString("\\i0")
	}
	if close.Len() > 1 {
		close.WriteByte('}')
		b.WriteString(close.String())
	}
}

// escapeASSText protects cue text from being interpreted as ASS override blocks
// and converts literal newlines to the ASS hard-break sequence.
func escapeASSText(s string) string {
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	s = strings.ReplaceAll(s, "\n", "\\N")
	return s
}

// rgbToASS converts a CSS hex colour to ASS format (&HAABBGGRR&).
// Accepts #RGB, #RRGGBB, and #RRGGBBAA; anything else falls back to white.
// ASS stores the leading byte as transparency, so CSS alpha AA is inverted.
func rgbToASS(hex string) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	hex = strings.ToUpper(hex)
	switch len(hex) {
	case 3:
		// #RGB → #RRGGBB
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		fallthrough
	case 6:
		if !isHex(hex) {
			break
		}
		return "&H00" + hex[4:6] + hex[2:4] + hex[0:2] + "&"
	case 8:
		if !isHex(hex) {
			break
		}
		alpha, err := parseHexByte(hex[6:8])
		if err != nil {
			break
		}
		transparency := 255 - alpha
		return fmt.Sprintf("&H%02X%s%s%s&", transparency, hex[4:6], hex[2:4], hex[0:2])
	}
	return "&H00FFFFFF&" // fallback: white
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func parseHexByte(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%02X", &n)
	return n, err
}

// formatDurationASS formats a time.Duration as ASS timestamp: H:MM:SS.cc (centiseconds).
func formatDurationASS(d time.Duration) string {
	totalMs := d.Milliseconds()
	cs := (totalMs % 1000) / 10
	totalS := totalMs / 1000
	s := totalS % 60
	totalM := totalS / 60
	m := totalM % 60
	h := totalM / 60
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}
