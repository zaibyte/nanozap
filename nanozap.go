// Copyright (c) 2016 Uber Technologies, Inc.
// Copyright (c) 2020 Temple3x
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package nanozap

import (
	"context"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/templexxx/nanozap/zapcore"
	"github.com/templexxx/tsc"
)

// For nanozap, the random order only happens when there is a flood,
// and random order logs won't cause serious issues.
type Logger struct {
	name string

	core zapcore.Core

	ring *Ring

	loopCtx    context.Context
	loopCancel func()
	loopWg     sync.WaitGroup
}

// New constructs a new Logger from the provided zapcore.Core and Options. If
// the passed zapcore.Core is nil, it falls back to using a no-op
// implementation.
//
// This is the most flexible way to construct a Logger, but also the most
// verbose. For typical use cases, the highly-opinionated presets
// (NewProduction, NewDevelopment, and NewExample) or the Config struct are
// more convenient.
//
// For sample code, see the package-level AdvancedConfiguration example.
func New(core zapcore.Core, options ...Option) *Logger {
	if core == nil {
		return nil
	}
	log := &Logger{
		core: core,
		ring: newRandRing(12),
	}
	log = log.WithOptions(options...)
	log.startLoop()
	return log
}

// Close close Logger background loop.
// Without guarantee anything except exiting loop.
func (log *Logger) Close() {
	log.stopLoop()
}

// Named adds a new path segment to the logger's name. Segments are joined by
// periods. By default, Loggers are unnamed.
func (log *Logger) Named(s string) *Logger {
	if s == "" {
		return log
	}
	l := log.clone()
	if log.name == "" {
		l.name = s
	} else {
		l.name = strings.Join([]string{l.name, s}, ".")
	}
	return l
}

// WithOptions clones the current Logger, applies the supplied Options, and
// returns the resulting Logger. It's safe to use concurrently.
func (log *Logger) WithOptions(opts ...Option) *Logger {
	c := log.clone()
	for _, opt := range opts {
		opt.apply(c)
	}
	return c
}

// With creates a child logger and adds structured context to it. Fields added
// to the child don't affect the parent, and vice versa.
func (log *Logger) With(fields ...Field) *Logger {
	if len(fields) == 0 {
		return log
	}
	l := log.clone()
	l.core = l.core.With(fields)
	return l
}

// Check returns a CheckedEntry if logging a message at the specified level
// is enabled. It's a completely optional optimization; in high-performance
// applications, Check can help avoid allocating a slice to hold fields.
func (log *Logger) Check(lvl zapcore.Level, msg string) *zapcore.CheckedEntry {
	return log.check(lvl, msg)
}

func (log *Logger) startLoop() {
	log.loopCtx, log.loopCancel = context.WithCancel(context.Background())
	log.loopWg.Add(1)
	go log.writeLoop()
}

func (log *Logger) stopLoop() {
	log.loopCancel()
	log.loopWg.Wait()
}

func (log *Logger) writeLoop() {

	defer log.loopWg.Done()

	ctx, cancel := context.WithCancel(log.loopCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			b, ok := log.ring.TryPop()
			if !ok {
				time.Sleep(2 * time.Millisecond) // If no log, wait for 2 millisecond.
				continue
			}
			lb := (*logBody)(b)
			if ce := log.check(lb.lvl, lb.msg); ce != nil {
				ce.Write()
			}
			lb.free()
		}
	}
}

// Debug logs a message at DebugLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *Logger) Debug(msg string, fields ...Field) {
	if !log.core.Enabled(DebugLevel) { // Fast check.
		return
	}

	lb := getLogBody()
	lb.msg = msg
	lb.lvl = DebugLevel

	log.ring.Push(unsafe.Pointer(lb))
}

// Info logs a message at InfoLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *Logger) Info(msg string, fields ...Field) {

	lb := getLogBody()
	lb.msg = msg
	lb.lvl = InfoLevel

	log.ring.Push(unsafe.Pointer(lb))
}

// Warn logs a message at WarnLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *Logger) Warn(msg string, fields ...Field) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = WarnLevel

	log.ring.Push(unsafe.Pointer(lb))
}

// Error logs a message at ErrorLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *Logger) Error(msg string, fields ...Field) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = ErrorLevel

	log.ring.Push(unsafe.Pointer(lb))
}

// Panic logs a message at PanicLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then panics, even if logging at PanicLevel is disabled.
func (log *Logger) Panic(msg string, fields ...Field) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = PanicLevel

	log.ring.Push(unsafe.Pointer(lb))
}

// Fatal logs a message at FatalLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then calls os.Exit(1), even if logging at FatalLevel is
// disabled.
func (log *Logger) Fatal(msg string, fields ...Field) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = FatalLevel

	log.ring.Push(unsafe.Pointer(lb))
}

// Sync calls the underlying Core's Sync method, flushing any buffered log
// entries. Applications should take care to call Sync before exiting.
func (log *Logger) Sync() error {
	return log.core.Sync()
}

// Core returns the Logger's underlying zapcore.Core.
func (log *Logger) Core() zapcore.Core {
	return log.core
}

func (log *Logger) clone() *Logger {
	copy := *log
	return &copy
}

func (log *Logger) check(lvl zapcore.Level, msg string) *zapcore.CheckedEntry {

	// Create basic checked entry thru the core; this will be non-nil if the
	// log message will actually be written somewhere.
	ent := zapcore.Entry{
		LoggerName: log.name,
		Time:       tsc.UnixNano(),
		Level:      lvl,
		Message:    msg,
	}
	ce := log.core.Check(ent, nil)
	willWrite := ce != nil

	// Set up any required terminal behavior.
	switch ent.Level {
	case zapcore.PanicLevel:
		ce = ce.Should(ent, zapcore.WriteThenPanic)
	case zapcore.FatalLevel:
		ce = ce.Should(ent, zapcore.WriteThenFatal)

	}

	// Only do further annotation if we're going to write this message; checked
	// entries that exist only for terminal behavior don't benefit from
	// annotation.
	if !willWrite {
		return ce
	}

	return ce
}
