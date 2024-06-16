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
	pk         *types.AttributeValueMemberS
	pkSHA256   string
	Pk4bit     uint32
	sk         *types.AttributeValueMemberS
	pkskSHA256 string
	UnixNano   int64
	Item       map[string]types.AttributeValue
}

func newItem(avm map[string]types.AttributeValue, c Config) (*item, error) {
	var i = &item{
		Item:     avm,
		UnixNano: time.Now().UnixNano(),
	}
	var pkAv, skAv types.AttributeValue
	for key, v := range i.Item {
		if key == c.Table.PartitionKey {
			pkAv = v
		}
		if key == c.Table.SortKey {
			skAv = v
		}
	}
	if pkAv == nil {
		return nil, ErrNotFoundPartitionKey
	}
	var ok bool
	if i.pk, ok = (pkAv).(*types.AttributeValueMemberS); !ok {
		return nil, ErrInvalidPartitionKeyType
	}
	if i.pk.Value == "" {
		return nil, ErrEmptyPartitionKey
	}
	pkSHA256, strPkSHA256 := sum256([]byte(i.pk.Value))
	i.pkSHA256 = strPkSHA256
	i.Pk4bit = binary.BigEndian.Uint32(pkSHA256[:4])

	if c.Table.SortKey != "" {
		if skAv == nil {
			return nil, ErrNotFoundSortKey
		}
		if i.sk, ok = (skAv).(*types.AttributeValueMemberS); !ok {
			return nil, ErrInvalidSortKeyType
		}
		if i.sk.Value == "" {
			return nil, ErrEmptySortKey
		}
		_, i.pkskSHA256 = joinStrSum256(i.pk.Value, i.sk.Value)
	}
	return i, nil
}

func (i *item) PrimaryKey() string {
	if i.pkskSHA256 != "" {
		return i.pkskSHA256
	}
	return i.pkSHA256
}

func (i *item) Value() ([]byte, error) {
	var buf = new(bytes.Buffer)
	var e = newEncoder(prefixInt64EncOption(i.UnixNano))
	err := e.Encode(&types.AttributeValueMemberM{Value: i.Item}, buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (i *item) Unmarshal(data []byte) error {
	var r = bytes.NewReader(data)
	var d = newDecoder(prefixInt64DecOption(&i.UnixNano))
	av, err := d.Decode(r)
	if err != nil {
		return err
	}
	avm, ok := av.(*types.AttributeValueMemberM)
	if !ok {
		return ErrCannotUnmarshal
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

type encoder struct {
	options []encodeOption
}

func newEncoder(opts ...encodeOption) encoder {
	return encoder{options: opts}
}

type encodeOption func(io.Writer) error

func prefixInt64EncOption(tm int64) encodeOption {
	return func(w io.Writer) error {
		return binary.Write(w, enc, uint64(tm))
	}
}

func prefixByteEncOption(bt byte) encodeOption {
	return func(w io.Writer) error {
		_, err := w.Write([]byte{bt})
		return err
	}
}

func (e *encoder) Encode(av types.AttributeValue, w io.Writer) error {
	for _, opt := range e.options {
		if err := opt(w); err != nil {
			return err
		}
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

type decoder struct {
	options []decodeOption
}

func newDecoder(opts ...decodeOption) decoder {
	return decoder{options: opts}
}

type decodeOption func(r io.Reader) error

func prefixInt64DecOption(n *int64) decodeOption {
	return func(r io.Reader) error {
		if n == nil {
			panic("n is nil")
		}
		var b8 = make([]byte, 8)
		if _, err := r.Read(b8); err != nil {
			return err
		}
		*n = int64(enc.Uint64(b8))
		return nil
	}
}

func prefixByteDecOption(bt *byte) decodeOption {
	return func(r io.Reader) error {
		if bt == nil {
			panic("bt is nil")
		}
		var b1 = make([]byte, 1)
		if _, err := r.Read(b1); err != nil {
			return err
		}
		*bt = b1[0]
		return nil
	}
}

func (d *decoder) Decode(r io.Reader) (types.AttributeValue, error) {
	for _, opt := range d.options {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return d.decode(r)
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

func joinStrSum256(s0, s1 string) (sha256Key []byte, strSha256Key string) {
	var b0, b1 = []byte(s0), []byte(s1)
	var b = make([]byte, len(b0)+len(b1))
	_ = copy(b, b0)
	_ = copy(b[len(b0):], b1)
	return sum256(b)
}

func sum256(data []byte) (sha256Key []byte, strSha256Key string) {
	h := sha256.Sum256(data)
	sha256Key = h[:]
	strSha256Key = toStrSha256(sha256Key)
	return
}
