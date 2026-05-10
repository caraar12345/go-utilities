# vtt2ttml
 
WebVTT subtitle converter. Supports TTML and ASS output.
 
## Build
 
```bash
go build -o vtt2ttml .
```
 
## Usage
 
```
vtt2ttml [-format ttml|ass] input.vtt [output]
```
 
Format is auto-detected from the output file extension (`.ttml`/`.dfxp` → TTML, `.ass`/`.ssa` → ASS). Falls back to TTML if writing to stdout or the extension is unrecognised. Use `-format` to override.
 
## Code structure
 
| File | Purpose |
|---|---|
| `parser.go` | WebVTT → internal data model (`VTTFile`, `Cue`, `Node` tree) |
| `converter.go` | `VTTFile` → TTML XML |
| `converter_ass.go` | `VTTFile` → ASS text |
| `main.go` | CLI wiring (`flag`, file I/O) |
 
## WebVTT features handled
 
- `STYLE` blocks: `::cue(v[voice="Name"])` rules → per-voice colour and bold
- `NOTE` blocks → preserved as XML comments (TTML) or `; comments` (ASS)
- `<v Name>` voice tags → colour spans referencing named styles
- `<i>`, `<b>`, `<u>` inline formatting
- `<c.class>`, `<ruby>`, `<rt>`, `<lang>` → children passed through, tags dropped
- `line:X%` cue setting → vertical region (TTML) or `\an8\pos` override (ASS)
- `position:`, `size:`, `align:` cue settings parsed (TTML only; ASS ignores size/align)
- HTML entities: `&amp;`, `&lt;`, `&gt;`, `&nbsp;`, `&apos;`, `&quot;`, `&#NNN;`, `&#xNNN;`
- Both `HH:MM:SS.mmm` and `MM:SS.mmm` timestamp formats
 
## TTML output notes
 
- One `<region>` per unique `line:X%` value. Formula: X ≥ 80 → bottom (`displayAlign=after`, originY=X−15%); X ≤ 20 → top (`before`); otherwise middle (`before`).
- `enc.Indent` is deliberately **not** used — whitespace inside `<p>` is rendered as content by TTML players, causing off-centre alignment. Indentation is applied manually to structural elements only.
 
## ASS output notes
 
- PlayRes: 1920×1080. `\pos` coordinates scale with this.
- Colours are BGR (`#RRGGBB` → `&H00BBGGRR&`).
- The dominant style of a Dialogue line is the first `<v>` tag in the cue. Other voices in the same cue get inline `{\c...}...{\r}` overrides.
- `line:90%` (default) uses the style's `MarginV=30` with no position override. All other `line:X%` values become `{\an8\pos(960,Y)}`.
