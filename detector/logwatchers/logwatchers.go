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

package logwatchers

import (
	"fmt"
	journald "github.com/nomad-node-problem-detector/detector/logwatchers/journald"
	types "github.com/nomad-node-problem-detector/types"
	"regexp"
)

func GetLogWatcher(config *types.LogWatcherConfig) (types.LogWatcher, error) {
	// ensure that all the patterns are valid first - if they are not,
	// we report an error and stop here. We will let the different log
	// watcher handle how they process the patterns themselves.
	if err := ValidatePatterns(config); err != nil {
		return nil, fmt.Errorf("failed to validate a pattern: %s", err)
	}

	switch config.Source {
	case "journald":
		return journald.NewJournaldWatcher(config)
	default:
		return nil, fmt.Errorf("%s is not a supported source for watching logs", config.Source)
	}
}

func ValidatePatterns(config *types.LogWatcherConfig) error {
	for _, rule := range config.Rules {
		_, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("pattern %s for rule %s failed: %v", rule.Name, rule.Pattern, err)
		}
	}
	return nil
}
