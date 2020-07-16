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
	"sync"

	"github.com/zaibyte/nanozap/zapcore"
)

var (
	_logBodyPool = newLogBodyPool()
	getLogBody   = _logBodyPool.get
)

type logBody struct {
	lvl   zapcore.Level
	msg   string
	reqid string
	pool  logBodyPool
}

func (b *logBody) free() {
	b.pool.put(b)
}

func (b *logBody) reset() {
	b.lvl = InfoLevel
	b.msg = ""
	b.reqid = ""
}

type logBodyPool struct {
	p *sync.Pool
}

func newLogBodyPool() logBodyPool {
	return logBodyPool{p: &sync.Pool{
		New: func() interface{} {
			return &logBody{
				lvl:   InfoLevel,
				msg:   "",
				reqid: "",
			}
		},
	}}
}

func (p logBodyPool) get() *logBody {
	buf := p.p.Get().(*logBody)
	buf.reset()
	buf.pool = p
	return buf
}

func (p logBodyPool) put(buf *logBody) {
	p.p.Put(buf)
}
