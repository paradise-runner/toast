package watcher

import (
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/yourusername/toast/internal/messages"
)

type Watcher struct {
	w    *fsnotify.Watcher
	send func(tea.Msg)
}

func New(send func(tea.Msg)) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watcher := &Watcher{w: w, send: send}
	go watcher.loop()
	return watcher, nil
}

func (wt *Watcher) Watch(path string) error   { return wt.w.Add(path) }
func (wt *Watcher) Unwatch(path string) error { return wt.w.Remove(path) }
func (wt *Watcher) Close()                    { wt.w.Close() }

func (wt *Watcher) loop() {
	for {
		select {
		case event, ok := <-wt.w.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				wt.send(messages.FileChangedOnDiskMsg{Path: event.Name})
			}
		case err, ok := <-wt.w.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
		}
	}
}
