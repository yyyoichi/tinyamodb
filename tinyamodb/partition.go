package tinyamodb

import (
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type partition struct {
	mu     sync.RWMutex
	dir    string
	config Config

	btreeIndex    *btreeIndex
	activeSegment *segment
	segments      []*segment
}

func newPartition(dir string, id int, c Config) (*partition, error) {
	if c.Segment.MaxStoreBytes == 0 {
		c.Segment.MaxStoreBytes = 1024
	}
	if c.Segment.MaxIndexBytes == 0 {
		c.Segment.MaxIndexBytes = 1024
	}
	p := &partition{
		dir:    fmt.Sprintf("%s/%d", dir, id),
		config: c,
	}
	if _, err := os.Stat(p.dir); err != nil {
		if err = os.Mkdir(p.dir, 0755); err != nil {
			return nil, err
		}
	}

	return p, p.setup()
}

func (p *partition) Put(item *item) (old *item, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return nil, p.write(item)
}

func (p *partition) Read(item *item) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, err := p.read(item)
	return err
}

func (p *partition) Delete(item *item) (*item, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := item.PrimaryKey()
	for _, s := range p.segments {
		if err := s.Delete(key); err != nil {
			return nil, err
		}
	}

	var pk, sk string = item.pk.Value, ""
	if p.config.Table.SortKey != "" {
		sk = item.sk.Value
	}

	_, err := p.btreeIndex.Delete(pk, sk)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (p *partition) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range p.segments {
		if err := s.Close(); err != nil {
			return err
		}
	}
	return p.btreeIndex.Close()
}

func (p *partition) read(item *item) (*segment, error) {
	var pk, sk string = item.pk.Value, ""
	if p.config.Table.SortKey != "" {
		sk = item.sk.Value
	}
	segId, err := p.btreeIndex.Read(pk, sk)
	if err != nil {
		return nil, err
	}
	if segId == 0 {
		return nil, io.EOF
	}
	s := p.getSegment(segId)
	key := item.PrimaryKey()
	data, err := s.Read(key)
	if err != nil {
		return nil, err
	}
	if err := item.Unmarshal(data); err != nil {
		item.Item = nil
		item.UnixNano = 0
		return nil, err
	}
	return s, nil
}

func (p *partition) write(item *item) error {
	if p.activeSegment.IsMaxed() {
		if err := p.newSegment(0); err != nil {
			return err
		}
	}

	key := item.PrimaryKey()
	data, err := item.Value()
	if err != nil {
		return err
	}
	if err = p.activeSegment.Write(key, data); err != nil {
		return err
	}

	var pk, sk string = item.pk.Value, ""
	if p.config.Table.SortKey != "" {
		sk = item.sk.Value
	}

	// set btree index
	if err = p.btreeIndex.Append(pk, sk, int64(len(p.segments))); err != nil {
		return err
	}
	return nil
}

func (p *partition) getSegment(id int64) *segment {
	return p.segments[id-1]
}

func (p *partition) setup() error {
	files, err := os.ReadDir(p.dir)
	if err != nil {
		return err
	}
	segmentIdMap := make(map[uint64]int, len(files)/2)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		strSegmentId := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		segmentId, _ := strconv.Atoi(strSegmentId)
		if segmentId == 0 {
			continue
		}

		segmentIdMap[uint64(segmentId)]++
	}

	segmentIds := make([]uint64, 0, len(segmentIdMap))
	for id, count := range segmentIdMap {
		if count != 2 {
			continue
		}
		segmentIds = append(segmentIds, id)
	}

	slices.Sort(segmentIds)
	for _, id := range segmentIds {
		if err = p.newSegment(id); err != nil {
			return err
		}
	}

	if p.activeSegment == nil {
		if err := p.newSegment(0); err != nil {
			return err
		}
	}

	// setup btree
	p.btreeIndex, err = newBtreeIndex(p.dir, p.config.Table.SortKey, p.config)
	if err != nil {
		return err
	}
	return nil
}

func (l *partition) newSegment(segmentId uint64) error {
	if segmentId == 0 {
		segmentId = uint64(len(l.segments)) + 1
	}
	s, err := newSegment(l.dir, segmentId, l.config)
	if err != nil {
		return err
	}
	l.segments = append(l.segments, s)
	if !s.IsMaxed() {
		l.activeSegment = s
	}
	return nil
}
