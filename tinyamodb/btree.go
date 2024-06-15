package tinyamodb

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/btree"
)

type btreeIndex struct {
	mu     sync.RWMutex
	dir    string
	config Config

	tree *btree.BTreeG[*btreeItem]

	activeStore *bstore
	stores      []*bstore
}

func newBtreeIndex(dir string, sortKey string, c Config) (*btreeIndex, error) {
	if c.Segment.MaxStoreBytes == 0 {
		c.Segment.MaxStoreBytes = 1024
	}
	bi := &btreeIndex{
		dir:    fmt.Sprintf("%s/%s", dir, sortKey),
		config: c,
		tree:   btree.NewG(10, func(a, b *btreeItem) bool { return a.rawSk < b.rawSk }),
	}
	if _, err := os.Stat(bi.dir); err != nil {
		if err = os.Mkdir(bi.dir, 0755); err != nil {
			return nil, err
		}
	}
	return bi, bi.setup()
}

func (bi *btreeIndex) getStore(id int64) *bstore {
	return bi.stores[id-1]
}

func (bi *btreeIndex) setup() error {
	files, err := os.ReadDir(bi.dir)
	if err != nil {
		return err
	}
	storeIds := make([]int64, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		strbStoreId := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		storeId, _ := strconv.Atoi(strbStoreId)
		if storeId == 0 {
			continue
		}

		storeIds = append(storeIds, int64(storeId))
	}
	slices.Sort(storeIds)
	for _, id := range storeIds {
		if err := bi.newStore(id); err != nil {
			return err
		}
	}
	if bi.activeStore == nil {
		if err := bi.newStore(0); err != nil {
			return nil
		}
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	for i, store := range bi.stores {
		storeId := i + 1
		for item := range store.ReadBtreeItem(ctx, func(err error) { cancel(err) }) {
			if item.rawPk == "" {
				continue
			}
			item.storeId = int64(storeId)
			_, _ = bi.tree.ReplaceOrInsert(item)
		}
	}
	return nil
}

func (bi *btreeIndex) newStore(storeId int64) error {
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
	bs := &bstore{s}
	bi.stores = append(bi.stores, bs)
	if s.size < bi.config.Segment.MaxStoreBytes {
		bi.activeStore = bs
	}
	return nil
}

const (
	treeFlagWidth = 1
)

type bstore struct {
	*store
}

func (s *bstore) Delete(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	f := []byte{'1'}
	if _, err := s.File.WriteAt(f, int64(pos+treeFlagWidth)); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *bstore) ReadBtreeItem(ctx context.Context, errHandle func(err error)) <-chan *btreeItem {
	dataCh := s.ReadAll(ctx, errHandle)
	ch := make(chan *btreeItem)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case data := <-dataCh:
				var item = &btreeItem{
					storePos: data.pos,
				}
				if err := item.Unmarshal(data.data); err != nil {
					errHandle(err)
				}
				ch <- item
			}
		}
	}()
	return ch
}

type btreeItem struct {
	rawSk     string // sort key value
	rawPk     string // partition key value
	segmentId int64  // index
	storeId   int64  // itself
	storePos  uint64 // itself
}

func (i *btreeItem) Value() ([]byte, error) {
	av := &types.AttributeValueMemberM{
		Value: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: i.rawPk},
			"sk": &types.AttributeValueMemberS{Value: i.rawSk},
			"sg": &types.AttributeValueMemberN{Value: strconv.Itoa(int(i.segmentId))},
		},
	}
	var buf = new(bytes.Buffer)
	var e = newEncoder(prefixByteEncOption('0'))
	err := e.Encode(av, buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (i *btreeItem) Unmarshal(data []byte) error {
	var r = bytes.NewReader(data)
	var f byte
	var d = newDecoder(prefixByteDecOption(&f))
	av, err := d.Decode(r)
	if err != nil {
		return err
	}
	if f == '1' {
		return nil
	}
	avm, ok := av.(*types.AttributeValueMemberM)
	if !ok {
		return ErrCannotUnmarshal
	}
	pk := avm.Value["pk"].(*types.AttributeValueMemberS)
	i.rawPk = pk.Value
	sk := avm.Value["sk"].(*types.AttributeValueMemberS)
	i.rawSk = sk.Value
	sg := avm.Value["sg"].(*types.AttributeValueMemberN)
	sgId, _ := strconv.Atoi(sg.Value)
	i.segmentId = int64(sgId)
	return nil
}
