/*
 * Copyright (c) 2019. Temple3x (temple3x@gmail.com)
 * Copyright (c) 2014 Nate Finch
 *
 * Use of this source code is governed by the MIT License
 * that can be found in the LICENSE file.
 */

package zaproll

import (
	"container/heap"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backup holds backup log file' path & create time.
type Backup struct {
	ts int64
	fp string
}

// Backups implements heap interface.
type Backups struct {
	bs []Backup
}

func (b *Backups) Less(i, j int) bool {
	return (*b).bs[i].ts < ((*b).bs[j].ts)
}

func (b *Backups) Swap(i, j int) {
	if i >= 0 && j >= 0 {
		(*b).bs[i], (*b).bs[j] = (*b).bs[j], (*b).bs[i]
	}
}

func (b *Backups) Len() int {
	return len((*b).bs)
}

func (b *Backups) Pop() (v interface{}) {
	if b.Len() > 0 {
		v = (*b).bs[b.Len()-1]
		b.bs = (*b).bs[:b.Len()-1]
	}
	return
}

func (b *Backups) Push(v interface{}) {
	b.bs = append((*b).bs, v.(Backup))
}

func listBackups(outputPath string, max int) (*Backups, error) {
	bs := make([]Backup, 0, max*2) // Enough cap.
	b := &Backups{
		bs: bs,
	}
	err := b.list(outputPath, max)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// List all backup log files (in init process),
// and remove them if there are too many backups.
func (b *Backups) list(outputPath string, max int) error {

	dir := filepath.Dir(outputPath)

	// Ensure we have this directory.
	// If already has, do nothing.
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	ns, err := os.ReadDir(dir)
	if err != nil {
		return err // Path error
	}

	prefix, ext := getPrefixAndExt(outputPath)

	for _, f := range ns {
		if f.IsDir() {
			continue
		}
		if ts := parseTime(f.Name(), prefix, ext); ts != 0 {
			heap.Push(b, Backup{ts, filepath.Join(dir, f.Name())})
			continue
		}
	}

	for b.Len() > max {
		v := heap.Pop(b)
		os.Remove(v.(Backup).fp)
	}

	return nil
}

// getPrefixAndExt returns the filename part and extension part from the rotation's filename.
func getPrefixAndExt(outputPath string) (prefix, ext string) {
	name := filepath.Base(outputPath)
	ext = filepath.Ext(name)
	prefix = name[:len(name)-len(ext)] + "-"
	return prefix, ext
}

const backupTimeFmt = "2006-01-02T15:04:05.000Z0700"

// parseTime extracts the formatted time from the filename by stripping off
// the filename's prefix and extension.
//
// Return 0 if the file is illegal zaproll backup file.
func parseTime(fp, prefix, ext string) int64 {
	filename := filepath.Base(fp)
	if !strings.HasPrefix(filename, prefix) {
		return 0
	}
	if !strings.HasSuffix(filename, ext) {
		return 0
	}
	tsStr := filename[len(prefix) : len(filename)-len(ext)]
	t, err := time.Parse(backupTimeFmt, tsStr)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func makeBackupFP(name string, local bool, t time.Time) (string, int64) {
	dir := filepath.Dir(name)
	filename := filepath.Base(name)
	ext := filepath.Ext(filename)
	prefix := filename[:len(filename)-len(ext)]
	if !local {
		t = t.UTC()
	}

	timestamp := t.Format(backupTimeFmt)
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", prefix, timestamp, ext)), t.Unix()
}
