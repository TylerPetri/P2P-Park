package parknode

import (
	"fmt"
	"io"
	"sync"
)

type Printer interface {
	Printf(format string, args ...any)
	Println(args ...any)
}

type StdPrinter struct {
	mu sync.Mutex
	w  io.Writer
}

func NewStdPrinter(w io.Writer) *StdPrinter { return &StdPrinter{w: w} }

func (p *StdPrinter) Printf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.w, format, args...)
}

func (p *StdPrinter) Println(args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.w, args...)
}
