package tinyamodb

import (
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

func TestPartitionPKSK(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-partition-pksk")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	const PARTITION_ID = 1
	var c Config
	c.Segment.MaxIndexBytes = entwidth
	c.Table.PartitionKey = "key"
	c.Table.SortKey = "order"
	p, err := newPartition(dir, PARTITION_ID, c)
	require.NoError(t, err)

	want0, err := newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key0"},
		"order": &types.AttributeValueMemberS{Value: "00"},
		"value": &types.AttributeValueMemberN{Value: "0"},
	}, c)
	require.NoError(t, err)
	_, err = p.Put(want0)
	require.NoError(t, err)

	want1, err := newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key0"},
		"order": &types.AttributeValueMemberS{Value: "01"},
		"value": &types.AttributeValueMemberN{Value: "1"},
	}, c)
	require.NoError(t, err)
	_, err = p.Put(want1)
	require.NoError(t, err)
	require.Equal(t, 2, len(p.segments))

	got0, err := newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key0"},
		"order": &types.AttributeValueMemberS{Value: "00"},
	}, c)
	require.NoError(t, err)
	err = p.Read(got0)
	require.NoError(t, err)
	require.Equal(t, want0.UnixNano, got0.UnixNano)
	err = p.Read(got0)
	require.NoError(t, err)
	require.Equal(t, want0.UnixNano, got0.UnixNano)

	// not found
	got2, err := newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key0"},
		"order": &types.AttributeValueMemberS{Value: "02"},
	}, c)
	require.NoError(t, err)
	err = p.Read(got2)
	require.Error(t, err)

	// close and open
	err = p.Close()
	require.NoError(t, err)

	p, err = newPartition(dir, PARTITION_ID, c)
	require.NoError(t, err)
	err = p.Read(got0)
	require.NoError(t, err)

	// overwrite
	got0, _ = newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key0"},
		"order": &types.AttributeValueMemberS{Value: "00"},
		"value": &types.AttributeValueMemberN{Value: "0"},
	}, c)
	_, err = p.Put(got0)
	require.NoError(t, err)
	require.NotEqual(t, want0.UnixNano, got0.UnixNano)

	// delete
	_, err = p.Delete(want0)
	require.NoError(t, err)
	// read
	err = p.Read(want0)
	require.ErrorIs(t, err, io.EOF)
	err = p.Read(want0)
	require.ErrorIs(t, err, io.EOF)
}

func TestPartitionPK(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-partition-pk")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	const PARTITION_ID = 1
	var c Config
	c.Segment.MaxIndexBytes = entwidth
	c.Table.PartitionKey = "key"
	p, err := newPartition(dir, PARTITION_ID, c)
	require.NoError(t, err)

	want00, err := newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key0"},
		"value": &types.AttributeValueMemberN{Value: "0"},
	}, c)
	require.NoError(t, err)
	_, err = p.Put(want00)
	require.NoError(t, err)

	want1, err := newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key1"},
		"value": &types.AttributeValueMemberN{Value: "0"},
	}, c)
	require.NoError(t, err)
	_, err = p.Put(want1)
	require.NoError(t, err)
	require.Equal(t, 2, len(p.segments))

	got0, err := newItem(map[string]types.AttributeValue{
		"key": &types.AttributeValueMemberS{Value: "key0"},
	}, c)
	require.NoError(t, err)
	err = p.Read(got0)
	require.NoError(t, err)
	require.Equal(t, want00.UnixNano, got0.UnixNano)
	err = p.Read(got0)
	require.NoError(t, err)
	require.Equal(t, want00.UnixNano, got0.UnixNano)

	// not found
	got2, err := newItem(map[string]types.AttributeValue{
		"key": &types.AttributeValueMemberS{Value: "key2"},
	}, c)
	require.NoError(t, err)
	err = p.Read(got2)
	require.Error(t, err)

	// close and open
	err = p.Close()
	require.NoError(t, err)

	p, err = newPartition(dir, PARTITION_ID, c)
	require.NoError(t, err)
	err = p.Read(got0)
	require.NoError(t, err)

	// overwrite
	got0, _ = newItem(map[string]types.AttributeValue{
		"key":   &types.AttributeValueMemberS{Value: "key0"},
		"value": &types.AttributeValueMemberN{Value: "1"},
	}, c)
	_, err = p.Put(got0)
	require.NoError(t, err)
	require.NotEqual(t, want00.UnixNano, got0.UnixNano)

	// delete
	_, err = p.Delete(want00)
	require.NoError(t, err)
	// read
	err = p.Read(want00)
	require.ErrorIs(t, err, io.EOF)
	err = p.Read(want00)
	require.ErrorIs(t, err, io.EOF)
}
