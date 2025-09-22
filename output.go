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
	level  OutputLevel
	writer io.Writer
	buf    *bytes.Buffer
}

// NewOutputChannel builds an OutputChannel targeting provided writer.
func NewOutputChannel(w io.Writer) *DefaultOutputChannel {
	buf := &bytes.Buffer{}
	mw := io.MultiWriter(w, buf)
	return &DefaultOutputChannel{level: OutputNormal, writer: mw, buf: buf}
}

// Level returns current verbosity.
func (c *DefaultOutputChannel) Level() OutputLevel { return c.level }

// SetLevel updates verbosity.
func (c *DefaultOutputChannel) SetLevel(level OutputLevel) { c.level = level }

// Info writes an informational message.
func (c *DefaultOutputChannel) Info(msg string) {
	if c.level >= OutputQuiet {
		fmt.Fprintln(c.writer, msg)
	}
}

// Warn writes a warning message.
func (c *DefaultOutputChannel) Warn(msg string) {
	if c.level >= OutputQuiet {
		fmt.Fprintf(c.writer, "WARNING: %s\n", msg)
	}
}

// Error writes an error message.
func (c *DefaultOutputChannel) Error(msg string) {
	fmt.Fprintf(c.writer, "ERROR: %s\n", msg)
}

// WriteJSON renders JSON output respecting verbosity.
func (c *DefaultOutputChannel) WriteJSON(v any) {
	if c.level < OutputNormal {
		return
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		c.Error(fmt.Sprintf("failed to encode json: %v", err))
		return
	}
	fmt.Fprintln(c.writer, string(data))
}

// WriteTable renders tabular output.
func (c *DefaultOutputChannel) WriteTable(headers []string, rows [][]string) {
	if c.level < OutputNormal {
		return
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	border := "+" + strings.Repeat("-", sum(widths)+len(widths)*3) + "+"
	fmt.Fprintln(c.writer, border)
	fmt.Fprintln(c.writer, formatRow(headers, widths))
	fmt.Fprintln(c.writer, border)
	for _, row := range rows {
		fmt.Fprintln(c.writer, formatRow(row, widths))
	}
	fmt.Fprintln(c.writer, border)
}

func formatRow(row []string, widths []int) string {
	cells := make([]string, len(row))
	for i, cell := range row {
		cells[i] = fmt.Sprintf(" %-*s ", widths[i], cell)
	}
	return "|" + strings.Join(cells, "|") + "|"
}

func sum(values []int) int {
	s := 0
	for _, v := range values {
		s += v
	}
	return s
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
