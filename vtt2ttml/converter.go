package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// region holds a computed TTML region definition.
type region struct {
	ID           string
	LineVal      float64
	OriginX      string
	OriginY      string
	ExtentX      string
	ExtentY      string
	DisplayAlign string
	TextAlign    string
}

// regionForLine computes a TTML region from a WebVTT line percentage.
func regionForLine(lineVal float64) region {
	r := region{
		ID:      fmt.Sprintf("r.%.10g", lineVal),
		LineVal: lineVal,
		OriginX: "0%",
		ExtentX: "100%",
		ExtentY: "15%",
	}
	// Sanitise ID (dots from float formatting are fine in XML IDs).
	// Replace any decimal point with "p" to keep it clean.
	r.ID = "r." + strings.ReplaceAll(fmt.Sprintf("%.10g", lineVal), ".", "p")

	switch {
	case lineVal >= 80:
		// Bottom region: anchor bottom edge at ~lineVal%.
		r.OriginY = fmt.Sprintf("%.10g%%", lineVal-15)
		r.DisplayAlign = "after"
	case lineVal <= 20:
		// Top region.
		r.OriginY = fmt.Sprintf("%.10g%%", lineVal)
		r.DisplayAlign = "before"
	default:
		// Middle.
		r.OriginY = fmt.Sprintf("%.10g%%", lineVal)
		r.DisplayAlign = "before"
	}
	r.TextAlign = "center"
	return r
}

// collectRegions returns the unique regions needed for a set of cues.
func collectRegions(cues []Cue) ([]region, map[float64]string) {
	seen := make(map[float64]region)
	for _, c := range cues {
		lv := c.Settings.Line
		if lv < 0 {
			lv = 90
		}
		if _, ok := seen[lv]; !ok {
			seen[lv] = regionForLine(lv)
		}
	}

	regions := make([]region, 0, len(seen))
	for _, r := range seen {
		regions = append(regions, r)
	}
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].LineVal < regions[j].LineVal
	})

	regionMap := make(map[float64]string, len(regions))
	for _, r := range regions {
		regionMap[r.LineVal] = r.ID
	}
	return regions, regionMap
}

// wi writes a newline followed by depth*2 spaces — manual indentation for
// structural (non-content) elements only. Never called inside <p> content.
func wi(enc *xml.Encoder, depth int) error {
	return enc.EncodeToken(xml.CharData("\n" + strings.Repeat("  ", depth)))
}

// ConvertToTTML converts a parsed VTTFile to a TTML document.
func ConvertToTTML(vtt *VTTFile) ([]byte, error) {
	regions, regionMap := collectRegions(vtt.Cues)

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	// No enc.Indent — indentation is applied manually to structural elements only.
	// Indenting inside <p> would inject whitespace text nodes that TTML renderers
	// treat as real content, causing off-centre alignment.

	// XML declaration.
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")

	// <tt> root element with namespaces.
	ttElem := xml.StartElement{
		Name: xml.Name{Local: "tt"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xml:lang"}, Value: vtt.Lang},
			{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/ns/ttml"},
			{Name: xml.Name{Local: "xmlns:tts"}, Value: "http://www.w3.org/ns/ttml#styling"},
			{Name: xml.Name{Local: "xmlns:ttp"}, Value: "http://www.w3.org/ns/ttml#parameter"},
			{Name: xml.Name{Local: "ttp:profile"}, Value: "http://www.w3.org/ns/ttml/profile/ttml2"},
		},
	}
	if err := enc.EncodeToken(ttElem); err != nil {
		return nil, err
	}

	// <head>
	if err := wi(enc, 1); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "head"}}); err != nil {
		return nil, err
	}

	// NOTE blocks as XML comments.
	for _, note := range vtt.Notes {
		if err := wi(enc, 2); err != nil {
			return nil, err
		}
		comment := " " + strings.ReplaceAll(note, "--", "- -") + " "
		if err := enc.EncodeToken(xml.Comment(comment)); err != nil {
			return nil, err
		}
	}

	// <styling>
	if err := writeStyleSection(enc, vtt.Styles); err != nil {
		return nil, err
	}

	// <layout>
	if err := writeLayoutSection(enc, regions); err != nil {
		return nil, err
	}

	// </head>
	if err := wi(enc, 1); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "head"}}); err != nil {
		return nil, err
	}

	// <body><div>
	if err := wi(enc, 1); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "body"}}); err != nil {
		return nil, err
	}
	if err := wi(enc, 2); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "div"}}); err != nil {
		return nil, err
	}

	if err := writeCues(enc, vtt.Cues, regionMap); err != nil {
		return nil, err
	}

	// </div></body>
	if err := wi(enc, 2); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "div"}}); err != nil {
		return nil, err
	}
	if err := wi(enc, 1); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "body"}}); err != nil {
		return nil, err
	}

	// </tt>
	if err := wi(enc, 0); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(ttElem.End()); err != nil {
		return nil, err
	}

	if err := enc.Flush(); err != nil {
		return nil, err
	}

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeStyleSection(enc *xml.Encoder, styles map[string]CueStyle) error {
	if err := wi(enc, 2); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "styling"}}); err != nil {
		return err
	}

	// Sort voices for deterministic output.
	voices := make([]string, 0, len(styles))
	for v := range styles {
		voices = append(voices, v)
	}
	sort.Strings(voices)

	for _, voice := range voices {
		style := styles[voice]
		id := "s." + voiceToXMLID(voice)

		attrs := []xml.Attr{
			{Name: xml.Name{Local: "xml:id"}, Value: id},
		}
		if style.Color != "" {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: "tts:color"},
				Value: style.Color,
			})
		}
		if style.Bold {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: "tts:fontWeight"},
				Value: "bold",
			})
		}
		if style.Italic {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Local: "tts:fontStyle"},
				Value: "italic",
			})
		}

		if err := wi(enc, 3); err != nil {
			return err
		}
		elem := xml.StartElement{Name: xml.Name{Local: "style"}, Attr: attrs}
		if err := enc.EncodeToken(elem); err != nil {
			return err
		}
		if err := enc.EncodeToken(elem.End()); err != nil {
			return err
		}
	}

	if err := wi(enc, 2); err != nil {
		return err
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "styling"}})
}

func writeLayoutSection(enc *xml.Encoder, regions []region) error {
	if err := wi(enc, 2); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "layout"}}); err != nil {
		return err
	}

	for _, r := range regions {
		if err := wi(enc, 3); err != nil {
			return err
		}
		elem := xml.StartElement{
			Name: xml.Name{Local: "region"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "xml:id"}, Value: r.ID},
				{Name: xml.Name{Local: "tts:origin"}, Value: r.OriginX + " " + r.OriginY},
				{Name: xml.Name{Local: "tts:extent"}, Value: r.ExtentX + " " + r.ExtentY},
				{Name: xml.Name{Local: "tts:displayAlign"}, Value: r.DisplayAlign},
				{Name: xml.Name{Local: "tts:textAlign"}, Value: r.TextAlign},
			},
		}
		if err := enc.EncodeToken(elem); err != nil {
			return err
		}
		if err := enc.EncodeToken(elem.End()); err != nil {
			return err
		}
	}

	if err := wi(enc, 2); err != nil {
		return err
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "layout"}})
}

func writeCues(enc *xml.Encoder, cues []Cue, regionMap map[float64]string) error {
	for _, cue := range cues {
		lv := cue.Settings.Line
		if lv < 0 {
			lv = 90
		}
		regionID := regionMap[lv]

		if err := wi(enc, 3); err != nil {
			return err
		}
		pAttrs := []xml.Attr{
			{Name: xml.Name{Local: "begin"}, Value: formatDuration(cue.Start)},
			{Name: xml.Name{Local: "end"}, Value: formatDuration(cue.End)},
			{Name: xml.Name{Local: "region"}, Value: regionID},
		}
		pElem := xml.StartElement{Name: xml.Name{Local: "p"}, Attr: pAttrs}
		if err := enc.EncodeToken(pElem); err != nil {
			return err
		}
		// Content written with NO indentation — whitespace inside <p> is rendered.
		if err := writeNodes(enc, cue.Nodes); err != nil {
			return err
		}
		if err := enc.EncodeToken(pElem.End()); err != nil {
			return err
		}
	}
	return nil
}

func writeNodes(enc *xml.Encoder, nodes []Node) error {
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			if err := writeTextNode(enc, n.Text); err != nil {
				return err
			}
		case *TagNode:
			if err := writeTagNode(enc, n); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeTextNode emits text, converting newlines to <br/> elements.
func writeTextNode(enc *xml.Encoder, text string) error {
	segments := strings.Split(text, "\n")
	for i, seg := range segments {
		if i > 0 {
			// Emit <br/>.
			brElem := xml.StartElement{Name: xml.Name{Local: "br"}}
			if err := enc.EncodeToken(brElem); err != nil {
				return err
			}
			if err := enc.EncodeToken(brElem.End()); err != nil {
				return err
			}
		}
		if seg != "" {
			if err := enc.EncodeToken(xml.CharData(seg)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeTagNode(enc *xml.Encoder, n *TagNode) error {
	switch n.Tag {
	case "v":
		id := voiceToXMLID(n.Voice)
		spanElem := xml.StartElement{
			Name: xml.Name{Local: "span"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "style"}, Value: "s." + id},
			},
		}
		if err := enc.EncodeToken(spanElem); err != nil {
			return err
		}
		if err := writeNodes(enc, n.Children); err != nil {
			return err
		}
		return enc.EncodeToken(spanElem.End())

	case "i":
		spanElem := xml.StartElement{
			Name: xml.Name{Local: "span"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "tts:fontStyle"}, Value: "italic"},
			},
		}
		if err := enc.EncodeToken(spanElem); err != nil {
			return err
		}
		if err := writeNodes(enc, n.Children); err != nil {
			return err
		}
		return enc.EncodeToken(spanElem.End())

	case "b":
		spanElem := xml.StartElement{
			Name: xml.Name{Local: "span"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "tts:fontWeight"}, Value: "bold"},
			},
		}
		if err := enc.EncodeToken(spanElem); err != nil {
			return err
		}
		if err := writeNodes(enc, n.Children); err != nil {
			return err
		}
		return enc.EncodeToken(spanElem.End())

	case "u":
		spanElem := xml.StartElement{
			Name: xml.Name{Local: "span"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "tts:textDecoration"}, Value: "underline"},
			},
		}
		if err := enc.EncodeToken(spanElem); err != nil {
			return err
		}
		if err := writeNodes(enc, n.Children); err != nil {
			return err
		}
		return enc.EncodeToken(spanElem.End())

	default:
		// c, ruby, rt, lang, unknown — pass children through transparently.
		return writeNodes(enc, n.Children)
	}
}

// formatDuration formats a time.Duration as HH:MM:SS.mmm.
func formatDuration(d time.Duration) string {
	totalMs := d.Milliseconds()
	ms := totalMs % 1000
	totalS := totalMs / 1000
	s := totalS % 60
	totalM := totalS / 60
	m := totalM % 60
	h := totalM / 60
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
