package main

import (
	"testing"
)

func TestWatcher(t *testing.T) {
	w := NewWatcher()
	if w == nil {
		t.Fatal("Can not new watcher")
		return
	}
	defer w.Shutdown()
}
