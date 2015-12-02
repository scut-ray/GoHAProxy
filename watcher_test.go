package main

import (
	"testing"
)

func TestWatcher(t *testing.T) {
	conf, _ := NewConfig("app.yaml")
	w := NewWatcher(conf)
	if w == nil {
		t.Fatal("Can not new watcher")
		return
	}
	defer w.Shutdown()
}
