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

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncStringSet(t *testing.T) {
	tElem1, tElem2, tElem3 := "test1", "test2", "test3"

	t.Run("new set is empty", func(t *testing.T) {
		ss := NewSyncStringSet()
		require.Empty(t, ss.GetAll())
	})

	t.Run("add element to set", func(t *testing.T) {
		ss := NewSyncStringSet()
		ss.Add(tElem1)
		assert.True(t, ss.Contains(tElem1))
		require.Equal(t, []string{tElem1}, ss.GetAll())

		ss.Add(tElem2)
		assert.True(t, ss.Contains(tElem1))
		assert.True(t, ss.Contains(tElem2))
		require.ElementsMatch(t, []string{tElem1, tElem2}, ss.GetAll())
	})

	t.Run("add duplicate elements to set", func(t *testing.T) {
		ss := NewSyncStringSet()
		ss.Add(tElem1)
		assert.True(t, ss.Contains(tElem1))
		assert.Equal(t, []string{tElem1}, ss.GetAll())

		ss.Add(tElem1)
		assert.True(t, ss.Contains(tElem1))
		assert.Equal(t, []string{tElem1}, ss.GetAll())
	})

	t.Run("delete element form set", func(t *testing.T) {
		ss := NewSyncStringSet()
		ss.Add(tElem1)
		ss.Add(tElem2)
		ss.Add(tElem3)

		ss.Delete(tElem2)
		assert.True(t, ss.Contains(tElem1))
		assert.False(t, ss.Contains(tElem2))
		assert.True(t, ss.Contains(tElem3))
		require.ElementsMatch(t, []string{tElem1, tElem3}, ss.GetAll())
	})

	t.Run("delete element twice", func(t *testing.T) {
		ss := NewSyncStringSet()
		ss.Add(tElem1)
		ss.Add(tElem2)

		ss.Delete(tElem2)
		assert.True(t, ss.Contains(tElem1))
		assert.False(t, ss.Contains(tElem2))
		require.Equal(t, []string{tElem1}, ss.GetAll())

		ss.Delete(tElem2)
		assert.True(t, ss.Contains(tElem1))
		assert.False(t, ss.Contains(tElem2))
		require.Equal(t, []string{tElem1}, ss.GetAll())
	})

}
