/*
Copyright 2021 Roblox Corporation

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

package types

import (
	"regexp"
	"time"
)

type HealthCheck struct {
	Type    string    `json:"type"`
	Result  string    `json:"result"`
	Message string    `json:"message"`
	LastRun time.Time `json:"last_run"`
}

func (h *HealthCheck) Update(result, message string) {
	h.Result = result
	h.Message = message
	h.LastRun = time.Now()
}

type Config struct {
	Type        string `json:"type"`
	HealthCheck string `json:"health_check"`
}

// LogWatcherConfig represents the configuration for a log
// monitor.
type LogWatcherConfig struct {
	// The type of the source (for example: journald)
	Source string `json:"source"`
	// The value of the syslog identifier
	SyslogIdentifier string `json:"syslog_identifier"`
	// The rules associated with the given source
	Rules []LogWatcherRule `json:"rules"`
}

// LogWatcherRule represents individual rules for a log monitors.
type LogWatcherRule struct {
	// The name of the rule (will be used as a label in the metrics)
	Name string `json:"name"`
	// The pattern that is used to match a problem
	Pattern string `json:"pattern"`
	// The compiled regexp from the given patterns
	Regexp *regexp.Regexp
}

// LogMessage represents the events that are matching a log event.
type LogMessage struct {
	Name    string
	Message string
}

// LogWatcher is the interface to create a new log watcher.
type LogWatcher interface {
	Watch() <-chan *LogMessage
}
