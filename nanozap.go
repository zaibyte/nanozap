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
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/templexxx/tsc"
	"github.com/zaibyte/nanozap/zapcore"
)

// For nanozap, the random order only happens when there is a flood,
// and random order logs won't cause serious issues.
type Logger struct {
	core zapcore.Core

	ring *Ring

	loopCtx    context.Context
	loopCancel func()
	loopWg     sync.WaitGroup
}

// New constructs a new Logger from the provided zapcore.Core. If
// the passed zapcore.Core is nil, it panic.
func New(core zapcore.Core) *Logger {
	if core == nil {
		panic("empty core")
	}
	log := &Logger{
		core: core,
		ring: newRandRing(12),
	}

	log.startLoop()
	return log
}

// Close close Logger background loop.
// Without guarantee anything except exiting loop.
func (log *Logger) Close() {
	log.stopLoop()
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
			if ce := log.check(lb.lvl, lb.msg, lb.reqid); ce != nil {
				ce.Write()
			}
			lb.free()
		}
	}
}

// Debug logs a message at DebugLevel.
func (log *Logger) Debug(reqid, msg string) {
	// Fast check. Debug level is a special case, because we usually use it in developing,
	// then close it in production env. There maybe lots of Enabled test, return it early.
	if !log.core.Enabled(DebugLevel) {
		return
	}

	lb := getLogBody()
	lb.msg = msg
	lb.lvl = DebugLevel
	lb.reqid = reqid

	log.ring.Push(unsafe.Pointer(lb))
}

func (log *Logger) Debugf(reqid, format string, args ...interface{}) {
	// Fast check. Debug level is a special case, because we usually use it in developing,
	// then close it in production env. There maybe lots of Enabled test, return it early.
	if !log.core.Enabled(DebugLevel) {
		return
	}

	lb := getLogBody()
	lb.msg = fmt.Sprintf(format, args...)
	lb.lvl = DebugLevel
	lb.reqid = reqid

	log.ring.Push(unsafe.Pointer(lb))
}

// Info logs a message at InfoLevel.
func (log *Logger) Info(reqid, msg string) {

	lb := getLogBody()
	lb.msg = msg
	lb.lvl = InfoLevel
	lb.reqid = reqid

	log.ring.Push(unsafe.Pointer(lb))
}

func (log *Logger) Infof(reqid, format string, args ...interface{}) {

	log.Info(reqid, fmt.Sprintf(format, args...))
}

// Warn logs a message at WarnLevel.
func (log *Logger) Warn(reqid, msg string) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = WarnLevel
	lb.reqid = reqid

	log.ring.Push(unsafe.Pointer(lb))
}

func (log *Logger) Warnf(reqid, format string, args ...interface{}) {

	log.Warn(reqid, fmt.Sprintf(format, args...))
}

// Error logs a message at ErrorLevel.
func (log *Logger) Error(reqid, msg string) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = ErrorLevel
	lb.reqid = reqid

	log.ring.Push(unsafe.Pointer(lb))
}

func (log *Logger) Errorf(reqid, format string, args ...interface{}) {

	log.Error(reqid, fmt.Sprintf(format, args...))
}

// Panic logs a message at PanicLevel. T
//
// The logger then panics, even if logging at PanicLevel is disabled.
func (log *Logger) Panic(reqid, msg string) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = PanicLevel
	lb.reqid = reqid

	log.ring.Push(unsafe.Pointer(lb))
}

func (log *Logger) Panicf(reqid, format string, args ...interface{}) {

	log.Panic(reqid, fmt.Sprintf(format, args...))
}

// Fatal logs a message at FatalLevel.
//
// The logger then calls os.Exit(1), even if logging at FatalLevel is
// disabled.
func (log *Logger) Fatal(reqid, msg string) {
	lb := getLogBody()
	lb.msg = msg
	lb.lvl = FatalLevel
	lb.reqid = reqid

	log.ring.Push(unsafe.Pointer(lb))
}

func (log *Logger) Fatalf(reqid, format string, args ...interface{}) {

	log.Fatal(reqid, fmt.Sprintf(format, args...))
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

func (log *Logger) check(lvl zapcore.Level, msg, reqid string) *zapcore.CheckedEntry {

	// Create basic checked entry thru the core; this will be non-nil if the
	// log message will actually be written somewhere.
	ent := zapcore.Entry{
		Time:    tsc.UnixNano(),
		Level:   lvl,
		Message: msg,
		ReqID:   reqid,
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
