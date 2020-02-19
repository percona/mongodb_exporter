// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

import "sync"

// String set synchronized with sync.RWMutex
type SyncStringSet struct {
	m  map[string]struct{}
	mu sync.RWMutex
}

// NewSyncStringSet returns new SyncStringSet
func NewSyncStringSet() *SyncStringSet {
	return &SyncStringSet{
		m: make(map[string]struct{}),
	}
}

// Add the value to the set.
func (l *SyncStringSet) Add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.m[s] = struct{}{}
}

// Delete value from the set
func (l *SyncStringSet) Delete(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.m, s)
}

// Contains check whether set contains the value
func (l *SyncStringSet) Contains(s string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	_, ok := l.m[s]
	return ok
}

// GetAll returns slice of all values stored in the set
func (l *SyncStringSet) GetAll() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var res = make([]string, 0, len(l.m))
	for k := range l.m {
		res = append(res, k)
	}

	return res
}
