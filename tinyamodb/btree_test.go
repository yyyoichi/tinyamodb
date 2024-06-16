package tinyamodb

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBtree(t *testing.T) {
	t.Run("btree item marshal", func(t *testing.T) {
		t.Parallel()
		item := btreeItem{rawSk: "sk", rawPk: "pk", segmentId: 1}
		data, err := item.Value()
		require.NoError(t, err)
		var got btreeItem
		err = got.Unmarshal(data)
		require.NoError(t, err)
		require.Equal(t, item, got)

		var empty btreeItem
		data[0] = '1'
		err = empty.Unmarshal(data)
		require.NoError(t, err)
		require.Empty(t, empty)
	})
	t.Run("Append and Read", func(t *testing.T) {
		t.Parallel()
		var c Config
		dir, err := os.MkdirTemp("", "test-btree-index-write-read")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		bi, err := newBtreeIndex(dir, "tt", c)
		require.NoError(t, err)

		// double
		err = bi.Append("pk", "1", 1)
		require.NoError(t, err)
		err = bi.Append("pk", "1", 2)
		require.NoError(t, err)
		err = bi.Append("pk", "2", 3)
		require.NoError(t, err)
		// test
		segId, _ := bi.Read("pk", "1")
		require.EqualValues(t, 2, segId)
		segId, _ = bi.Read("pk", "2")
		require.EqualValues(t, 3, segId)

		err = bi.Close()
		require.NoError(t, err)

		bi, err = newBtreeIndex(dir, "tt", c)
		require.NoError(t, err)

		segId, _ = bi.Read("pk", "1")
		require.EqualValues(t, 2, segId)
		segId, _ = bi.Read("pk", "2")
		require.EqualValues(t, 3, segId)
	})
}
