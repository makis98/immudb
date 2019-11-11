/*
Copyright 2019 vChain, Inc.

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

package db

import (
	"io/ioutil"
	"log"
	"strconv"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
)

func makeBadger() *badger.DB {

	dir, err := ioutil.TempDir("", "badger")
	if err != nil {
		log.Fatal(err)
	}

	opts := badger.
		DefaultOptions(dir).
		WithKeepL0InMemory(true).
		WithSyncWrites(false)

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func TestTopic(t *testing.T) {
	db := makeBadger()
	defer db.Close()

	topic := NewTopic(db)

	for n := uint64(0); n <= 64; n++ {
		key := strconv.FormatUint(n, 10)
		err := topic.Set("key"+key, []byte(key))

		assert.NoError(t, err)

		// assert.Equal(t, n, tr.N())
		// d := uint64(math.Ceil(math.Log2(float64(n + 1))))
		// assert.Equal(t, d, tr.Depth())

		// assert.Equal(t, testRoots[n], tr.Root())

		// // internal state
		// assert.Len(t, tr.data[tr.Depth()], 1)
	}
}

func BenchmarkTreeAdd(b *testing.B) {
	db := makeBadger()
	defer db.Close()
	topic := NewTopic(db)

	for i := 0; i < b.N; i++ {
		topic.Set("key"+strconv.FormatUint(uint64(i), 10), []byte{0, 1, 3, 4, 5, 6, 7})
	}
}