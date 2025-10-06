package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var (
	checkMark = "✔"
	crossMark = "✖"
	verbose   = false
	writer    io.Writer = os.Stdout
)

func SetVerbose(v bool) {
	verbose = v
}

func SetWriter(w io.Writer) {
	writer = w
}

func Success(service string, duration time.Duration) {
	fmt.Fprintf(writer, "[%s] %-35s %s\n", checkMark, service, ShortDuration(duration))
}

func Failure(service string, duration time.Duration) {
	fmt.Fprintf(writer, "[%s] %-35s %s\n", crossMark, service, ShortDuration(duration))
}

func SuccessMsg(service, message string) {
	fmt.Fprintf(writer, "[%s] %-35s %s\n", checkMark, service, message)
}

func FailureMsg(service, message string) {
	fmt.Fprintf(writer, "[%s] %-35s %s\n", crossMark, service, message)
}

func Info(message string) {
	fmt.Fprintf(writer, "%s\n", message)
}

func Verbose(message string) {
	if verbose {
		fmt.Fprintf(writer, "    %s\n", message)
	}
}

func Error(message string) {
	fmt.Fprintf(writer, "[%s] %s\n", crossMark, message)
}

func Section(title string) {
	fmt.Fprintf(writer, "\n%s\n", title)
}

func Summary(successful, total int) {
	fmt.Fprintf(writer, "\nStarted %d/%d services\n", successful, total)
}

func ShortDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}

	return fmt.Sprintf("%.1fm", d.Minutes())
}

type ServiceOutput struct {
	service   string
	startTime time.Time
	verbose   bool
}

func NewServiceOutput(service string, verboseMode bool) *ServiceOutput {
	return &ServiceOutput{
		service:   service,
		startTime: time.Now(),
		verbose:   verboseMode,
	}
}

func (s *ServiceOutput) Start(message string) {
	if s.verbose {
		Info(fmt.Sprintf("Starting %s: %s", s.service, message))
	}
}

func (s *ServiceOutput) Step(message string) {
	if s.verbose {
		Verbose(message)
	}
}

func (s *ServiceOutput) Complete(err error) {
	duration := time.Since(s.startTime)
	if err != nil {
		Failure(s.service, duration)
		if s.verbose {
			Error(fmt.Sprintf("  Error: %v", err))
		}
	} else {
		Success(s.service, duration)
	}
}

func (s *ServiceOutput) CompleteWithMessage(success bool, message string) {
	if success {
		SuccessMsg(s.service, message)
	} else {
		FailureMsg(s.service, message)
	}
}

type BoxStyle struct {
	lines []string
}

func NewBox() *BoxStyle {
	return &BoxStyle{lines: make([]string, 0)}
}

func (b *BoxStyle) AddLine(line string) {
	b.lines = append(b.lines, line)
}

func (b *BoxStyle) AddKeyValue(key, value string) {
	b.lines = append(b.lines, fmt.Sprintf("  %-20s %s", key+":", value))
}

func (b *BoxStyle) Print() {
	if len(b.lines) == 0 {
		return
	}

	maxLen := 0
	for _, line := range b.lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	border := strings.Repeat("─", maxLen+4)
	fmt.Fprintf(writer, "┌%s┐\n", border)
	for _, line := range b.lines {
		padding := strings.Repeat(" ", maxLen-len(line))
		fmt.Fprintf(writer, "│  %s%s  │\n", line, padding)
	}
	fmt.Fprintf(writer, "└%s┘\n", border)
}

func PrintEndpoints(title string, endpoints map[string]string) {
	if len(endpoints) == 0 {
		return
	}

	box := NewBox()
	box.AddLine(title)
	box.AddLine("")
	for name, url := range endpoints {
		box.AddKeyValue(name, url)
	}
	box.Print()
}

func PrintServiceInfo(service string, info map[string]string) {
	box := NewBox()
	box.AddLine(fmt.Sprintf("%s Service", service))
	box.AddLine("")
	for key, value := range info {
		box.AddKeyValue(key, value)
	}
	box.Print()
}
