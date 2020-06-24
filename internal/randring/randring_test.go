package randring

import (
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func TestRing_Push(t *testing.T) {
	r := New(2)
	data := []byte{'1'}
	for i := 0; i < 5; i++ { // Ensure it's ok to set more than size.
		r.Push(unsafe.Pointer(&data))
	}
}

func TestRing_TryPop(t *testing.T) {
	r := New(2)
	for i := 0; i < 4; i++ {
		v := i
		r.Push(unsafe.Pointer(&v))
	}
	for i := 0; i < 4; i++ {
		v, ok := r.TryPop()
		if !ok {
			t.Fatal("should ok")
		}
		if *(*int)(v) != i {
			t.Fatal("mismatch", *(*int)(v), i)
		}
	}
}

func TestRing_TryPopConcurrent(t *testing.T) {

	r := New(8)

	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Push(unsafe.Pointer(&i))
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < runtime.NumCPU(); i++ {
			time.Sleep(time.Millisecond)
			_, ok := r.TryPop()
			if !ok {
				t.Fatal("should ok")
			}
		}
	}()

	wg.Wait()
}

func TestRing_TryPopAhead(t *testing.T) {
	r := New(2)
	for i := 0; i < 4; i++ {
		v := i
		r.Push(unsafe.Pointer(&v))
	}
	for i := 0; i < 4; i++ {
		v, ok := r.TryPop()
		if !ok {
			t.Fatal("should ok")
		}
		if *(*int)(v) != i {
			t.Fatal("mismatch", *(*int)(v), i)
		}
	}

	_, ok := r.TryPop()
	if ok {
		t.Fatal("should not ok")
	}
}

func TestRing_SetAhead(t *testing.T) {

	r := New(2)
	for i := 0; i < 4+1; i++ {
		v := i
		r.Push(unsafe.Pointer(&v))
	}

	v, ok := r.TryPop()
	if !ok {
		t.Fatal("should ok")
	}
	if *(*int)(v) != 4 {
		t.Fatal("mismatch")
	}

	for i := 0; i < 3; i++ {
		v, ok := r.TryPop()
		if !ok {
			t.Fatal("should ok")
		}
		if *(*int)(v) != i+1 {
			t.Fatal("mismatch", *(*int)(v), i+1)
		}
	}
}

func BenchmarkRing_Push(b *testing.B) {

	r := New(8)

	p := 1

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.Push(unsafe.Pointer(&p))
		}
	})
}
