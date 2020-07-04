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
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/templexxx/nanozap/zapcore"
)

type username string

func (n username) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("username", string(n))
	return nil
}

func assertCanBeReused(t testing.TB, field Field) {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		enc := zapcore.NewMapObjectEncoder()

		// Ensure using the field in multiple encoders in separate goroutines
		// does not cause any races or panics.
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotPanics(t, func() {
				field.AddTo(enc)
			}, "Reusing a field should not cause issues")
		}()
	}

	wg.Wait()
}

func TestFieldConstructors(t *testing.T) {
	// Interface types.
	addr := net.ParseIP("1.2.3.4")
	name := username("phil")
	ints := []int{5, 6}

	tests := []struct {
		name   string
		field  Field
		expect Field
	}{
		{"Skip", Field{Type: zapcore.SkipType}, Skip()},
		{"Binary", Field{Key: "k", Type: zapcore.BinaryType, Interface: []byte("ab12")}, Binary("k", []byte("ab12"))},
		{"Bool", Field{Key: "k", Type: zapcore.BoolType, Integer: 1}, Bool("k", true)},
		{"Bool", Field{Key: "k", Type: zapcore.BoolType, Integer: 1}, Bool("k", true)},
		{"ByteString", Field{Key: "k", Type: zapcore.ByteStringType, Interface: []byte("ab12")}, ByteString("k", []byte("ab12"))},
		{"Complex128", Field{Key: "k", Type: zapcore.Complex128Type, Interface: 1 + 2i}, Complex128("k", 1+2i)},
		{"Complex64", Field{Key: "k", Type: zapcore.Complex64Type, Interface: complex64(1 + 2i)}, Complex64("k", 1+2i)},
		{"Duration", Field{Key: "k", Type: zapcore.DurationType, Integer: 1}, Duration("k", 1)},
		{"Int", Field{Key: "k", Type: zapcore.Int64Type, Integer: 1}, Int("k", 1)},
		{"Int64", Field{Key: "k", Type: zapcore.Int64Type, Integer: 1}, Int64("k", 1)},
		{"Int32", Field{Key: "k", Type: zapcore.Int32Type, Integer: 1}, Int32("k", 1)},
		{"Int16", Field{Key: "k", Type: zapcore.Int16Type, Integer: 1}, Int16("k", 1)},
		{"Int8", Field{Key: "k", Type: zapcore.Int8Type, Integer: 1}, Int8("k", 1)},
		{"String", Field{Key: "k", Type: zapcore.StringType, String: "foo"}, String("k", "foo")},
		{"Time", Field{Key: "k", Type: zapcore.TimeType, Integer: 0}, Time("k", 0)},
		{"Time", Field{Key: "k", Type: zapcore.TimeType, Integer: 1000}, Time("k", 1000)},
		{"Uint", Field{Key: "k", Type: zapcore.Uint64Type, Integer: 1}, Uint("k", 1)},
		{"Uint64", Field{Key: "k", Type: zapcore.Uint64Type, Integer: 1}, Uint64("k", 1)},
		{"Uint32", Field{Key: "k", Type: zapcore.Uint32Type, Integer: 1}, Uint32("k", 1)},
		{"Uint16", Field{Key: "k", Type: zapcore.Uint16Type, Integer: 1}, Uint16("k", 1)},
		{"Uint8", Field{Key: "k", Type: zapcore.Uint8Type, Integer: 1}, Uint8("k", 1)},
		{"Uintptr", Field{Key: "k", Type: zapcore.UintptrType, Integer: 10}, Uintptr("k", 0xa)},
		{"Reflect", Field{Key: "k", Type: zapcore.ReflectType, Interface: ints}, Reflect("k", ints)},
		{"Stringer", Field{Key: "k", Type: zapcore.StringerType, Interface: addr}, Stringer("k", addr)},
		{"Object", Field{Key: "k", Type: zapcore.ObjectMarshalerType, Interface: name}, Object("k", name)},
		{"Namespace", Namespace("k"), Field{Key: "k", Type: zapcore.NamespaceType}},
	}

	for _, tt := range tests {
		if !assert.Equal(t, tt.expect, tt.field, "Unexpected output from convenience field constructor %s.", tt.name) {
			t.Logf("type expected: %T\nGot: %T", tt.expect.Interface, tt.field.Interface)
		}
		assertCanBeReused(t, tt.field)
	}
}
