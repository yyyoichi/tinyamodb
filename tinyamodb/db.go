package tinyamodb

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

type TinyamoDb interface {
	PutKey(context.Context, string) (*PutKeyItemOutput, error)
	DeleteKey(context.Context, string) (*DeleteKeyItemOutput, error)
	ReadKey(context.Context, string) (*ReadKeyItemOutput, error)
	Close() error
}

type db struct {
	TinyamoDb
	// partition id start with 1
	partitions map[int]*partition
}

func New(dir string, c Config) (TinyamoDb, error) {
	if _, err := os.Stat(dir); err != nil {
		if err = os.Mkdir(dir, 0755); err != nil {
			return nil, err
		}
	}

	db := &db{
		partitions: make(map[int]*partition),
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

func (db *db) PutKey(ctx context.Context, key string) (*PutKeyItemOutput, error) {
	item := NewKeyTimeItem(key)
	p := db.determinePartition(item.sha256Key)
	old, err := p.Put(item)
	if err != nil {
		return nil, err
	}
	olditem := old.(*KeyTimeItem)
	return &PutKeyItemOutput{Key: &olditem.RawKey}, nil
}
func (db *db) ReadKey(ctx context.Context, key string) (*ReadKeyItemOutput, error) {
	item := NewKeyTimeItem(key)
	p := db.determinePartition(item.sha256Key)
	err := p.Read(item)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return &ReadKeyItemOutput{Key: nil}, nil
		}
		return nil, err
	}
	return &ReadKeyItemOutput{Key: &item.RawKey}, nil
}
func (db *db) DeleteKey(ctx context.Context, key string) (*DeleteKeyItemOutput, error) {
	item := NewKeyTimeItem(key)
	p := db.determinePartition(item.sha256Key)
	old, err := p.Delete(item)
	if err != nil {
		return nil, err
	}
	olditem := old.(*KeyTimeItem)
	return &DeleteKeyItemOutput{Key: &olditem.RawKey}, nil
}
func (db *db) Close() error {
	for _, p := range db.partitions {
		if err := p.Close(); err != nil {
			return err
		}
	}
	return nil
}
func (db *db) determinePartition(sha256key []byte) *partition {
	v := binary.BigEndian.Uint32(sha256key[:4])
	id := int(v) % len(db.partitions)
	// partition id start with 1
	return db.partitions[id+1]
}