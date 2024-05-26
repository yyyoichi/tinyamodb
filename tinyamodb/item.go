package tinyamodb

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	toStrSha256 = hex.EncodeToString
)

type Item interface {
	SHA256Key() []byte
	StrSHA2526Key() string
	Value() ([]byte, error)
	Unmarshal([]byte) error
}

var (
	ErrNotFoundPartitionKey    = errors.New("not found partition key")
	ErrInvalidPartitionKeyType = errors.New("partition key must be 'string' type")
	ErrEmptyPartitionKey       = errors.New("partition key is empty")
	ErrNotFoundSortKey         = errors.New("not found sort key")
	ErrInvalidSortKeyType      = errors.New("partition key must be 'string' or 'number' type")
	ErrEmptySortKey            = errors.New("sort key is empty")
	ErrCannotUnmarshal         = errors.New("cannot unmarshal")
)

type item struct {
	pk       types.AttributeValue
	pkSHA256 string
	Pk4bit   uint32
	sk       types.AttributeValue
	skSHA256 string
	UnixNano int64
	Item     map[string]types.AttributeValue
}

func newItem(avm map[string]types.AttributeValue, c Config) (*item, error) {
	var i = &item{
		Item:     avm,
		UnixNano: time.Now().UnixNano(),
	}
	for key, v := range i.Item {
		if key == c.Table.PartitionKey {
			i.pk = v
		}
		if key == c.Table.SortKey {
			i.sk = v
		}
	}
	if i.pk == nil {
		return nil, ErrNotFoundPartitionKey
	}
	var pk []byte
	if av, ok := (i.pk).(*types.AttributeValueMemberS); !ok {
		return nil, ErrInvalidPartitionKeyType
	} else {
		pk = []byte(av.Value)
	}
	if len(pk) == 0 {
		return nil, ErrEmptyPartitionKey
	}
	var partitionKey []byte
	partitionKey, i.pkSHA256 = sum256(pk)
	i.Pk4bit = binary.BigEndian.Uint32(partitionKey[:4])

	if c.Table.SortKey != "" {
		if i.sk == nil {
			return nil, ErrNotFoundSortKey
		}
		var sk []byte
		switch av := (i.sk).(type) {
		case *types.AttributeValueMemberN:
			sk = []byte(av.Value)

		case *types.AttributeValueMemberS:
			sk = []byte(av.Value)

		default:
			return nil, ErrInvalidSortKeyType
		}
		if len(sk) == 0 {
			return nil, ErrEmptySortKey
		}
		_, i.skSHA256 = sum256(sk)
	}
	return i, nil
}

func (i *item) PrimaryKey() string {
	if i.skSHA256 != "" {
		return i.skSHA256
	}
	return i.pkSHA256
}

func (i *item) Value() ([]byte, error) {
	var buf = new(bytes.Buffer)
	var e encoder
	err := e.Encode(&types.AttributeValueMemberM{Value: i.Item}, i.UnixNano, buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (i *item) Unmarshal(data []byte) error {
	var r = bytes.NewReader(data)
	var d decoder
	av, unixNano, err := d.Decode(r)
	if err != nil {
		return err
	}
	i.UnixNano = unixNano
	avm, ok := av.(*types.AttributeValueMemberM)
	if !ok {
		return err
	}
	i.Item = avm.Value
	return nil
}

type tinyamodbItem struct {
	sha256Key    []byte
	strSha256Key string
	sortKey      types.AttributeValue
	Item         map[string]types.AttributeValue
	UnixNano     int64
}

func NewTinyamoDbItem(item map[string]types.AttributeValue, c Config) (*tinyamodbItem, error) {
	var av types.AttributeValue
	var sav types.AttributeValue
	for key, v := range item {
		if key == c.Table.PartitionKey {
			av = v
		}
		if key == c.Table.SortKey {
			sav = v
		}
	}
	if av == nil {
		return nil, ErrNotFoundPartitionKey
	}
	avs, ok := (av).(*types.AttributeValueMemberS)
	if !ok {
		return nil, ErrInvalidPartitionKeyType
	}
	if c.Table.SortKey != "" {
		if sav == nil {
			return nil, ErrNotFoundSortKey
		}
		switch (sav).(type) {
		case *types.AttributeValueMemberS, *types.AttributeValueMemberN:
		default:
			return nil, ErrInvalidSortKeyType
		}
	}
	key, strKey := sum256([]byte(avs.Value))
	return &tinyamodbItem{
		sha256Key:    key,
		strSha256Key: strKey,
		sortKey:      sav,
		Item:         item,
		UnixNano:     time.Now().UnixNano(),
	}, nil
}

func (i *tinyamodbItem) SHA256Key() []byte {
	return i.sha256Key
}
func (i *tinyamodbItem) StrSHA2526Key() string {
	return i.strSha256Key
}
func (i *tinyamodbItem) Value() ([]byte, error) {
	var buf = new(bytes.Buffer)
	var e encoder
	err := e.Encode(&types.AttributeValueMemberM{Value: i.Item}, i.UnixNano, buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func (i *tinyamodbItem) Unmarshal(data []byte) error {
	var r = bytes.NewReader(data)
	var d decoder
	av, unixNano, err := d.Decode(r)
	if err != nil {
		return err
	}
	i.UnixNano = unixNano
	avm, ok := av.(*types.AttributeValueMemberM)
	if !ok {
		return err
	}
	i.Item = avm.Value
	return nil
}

const (
	_bs = byte('s') // string
	_bS = byte('S') // []string
	_bn = byte('n') // number
	_bN = byte('N') // []number
	_bb = byte('b') // []byte
	_bB = byte('B') // [][]byte
	_bo = byte('o') // bool
	_bu = byte('u') // NULL
	_bl = byte('l') // list
	_bm = byte('m') // map
)

type encoder struct{}

func (e *encoder) Encode(av types.AttributeValue, unixNano int64, w io.Writer) error {
	if err := binary.Write(w, enc, uint64(unixNano)); err != nil {
		return err
	}
	return e.encode(av, w)
}
func (e *encoder) encode(av types.AttributeValue, w io.Writer) error {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		if _, err := w.Write([]byte{_bs}); err != nil {
			return err
		}
		return e.encodeString(v.Value, w)

	case *types.AttributeValueMemberSS:
		if _, err := w.Write([]byte{_bS}); err != nil {
			return err
		}
		return e.encodeSSet(v.Value, w)

	case *types.AttributeValueMemberN:
		if _, err := w.Write([]byte{_bn}); err != nil {
			return err
		}
		return e.encodeString(v.Value, w)

	case *types.AttributeValueMemberNS:
		if _, err := w.Write([]byte{_bN}); err != nil {
			return err
		}
		return e.encodeSSet(v.Value, w)

	case *types.AttributeValueMemberB:
		if _, err := w.Write([]byte{_bb}); err != nil {
			return err
		}
		return e.encodeBytes(v.Value, w)

	case *types.AttributeValueMemberBS:
		if _, err := w.Write([]byte{_bB}); err != nil {
			return err
		}
		return e.encodeBSet(v.Value, w)

	case *types.AttributeValueMemberBOOL:
		if _, err := w.Write([]byte{_bo}); err != nil {
			return err
		}
		return e.encodeBool(v.Value, w)

	case *types.AttributeValueMemberNULL:
		if _, err := w.Write([]byte{_bu}); err != nil {
			return err
		}
		return e.encodeBool(v.Value, w)

	case *types.AttributeValueMemberL:
		if _, err := w.Write([]byte{_bl}); err != nil {
			return err
		}
		return e.encodeList(v.Value, w)

	case *types.AttributeValueMemberM:
		if _, err := w.Write([]byte{_bm}); err != nil {
			return err
		}
		return e.encodeMap(v.Value, w)

	}
	return errors.New("unkown type")
}

func (e *encoder) encodeString(v string, w io.Writer) error {
	bv := []byte(v)
	if _, err := w.Write([]byte{byte(len(bv))}); err != nil {
		return err
	}
	_, err := w.Write(bv)
	return err
}

func (e *encoder) encodeSSet(v []string, w io.Writer) error {
	if _, err := w.Write([]byte{byte(len(v))}); err != nil {
		return err
	}
	for _, s := range v {
		if err := e.encodeString(s, w); err != nil {
			return err
		}
	}
	return nil
}

func (e *encoder) encodeBytes(v []byte, w io.Writer) error {
	if _, err := w.Write([]byte{byte(len(v))}); err != nil {
		return err
	}
	_, err := w.Write(v)
	return err
}

func (e *encoder) encodeBSet(v [][]byte, w io.Writer) error {
	if _, err := w.Write([]byte{byte(len(v))}); err != nil {
		return err
	}
	for _, b := range v {
		if err := e.encodeBytes(b, w); err != nil {
			return err
		}
	}
	return nil
}

func (e *encoder) encodeBool(v bool, w io.Writer) error {
	var b byte
	if v {
		b = '1'
	} else {
		b = '0'
	}
	_, err := w.Write([]byte{b})
	return err
}

func (e *encoder) encodeList(v []types.AttributeValue, w io.Writer) error {
	if _, err := w.Write([]byte{byte(len(v))}); err != nil {
		return err
	}
	for _, av := range v {
		if err := e.encode(av, w); err != nil {
			return err
		}
	}
	return nil
}

func (e *encoder) encodeMap(v map[string]types.AttributeValue, w io.Writer) error {
	if _, err := w.Write([]byte{byte(len(v))}); err != nil {
		return err
	}
	for k, av := range v {
		if err := e.encodeString(k, w); err != nil {
			return err
		}
		if err := e.encode(av, w); err != nil {
			return err
		}
	}
	return nil
}

type decoder struct{}

func (d *decoder) Decode(r io.Reader) (types.AttributeValue, int64, error) {
	var unixNanoB = make([]byte, 8)
	if _, err := r.Read(unixNanoB); err != nil {
		return nil, 0, err
	}
	av, err := d.decode(r)
	return av, int64(enc.Uint64(unixNanoB)), err
}

func (d *decoder) decode(r io.Reader) (types.AttributeValue, error) {
	var _bx = make([]byte, 1)
	if _, err := r.Read(_bx); err != nil {
		return nil, err
	}
	switch _bx[0] {
	case _bs:
		v, err := d.decodeString(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberS{Value: v}, nil

	case _bS:
		v, err := d.decodeSSet(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberSS{Value: v}, nil

	case _bn:
		v, err := d.decodeString(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberN{Value: v}, nil

	case _bN:
		v, err := d.decodeSSet(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberNS{Value: v}, nil

	case _bb:
		v, err := d.decodeBytes(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberB{Value: v}, nil

	case _bB:
		v, err := d.decodeBSet(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberBS{Value: v}, nil

	case _bo:
		v, err := d.decodeBool(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberBOOL{Value: v}, nil

	case _bu:
		v, err := d.decodeBool(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberNULL{Value: v}, nil

	case _bl:
		v, err := d.decodeList(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberL{Value: v}, nil

	case _bm:
		v, err := d.decodeMap(r)
		if err != nil {
			return nil, err
		}
		return &types.AttributeValueMemberM{Value: v}, nil

	}
	return nil, fmt.Errorf("unexpected identifier: '%v'", string(_bm))
}

func (d *decoder) decodeString(r io.Reader) (string, error) {
	v, err := d.decodeBytes(r)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

func (d *decoder) decodeSSet(r io.Reader) ([]string, error) {
	l, err := d.decodeLen(r)
	if err != nil {
		return nil, err
	}
	var v = make([]string, l)
	for i := range l {
		s, err := d.decodeString(r)
		if err != nil {
			return nil, err
		}
		v[i] = s
	}
	return v, nil
}

func (d *decoder) decodeBytes(r io.Reader) ([]byte, error) {
	l, err := d.decodeLen(r)
	if err != nil {
		return nil, err
	}
	var v = make([]byte, l)
	if _, err := r.Read(v); err != nil {
		return nil, err
	}
	return v, nil
}

func (d *decoder) decodeBSet(r io.Reader) ([][]byte, error) {
	l, err := d.decodeLen(r)
	if err != nil {
		return nil, err
	}
	var v = make([][]byte, l)
	for i := range l {
		s, err := d.decodeBytes(r)
		if err != nil {
			return nil, err
		}
		v[i] = s
	}
	return v, nil
}

func (d *decoder) decodeBool(r io.Reader) (bool, error) {
	bv := make([]byte, 1)
	if _, err := r.Read(bv); err != nil {
		return false, err
	}
	return bv[0] == '1', nil
}

func (d *decoder) decodeList(r io.Reader) ([]types.AttributeValue, error) {
	l, err := d.decodeLen(r)
	if err != nil {
		return nil, err
	}
	var v = make([]types.AttributeValue, l)
	for i := range l {
		av, err := d.decode(r)
		if err != nil {
			return nil, err
		}
		v[i] = av
	}
	return v, nil
}

func (d *decoder) decodeMap(r io.Reader) (map[string]types.AttributeValue, error) {
	l, err := d.decodeLen(r)
	if err != nil {
		return nil, err
	}
	var v = make(map[string]types.AttributeValue, l)
	for range l {
		s, err := d.decodeString(r)
		if err != nil {
			return nil, err
		}
		av, err := d.decode(r)
		if err != nil {
			return nil, err
		}
		v[s] = av
	}
	return v, nil
}

func (d *decoder) decodeLen(r io.Reader) (int, error) {
	bl := make([]byte, 1)
	if _, err := r.Read(bl); err != nil {
		return 0, err
	}
	return int(bl[0]), nil
}

func sum256(data []byte) (sha256Key []byte, strSha256Key string) {
	h := sha256.Sum256(data)
	sha256Key = h[:]
	strSha256Key = toStrSha256(sha256Key)
	return
}
