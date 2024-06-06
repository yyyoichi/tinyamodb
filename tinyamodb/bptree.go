package tinyamodb

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/google/btree"
)

type bptreeInterface interface {
}
type bptreeItemInterface interface {
	Less(than bptreeItemInterface) bool
}

type btreeItem struct {
	rawSk     string // sort key value
	segmentId int64  // index
	offset    int32  // index

	less func(than bptreeItemInterface) bool

	storeId     int64 // itself
	storeOffset int64 // itself
}

func (i *btreeItem) Less(than bptreeItemInterface) bool { return i.less(than) }

type bptreeIndex struct {
	mu     sync.RWMutex
	dir    string
	config Config

	tree *btree.BTree

	activeStore *store
	stores      []*store
}

func newBptreeIndex(dir string, sortKey string, c Config) (*bptreeIndex, error) {
	bi := &bptreeIndex{
		dir:    fmt.Sprintf("%s/%s", dir, sortKey),
		config: c,
	}
	if _, err := os.Stat(bi.dir); err != nil {
		if err = os.Mkdir(bi.dir, 0755); err != nil {
			return nil, err
		}
	}
	return bi, bi.setup()
}

func (bi *bptreeIndex) getStore(id int64) *store {
	return bi.stores[id-1]
}

func (bi *bptreeIndex) setup() error {
	files, err := os.ReadDir(bi.dir)
	if err != nil {
		return err
	}
	storeId := make([]int64, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		strbStoreId := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		bStoreId, _ := strconv.Atoi(strbStoreId)
		if bStoreId == 0 {
			continue
		}

		storeId = append(storeId, int64(bStoreId))
	}
	slices.Sort(storeId)
	for _, id := range storeId {
		if err := bi.newStore(id); err != nil {
			return err
		}
	}
	if bi.activeStore == nil {
		if err := bi.newStore(0); err != nil {
			return nil
		}
	}
	return nil
}

func (bi *bptreeIndex) newStore(storeId int64) error {
	if storeId == 0 {
		storeId = int64(len(bi.stores) + 1)
	}
	storeFile, err := os.OpenFile(
		filepath.Join(bi.dir, fmt.Sprintf("%d.store", storeId)),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0600,
	)
	if err != nil {
		return err
	}
	s, err := newStore(storeFile)
	if err != nil {
		return err
	}
	bi.stores = append(bi.stores, s)
	if s.size < bi.config.Segment.MaxStoreBytes {
		bi.activeStore = s
	}
	return nil
}
