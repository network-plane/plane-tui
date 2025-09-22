package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// OutputChannel controls command output levels and formats.
type OutputChannel interface {
	Level() OutputLevel
	SetLevel(level OutputLevel)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	WriteJSON(v any)
	WriteTable(headers []string, rows [][]string)
	Writer() io.Writer
	Buffer() *bytes.Buffer
}

// OutputLevel enumerates verbosity levels.
type OutputLevel int

const (
	OutputQuiet OutputLevel = iota
	OutputNormal
	OutputVerbose
	OutputDebug
)

// DefaultOutputChannel is an in-memory channel writing to io.Writer.
type DefaultOutputChannel struct {
	level   OutputLevel
	writer  io.Writer
	buf     *bytes.Buffer
	started bool
}

// NewOutputChannel builds an OutputChannel targeting provided writer.
func NewOutputChannel(w io.Writer) *DefaultOutputChannel {
	buf := &bytes.Buffer{}
	mw := io.MultiWriter(w, buf)
	return &DefaultOutputChannel{level: OutputNormal, writer: mw, buf: buf}
}

func (c *DefaultOutputChannel) ensureLead() {
	if c == nil || c.started {
		return
	}
	fmt.Fprint(c.writer, "\n")
	c.started = true
}

// Level returns current verbosity.
func (c *DefaultOutputChannel) Level() OutputLevel { return c.level }

// SetLevel updates verbosity.
func (c *DefaultOutputChannel) SetLevel(level OutputLevel) { c.level = level }

// Info writes an informational message.
func (c *DefaultOutputChannel) Info(msg string) {
	if c.level >= OutputQuiet {
		c.ensureLead()
		fmt.Fprintln(c.writer, msg)
	}
}

// Warn writes a warning message.
func (c *DefaultOutputChannel) Warn(msg string) {
	if c.level >= OutputQuiet {
		c.ensureLead()
		fmt.Fprintf(c.writer, "WARNING: %s\n", msg)
	}
}

// Error writes an error message.
func (c *DefaultOutputChannel) Error(msg string) {
	c.ensureLead()
	fmt.Fprintf(c.writer, "ERROR: %s\n", msg)
}

// WriteJSON renders JSON output respecting verbosity.
func (c *DefaultOutputChannel) WriteJSON(v any) {
	if c.level < OutputNormal {
		return
	}
	c.ensureLead()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		c.Error(fmt.Sprintf("failed to encode json: %v", err))
		return
	}
	fmt.Fprintln(c.writer, string(data))
}

// WriteTable renders tabular output without border markers.
func (c *DefaultOutputChannel) WriteTable(headers []string, rows [][]string) {
	if c.level < OutputNormal {
		return
	}
	if len(headers) == 0 {
		return
	}
	c.ensureLead()
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(strings.TrimSpace(h))
	}
	for _, row := range rows {
		for i := range widths {
			if i >= len(row) {
				continue
			}
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	fmt.Fprintln(c.writer, formatHeader(headers, widths))
	for _, row := range rows {
		fmt.Fprintln(c.writer, formatRow(row, widths))
	}
}
func formatHeader(headers []string, widths []int) string {
	if len(widths) == 0 {
		return ""
	}
	cells := make([]string, len(widths))
	for i := range widths {
		value := ""
		if i < len(headers) {
			value = strings.TrimSpace(headers[i])
		}
		cells[i] = fmt.Sprintf(" %-*s ", widths[i], value)
	}
	return "|" + strings.Join(cells, "|") + "|"
}

func formatRow(row []string, widths []int) string {
	if len(widths) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(widths) * 8)
	b.WriteString("  ")
	for i := range widths {
		value := ""
		if i < len(row) {
			value = row[i]
		}
		b.WriteString(fmt.Sprintf("%-*s", widths[i], value))
		if i < len(widths)-1 {
			b.WriteString("   ")
		}
	}
	return b.String()
}

// EnsureLineBreak guarantees the next prompt starts on a fresh line.
func EnsureLineBreak(out OutputChannel) {
	if out == nil {
		return
	}
	buf := out.Buffer()
	needNewline := false
	if dc, ok := out.(*DefaultOutputChannel); ok {
		if dc.started {
			needNewline = true
			dc.started = false
		}
	}
	if buf != nil {
		data := buf.Bytes()
		if len(data) > 0 {
			if data[len(data)-1] != '\n' {
				needNewline = true
			}
		}
		buf.Reset()
	}
	if needNewline {
		fmt.Fprintln(out.Writer())
	}
}

// AggregateMessages renders structured messages to an output channel.
func AggregateMessages(out OutputChannel, messages []OutputMessage) {
	if len(messages) == 0 {
		return
	}
	order := map[SeverityLevel]int{
		SeverityInfo:    0,
		SeverityWarning: 1,
		SeverityError:   2,
	}
	sort.SliceStable(messages, func(i, j int) bool {
		return order[messages[i].Level] < order[messages[j].Level]
	})
	for _, msg := range messages {
		switch msg.Level {
		case SeverityInfo:
			out.Info(msg.Content)
		case SeverityWarning:
			out.Warn(msg.Content)
		default:
			out.Error(msg.Content)
		}
	}
}

// Writer returns the underlying writer.
func (c *DefaultOutputChannel) Writer() io.Writer { return c.writer }

// Buffer exposes captured output, useful in tests.
func (c *DefaultOutputChannel) Buffer() *bytes.Buffer { return c.buf }
