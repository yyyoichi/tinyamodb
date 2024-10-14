package tinyamodb

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

func TestTinyamoDb(t *testing.T) {
	test := []Config{
		func() Config {
			var c Config
			c.Partition.Num = 8
			c.Table.PartitionKey = "key"
			return c
		}(),
		func() Config {
			var c Config
			c.Partition.Num = 8
			c.Table.PartitionKey = "key"
			c.Table.SortKey = "s"
			return c
		}(),
	}
	for _, c := range test {
		dir, err := os.MkdirTemp("", "test-db")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		db, err := New(dir, c)
		require.NoError(t, err)

		// put
		testPutItem(t, db)
		testGetItem(t, db)

		// overwrite
		testPutItem(t, db)
		testGetItem(t, db)

		err = db.Close()
		require.NoError(t, err)

		db, err = New(dir, c)
		require.NoError(t, err)
		testGetItem(t, db)

		// delete
		testDeleteItem(t, db)
	}
}

func testPutItem(t *testing.T, db *Db) {
	t.Helper()
	for _, v := range getAttributeValues() {
		_, err := db.PutItem(context.Background(), &PutItemInput{
			Item: v,
		})
		require.NoError(t, err)
	}
}

func testGetItem(t *testing.T, db *Db) {
	for _, v := range getAttributeValues() {
		output, err := db.GetItem(context.Background(), &GetItemInput{
			Key: v,
		})
		require.NoError(t, err)
		require.Equal(t, v, output.Item)
	}
}

func testDeleteItem(t *testing.T, db *Db) {
	t.Helper()
	for _, v := range getAttributeValues() {
		_, err := db.DeleteItem(context.Background(), &DeleteItemInput{
			Key: v,
		})
		require.NoError(t, err)
	}
	for _, v := range getAttributeValues() {
		output, err := db.GetItem(context.Background(), &GetItemInput{
			Key: v,
		})
		require.NoError(t, err)
		require.NotNil(t, output.Item)
	}
}

var getAttributeValues = func() []map[string]types.AttributeValue {
	return []map[string]types.AttributeValue{
		{
			"key": &types.AttributeValueMemberS{
				Value: "first",
			},
			"s": &types.AttributeValueMemberS{
				Value: "000",
			},
			"doc": &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberN{
						Value: "1.23",
					},
					&types.AttributeValueMemberN{
						Value: "98.7",
					},
				},
			},
		},
		{
			"key": &types.AttributeValueMemberS{
				Value: "second",
			},
			"s": &types.AttributeValueMemberS{
				Value: "001",
			},
			"doc": &types.AttributeValueMemberBOOL{
				Value: true,
			},
		},
		{
			"key": &types.AttributeValueMemberS{
				Value: "thrid",
			},
			"s": &types.AttributeValueMemberS{
				Value: "002",
			},
			"doc": &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"name": &types.AttributeValueMemberS{
						Value: "taro",
					},
					"age": &types.AttributeValueMemberN{
						Value: "10",
					},
				},
			},
		},
	}
}
