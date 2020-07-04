/*
 * Copyright (c) 2019. Temple3x (temple3x@gmail.com)
 * Copyright (c) 2014 Nate Finch
 *
 * Use of this source code is governed by the MIT License
 * that can be found in the LICENSE file.
 */

// zaproll provides a rolling logger for nanozap.
//
// zaproll use buffer writer to improve I/O throughput,
// and there is only one goroutine could write, so Mutex is light.
package zaproll

import (
	"container/heap"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/templexxx/fnc"
)

// Rotation is implement io.WriteCloser interface with func Sync() (err error).
type Rotation struct {
	locker sync.Mutex

	cfg *Config

	backups *Backups

	f       *os.File
	buf     *bufIO
	written int64 // Total written to file.
	dirty   int64 // Dirty page size.
	synced  int64 // Total flushed to disk.
}

// New creates a Rotation.
func New(cfg *Config) (r *Rotation, err error) {

	r, err = prepare(cfg)
	if err != nil {
		return
	}

	return
}

func prepare(cfg *Config) (r *Rotation, err error) {

	if cfg.OutputPath == "" {
		return nil, errors.New("empty log file path")
	}

	cfg.adjust()

	r = &Rotation{cfg: cfg}
	bs, err := listBackups(cfg.OutputPath, cfg.MaxBackups)
	if err != nil {
		return
	}
	r.backups = bs

	err = r.open()
	if err != nil {
		return
	}

	r.buf = newBufIO(r.f, int(r.cfg.PerWriteSize))

	return
}

// open opens a new log file.
// If log file existed, move it to backups.
func (r *Rotation) open() (err error) {

	fp := r.cfg.OutputPath

	if r.f != nil { // File exist may happen in rotation process.
		backupFP, t := makeBackupFP(fp, r.cfg.LocalTime, time.Now())
		err = os.Rename(fp, backupFP)
		if err != nil {
			return fmt.Errorf("failed to rename log file, output: %s backup: %s", fp, backupFP)
		}

		heap.Push(r.backups, Backup{t, backupFP})
		if r.backups.Len() > r.cfg.MaxBackups {
			v := heap.Pop(r.backups)
			os.Remove(v.(Backup).fp)
		}
	}

	// Create a new log file.
	dir := filepath.Dir(fp)
	err = os.MkdirAll(dir, 0755) // ensure we have created the right dir.
	if err != nil {
		return fmt.Errorf("failed to make dirs for log file: %s", err.Error())
	}
	// Truncate here to clean up file content if someone else creates
	// the file between exist checking and create file.
	// Can't use os.O_EXCL here, because it may break rotation process.
	//
	// Most of log shippers monitor file size, and APPEND only can avoid Read-Modify-Write.
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC | os.O_APPEND
	f, err := fnc.OpenFile(fp, flag, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %s", err.Error())
	}

	r.f = f
	return
}

// Write writes data to buffer then notify file write.
func (r *Rotation) Write(p []byte) (written int, err error) {

	r.locker.Lock()
	defer r.locker.Unlock()

	_, fw, _ := r.buf.write(p)
	r.dirty += int64(fw)
	r.written += int64(fw)

	if fw == 0 { // Nothing write to file, just memory copy.
		return len(p), nil
	}

	r.flushDirty(false)

	return len(p), nil
}

// Sync syncs all dirty data.
func (r *Rotation) Sync() (err error) {

	r.locker.Lock()
	defer r.locker.Unlock()

	fw, _ := r.buf.flush()
	r.dirty += int64(fw)
	r.written += int64(fw)

	r.flushDirty(true)

	return
}

func (r *Rotation) flushDirty(force bool) {
	if r.dirty >= r.cfg.PerSyncSize || force {

		fnc.FlushHint(r.f, r.synced, r.dirty)
		r.synced += r.dirty
		r.dirty = 0
	}

	if r.written >= r.cfg.MaxSize {
		oldF := r.f
		err := r.open()
		if err == nil {
			fnc.FlushHint(oldF, 0, r.cfg.MaxSize)
			fnc.DropCache(oldF, 0, r.cfg.MaxSize)
			oldF.Close()
			r.buf.reset(r.f)
		}

		r.dirty = 0
		r.written = 0
		r.synced = 0
	}
}

// Close closes Rotation and release all resources.
func (r *Rotation) Close() (err error) {

	if r.f != nil { // Just in case.
		return r.f.Close()
	}

	return
}
