/*
Copyright 2022 Roblox Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0


Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package journald

import (
	"fmt"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/coreos/go-systemd/sdjournal"
	types "github.com/nomad-node-problem-detector/types"
)

type journaldWatcher struct {
	journal *sdjournal.Journal
	config  *types.LogWatcherConfig
	status  chan *types.LogMessage
}

func NewJournaldWatcher(config *types.LogWatcherConfig) (types.LogWatcher, error) {
	journal, err := sdjournal.NewJournal()
	if err != nil {
		return nil, fmt.Errorf("failed to create journal client: %v", err)
	}

	seekTime := uint64(time.Now().UnixNano() / 1000)
	err = journal.SeekRealtimeUsec(seekTime)
	if err != nil {
		return nil, fmt.Errorf("failed to seek journal at %v: %v", seekTime, err)
	}

	match := sdjournal.Match{
		Field: sdjournal.SD_JOURNAL_FIELD_SYSLOG_IDENTIFIER,
		Value: config.SyslogIdentifier,
	}
	err = journal.AddMatch(match.String())
	if err != nil {
		return nil, fmt.Errorf("failed to add log filter %#v: %v", match, err)
	}

	watcher := &journaldWatcher{
		journal: journal,
		config:  config,
		status:  make(chan *types.LogMessage, 1000),
	}

	return watcher, nil
}

func (w *journaldWatcher) Watch() <-chan *types.LogMessage {
	for i := range w.config.Rules {
		w.config.Rules[i].Regexp = regexp.MustCompile(w.config.Rules[i].Pattern)
	}

	go w.monitorLoop()

	return w.status
}

const waitLogTimeOut = 5 * time.Second

func (w *journaldWatcher) monitorLoop() {
	defer func() {
		if err := w.journal.Close(); err != nil {
			log.Errorf("failed to close journal: %v", err)
		}
	}()

	for {
		n, err := w.journal.Next()
		if err != nil {
			log.Errorf("failed to get the next message from the journal", err)
			continue
		}

		if n == 0 {
			w.journal.Wait(waitLogTimeOut)
			continue
		}

		entry, err := w.journal.GetEntry()
		if err != nil {
			log.Errorf("failed to read the journal's entry: %v", err)
			continue
		}

		for _, rule := range w.config.Rules {
			matches := rule.Regexp.FindStringSubmatch(entry.Fields["MESSAGE"])
			if len(matches) == 0 {
				continue
			}

			l := &types.LogMessage{
				Name:    rule.Name,
				Message: entry.Fields["MESSAGE"],
			}
			w.status <- l
		}
	}
}
