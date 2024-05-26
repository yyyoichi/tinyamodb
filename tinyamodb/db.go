package tinyamodb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

type Db struct {
	// partition id start with 1
	partitions map[int]*partition
	c          Config
}

func New(dir string, c Config) (*Db, error) {
	if _, err := os.Stat(dir); err != nil {
		if err = os.Mkdir(dir, 0755); err != nil {
			return nil, err
		}
	}

	db := &Db{
		partitions: make(map[int]*partition),
		c:          c,
	}

	// read from children dir.
	// cannot change partition num after init the database.
	children, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		if !child.IsDir() {
			continue
		}
		name := child.Name()
		id, _ := strconv.Atoi(name)
		if id == 0 {
			continue
		}
		db.partitions[id], err = newPartition(dir, id, c)
		if err != nil {
			return nil, err
		}
	}

	if l := len(db.partitions); l == 0 {
		// create
		if c.Partition.Num == 0 {
			c.Partition.Num = 10
		}
		for i := 1; i <= int(c.Partition.Num); i++ {
			db.partitions[i], err = newPartition(dir, i, c)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// serial
		for i := 1; i <= l; i++ {
			_, found := db.partitions[i]
			if !found {
				return nil, fmt.Errorf("unexpected error: partition '%d' is not found", i)
			}
		}
	}

	return db, nil
}

func (db *Db) Close() error {
	for _, p := range db.partitions {
		if err := p.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (db *Db) GetItem(ctx context.Context, input *GetItemInput) (*GetItemOutput, error) {
	item, err := newItem(input.Key, db.c)
	if err != nil {
		return nil, err
	}
	p := db.determinePartition(item)

	err = p.Read(item)
	if err != nil && !errors.Is(err, io.EOF) {
		if errors.Is(err, io.EOF) {
			return &GetItemOutput{Item: nil}, nil
		}
		return nil, err
	}
	return &GetItemOutput{Item: item.Item}, nil
}

func (db *Db) PutItem(ctx context.Context, input *PutItemInput) (*PutItemOutput, error) {
	item, err := newItem(input.Item, db.c)
	if err != nil {
		return nil, err
	}
	p := db.determinePartition(item)
	_, err = p.Put(item)
	if err != nil {
		return nil, err
	}
	return &PutItemOutput{}, nil
}

func (db *Db) DeleteItem(ctx context.Context, input *DeleteItemInput) (*DeleteItemOutput, error) {
	item, err := newItem(input.Key, db.c)
	if err != nil {
		return nil, err
	}
	p := db.determinePartition(item)
	_, err = p.Delete(item)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return &DeleteItemOutput{}, nil
}

func (db *Db) determinePartition(item *item) *partition {
	id := int(item.Pk4bit) % len(db.partitions)
	// partition id start with 1
	return db.partitions[id+1]
}
