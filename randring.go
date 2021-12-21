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

// Package randring provides a ring buckets for multi-producer & one-consumer
// which will drop messages if buckets full and won't guarantee order.
// randring only cares about memory corruption.
package nanozap

import (
	"sync/atomic"
	"unsafe"

	"github.com/templexxx/cpu"
)

const falseSharingRange = cpu.X86FalseSharingRange

// Ring provides a ring buckets for multi-producer & one-consumer.
type Ring struct {
	mask       uint64
	_          [falseSharingRange]byte
	writeIndex uint64
	_          [falseSharingRange]byte

	// writeIndex cache for Pop, only get new write index when read catch write.
	// Help to reduce caching missing.
	writeIndexCache uint64
	readIndex       uint64

	buckets []unsafe.Pointer
}

// New creates a ring.
// ring size = 2 ^ n.
func newRandRing(n uint64) *Ring {

	if n > 16 || n == 0 {
		panic("illegal ring size")
	}

	r := &Ring{
		buckets: make([]unsafe.Pointer, 1<<n),
		mask:    (1 << n) - 1,
	}

	r.writeIndex = ^r.writeIndex
	return r
}

// Push puts the data in ring in the next bucket no matter what in it.
func (r *Ring) Push(data unsafe.Pointer) {
	idx := atomic.AddUint64(&r.writeIndex, 1) & r.mask
	old := atomic.SwapPointer(&r.buckets[idx], data)
	if old != nil {
		lb := (*logBody)(old)
		lb.free()
	}
}

// TryPop tries to pop data from the next bucket,
// return (nil, false) if no data available.
func (r *Ring) TryPop() (unsafe.Pointer, bool) {

	if r.readIndex >= r.writeIndexCache {
		r.writeIndexCache = atomic.LoadUint64(&r.writeIndex)
		if r.readIndex >= r.writeIndexCache {
			return nil, false
		}
	}

	idx := r.readIndex & r.mask
	data := atomic.SwapPointer(&r.buckets[idx], nil)

	if data == nil {
		return nil, false
	}

	r.readIndex++
	return data, true
}
