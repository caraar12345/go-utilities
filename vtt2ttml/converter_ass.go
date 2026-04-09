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

	writeASSStyle(&buf, "Default", "&H00FFFFFF&", false)

	voices := make([]string, 0, len(vtt.Styles))
	for v := range vtt.Styles {
		voices = append(voices, v)
	}
	sort.Strings(voices)
	for _, voice := range voices {
		style := vtt.Styles[voice]
		writeASSStyle(&buf, voice, rgbToASS(style.Color), style.Bold)
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

		text := renderNodesASS(cue.Nodes, dominant, vtt.Styles, cue.Settings)
		fmt.Fprintf(&buf, "Dialogue: 0,%s,%s,%s,,0,0,0,,%s\n", start, end, styleName, text)
	}

	return buf.Bytes(), nil
}

// writeASSStyle writes one Style line. All styles use the same font/border defaults;
// only colour and bold vary.
func writeASSStyle(buf *bytes.Buffer, name, primaryColour string, bold bool) {
	boldVal := 0
	if bold {
		boldVal = -1
	}
	// Format: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,
	//         Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,
	//         BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding
	fmt.Fprintf(buf,
		"Style: %s,Arial,48,%s,&H000000FF&,&H00000000&,&H80000000&,%d,0,0,0,100,100,0,0,1,2,0,2,10,10,30,1\n",
		name, primaryColour, boldVal,
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

// renderNodesASS produces the ASS Text field for a cue.
func renderNodesASS(nodes []Node, dominant string, styles map[string]CueStyle, settings CueSettings) string {
	var b strings.Builder

	// Vertical positioning override for non-default line positions.
	// line:90 is the default (handled by style MarginV); anything else gets \an + \pos.
	lv := settings.Line
	if lv >= 0 && lv != 90 {
		y := int(lv / 100.0 * float64(assPlayResY))
		// \an8 = top-centre anchor; text flows downward from y.
		fmt.Fprintf(&b, "{\\an8\\pos(%d,%d)}", assPlayResX/2, y)
	}

	renderNodesASSInner(&b, nodes, dominant, styles)
	return b.String()
}

func renderNodesASSInner(b *strings.Builder, nodes []Node, dominant string, styles map[string]CueStyle) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			b.WriteString(strings.ReplaceAll(n.Text, "\n", "\\N"))

		case *TagNode:
			switch n.Tag {
			case "v":
				if n.Voice == dominant {
					// Same style as the Dialogue line — no override needed.
					renderNodesASSInner(b, n.Children, dominant, styles)
				} else {
					// Different voice: inline colour (and bold if needed), then reset.
					if style, ok := styles[n.Voice]; ok {
						override := fmt.Sprintf("{\\c%s}", rgbToASS(style.Color))
						if style.Bold {
							override = fmt.Sprintf("{\\c%s\\b1}", rgbToASS(style.Color))
						}
						b.WriteString(override)
						renderNodesASSInner(b, n.Children, dominant, styles)
						b.WriteString("{\\r}") // resets to Dialogue style
					} else {
						renderNodesASSInner(b, n.Children, dominant, styles)
					}
				}
			case "i":
				b.WriteString("{\\i1}")
				renderNodesASSInner(b, n.Children, dominant, styles)
				b.WriteString("{\\i0}")
			case "b":
				b.WriteString("{\\b1}")
				renderNodesASSInner(b, n.Children, dominant, styles)
				b.WriteString("{\\b0}")
			case "u":
				b.WriteString("{\\u1}")
				renderNodesASSInner(b, n.Children, dominant, styles)
				b.WriteString("{\\u0}")
			default:
				// c, ruby, rt, lang — pass children through.
				renderNodesASSInner(b, n.Children, dominant, styles)
			}
		}
	}
}

// rgbToASS converts a CSS hex colour (#RRGGBB) to ASS format (&H00BBGGRR&).
// ASS stores colours in BBGGRR order with a leading alpha byte (00 = fully opaque).
func rgbToASS(hex string) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	hex = strings.ToUpper(hex)
	if len(hex) == 6 {
		return "&H00" + hex[4:6] + hex[2:4] + hex[0:2] + "&"
	}
	return "&H00FFFFFF&" // fallback: white
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
