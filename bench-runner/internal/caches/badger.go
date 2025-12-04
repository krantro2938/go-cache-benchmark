package caches

import (
	"sync"

	"github.com/dgraph-io/badger/v4"
)

type BadgerCache struct {
	db   *badger.DB
	hits int64
	miss int64
	mu   sync.RWMutex
}

func NewBadgerCache() (*BadgerCache, error) {
	opts := badger.Options{}.
		WithInMemory(true).
		// Critical: set memory table size explicitly
		WithMemTableSize(64 << 20).    // 64 MB
		WithBaseTableSize(16 << 20).   // 16 MB
		WithValueLogFileSize(1 << 20). // 1 MB (minimum valid)
		WithLogger(nil).
		WithCompactL0OnClose(false)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerCache{db: db}, nil
}

func (b *BadgerCache) Set(key string, value []byte, _ int64) bool {
	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	return err == nil
}

func (b *BadgerCache) Get(key string) ([]byte, bool) {
	var result []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			result = append([]byte(nil), val...)
			return nil
		})
	})
	if err != nil {
		b.mu.Lock()
		b.miss++
		b.mu.Unlock()
		return nil, false
	}
	b.mu.Lock()
	b.hits++
	b.mu.Unlock()
	return result, true
}

func (b *BadgerCache) Close() error {
	return b.db.Close()
}

func (b *BadgerCache) Metrics() Metrics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Metrics{
		HitCount:  b.hits,
		MissCount: b.miss,
	}
}

func (b *BadgerCache) Name() string {
	return "Badger"
}
