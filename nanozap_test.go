// Copyright (c) 2016 Uber Technologies, Inc.
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
	"io/ioutil"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/zaibyte/nanozap/zapcore"
)

func withBenchedLogger(b *testing.B, f func(*Logger)) {
	logger := New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(defaultEncoderConf()),
			&Discarder{},
			DebugLevel,
		))
	defer logger.Close()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			f(logger)
		}
	})
}

// A Syncer is a spy for the Sync portion of zapcore.WriteSyncer.
type Syncer struct {
	err    error
	called bool
}

// SetError sets the error that the Sync method will return.
func (s *Syncer) SetError(err error) {
	s.err = err
}

// Sync records that it was called, then returns the user-supplied error (if
// any).
func (s *Syncer) Sync() error {
	s.called = true
	return s.err
}

// Called reports whether the Sync method was called.
func (s *Syncer) Called() bool {
	return s.called
}

// A Discarder sends all writes to ioutil.Discard.
type Discarder struct{ Syncer }

// Write implements io.Writer.
func (d *Discarder) Write(b []byte) (int, error) {
	return ioutil.Discard.Write(b)
}

// default without caller and stack trace,
func defaultEncoderConf() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		MessageKey:     "msg",
		ReqIDKey:       "reqid",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.EpochNanosTimeEncoder,
		EncodeDuration: zapcore.NanosDurationEncoder,
	}
}

func BenchmarkLogger_Info_Parallel(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("", "Ten fields, passed at the log site.")

	})
}

func BenchmarkLogger_Info(b *testing.B) {
	logger := New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(defaultEncoderConf()),
			&Discarder{},
			DebugLevel,
		))

	defer logger.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info("", "logger_info")
	}
}

// check no goroutine leak
func TestLogger_Close(t *testing.T) {

	defer goleak.VerifyNone(t)

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(defaultEncoderConf()),
		&Discarder{},
		DebugLevel,
	)

	logger := New(core)

	logger.Info("reqid", "logger_info")
	time.Sleep(2 * time.Millisecond)

	logger.Close()
}
