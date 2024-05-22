package tinyamodb

import (
	"bytes"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

func TestItem(t *testing.T) {
	t.Run("new tinyamodb", func(t *testing.T) {
		test := []struct {
			item               map[string]types.AttributeValue
			configPK, configSK string
			expErr             error
		}{
			{map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "hoge"},
				"sk": &types.AttributeValueMemberS{Value: "fuga"}}, "pk", "sk", nil},
			{map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "hoge"},
				"sk": &types.AttributeValueMemberN{Value: "1.00"}}, "pk", "sk", nil},
			{map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "hoge"}}, "pk", "", nil},
			{map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "hoge"}}, "pk", "", nil},
			{map[string]types.AttributeValue{
				"k": &types.AttributeValueMemberS{Value: "hoge"}}, "pk", "", ErrNotFoundPartitionKey},
			{map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberN{Value: "1.00"}}, "pk", "", ErrInvalidPartitionKeyType},
			{map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "hoge"}}, "pk", "sk", ErrNotFoundSortKey},
			{map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "hoge"},
				"sk": &types.AttributeValueMemberBOOL{Value: true}}, "pk", "sk", ErrInvalidSortKeyType},
		}
		for _, tt := range test {
			var c Config
			c.Table.PartitionKey = tt.configPK
			c.Table.SortKey = tt.configSK
			_, err := NewTinyamoDbItem(tt.item, c)
			require.ErrorIs(t, err, tt.expErr)
		}
	})

	var e encoder
	var d decoder

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		want := "hello world"
		var b = new(bytes.Buffer)
		err := e.encodeString(want, b)
		require.NoError(t, err)
		got, err := d.decodeString(b)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("string set", func(t *testing.T) {
		t.Parallel()
		want := []string{"hoge", "fuga", "bar", "foo"}
		var b = new(bytes.Buffer)
		err := e.encodeSSet(want, b)
		require.NoError(t, err)
		got, err := d.decodeSSet(b)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("bytes", func(t *testing.T) {
		t.Parallel()
		want := []byte("hello world")
		var b = new(bytes.Buffer)
		err := e.encodeBytes(want, b)
		require.NoError(t, err)
		got, err := d.decodeBytes(b)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("bytes set", func(t *testing.T) {
		t.Parallel()
		want := [][]byte{[]byte("hoge"), []byte("fuga"), []byte("bar"), []byte("foo")}
		var b = new(bytes.Buffer)
		err := e.encodeBSet(want, b)
		require.NoError(t, err)
		got, err := d.decodeBSet(b)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("bool", func(t *testing.T) {
		t.Parallel()
		want := true
		var b = new(bytes.Buffer)
		err := e.encodeBool(want, b)
		require.NoError(t, err)
		got, err := d.decodeBool(b)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("attribute values", func(t *testing.T) {
		t.Parallel()
		test := map[string]types.AttributeValue{
			"string":     &types.AttributeValueMemberS{Value: "test string"},
			"string set": &types.AttributeValueMemberSS{Value: []string{"test", "string", "set"}},
			"number":     &types.AttributeValueMemberN{Value: "20.14"},
			"number set": &types.AttributeValueMemberNS{Value: []string{"20.14", "0", "1000000", "0.11111"}},
			"bytes":      &types.AttributeValueMemberB{Value: []byte("test bytes")},
			"bytes set":  &types.AttributeValueMemberBS{Value: [][]byte{[]byte("test"), []byte("bytes"), []byte("set")}},
			"bool":       &types.AttributeValueMemberBOOL{Value: false},
			"null":       &types.AttributeValueMemberNULL{Value: true},
			"list": &types.AttributeValueMemberL{Value: []types.AttributeValue{
				&types.AttributeValueMemberL{
					Value: []types.AttributeValue{
						&types.AttributeValueMemberS{Value: "string1"},
						&types.AttributeValueMemberS{Value: "string2"},
					},
				},
				&types.AttributeValueMemberN{Value: "1234567890"},
				&types.AttributeValueMemberBS{Value: [][]byte{
					[]byte("bytes1"), []byte("bytes2"),
				}},
			}},
			"map": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"key1": &types.AttributeValueMemberL{
					Value: []types.AttributeValue{&types.AttributeValueMemberBOOL{Value: false}, &types.AttributeValueMemberBOOL{Value: true}},
				},
				"key2": &types.AttributeValueMemberS{Value: "hoge"},
				"key3": &types.AttributeValueMemberS{Value: "fuga"},
				"key4": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"nest1": &types.AttributeValueMemberBS{Value: [][]byte{[]byte("hoge"), []byte("fuga")}},
					"nest2": &types.AttributeValueMemberNS{Value: []string{"1", "2", "3", "4"}},
				}},
			}},
		}
		for name, tt := range test {
			t.Run(name, func(t *testing.T) {
				var b = new(bytes.Buffer)
				var unixNano = time.Now().UnixNano()
				err := e.Encode(tt, unixNano, b)
				require.NoError(t, err)
				got, gotUnixNano, err := d.Decode(b)
				require.NoError(t, err)
				require.Equal(t, tt, got)
				require.Equal(t, gotUnixNano, unixNano)
			})
		}
	})
}
