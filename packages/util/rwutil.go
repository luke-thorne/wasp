package util

import (
	"encoding"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/pkg/errors"
)

//////////////////// byte \\\\\\\\\\\\\\\\\\\\

func ReadByte(r io.Reader) (byte, error) {
	var b [1]byte
	_, err := r.Read(b[:])
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func WriteByte(w io.Writer, val byte) error {
	b := []byte{val}
	_, err := w.Write(b)
	return err
}

//////////////////// uint16 \\\\\\\\\\\\\\\\\\\\

func Uint16To2Bytes(val uint16) []byte {
	var tmp2 [2]byte
	binary.LittleEndian.PutUint16(tmp2[:], val)
	return tmp2[:]
}

func Uint16From2Bytes(b []byte) (uint16, error) {
	if len(b) != 2 { //nolint:gomnd
		return 0, errors.New("len(b) != 2")
	}
	return binary.LittleEndian.Uint16(b), nil
}

func MustUint16From2Bytes(b []byte) uint16 {
	ret, err := Uint16From2Bytes(b)
	if err != nil {
		panic(err)
	}
	return ret
}

func ReadUint16(r io.Reader, pval *uint16) error {
	var tmp2 [2]byte
	_, err := r.Read(tmp2[:])
	if err != nil {
		return err
	}
	*pval = binary.LittleEndian.Uint16(tmp2[:])
	return nil
}

func WriteUint16(w io.Writer, val uint16) error {
	_, err := w.Write(Uint16To2Bytes(val))
	return err
}

//////////////////// int32 \\\\\\\\\\\\\\\\\\\\

func Int32To4Bytes(val int32) []byte {
	return Uint32To4Bytes(uint32(val))
}

func ReadInt32(r io.Reader, pval *int32) error {
	var tmp4 [4]byte
	_, err := r.Read(tmp4[:])
	if err != nil {
		return err
	}
	*pval = int32(binary.LittleEndian.Uint32(tmp4[:]))
	return nil
}

//////////////////// uint32 \\\\\\\\\\\\\\\\\\\\

func Uint32To4Bytes(val uint32) []byte {
	var tmp4 [4]byte
	binary.LittleEndian.PutUint32(tmp4[:], val)
	return tmp4[:]
}

func Uint32From4Bytes(b []byte) (uint32, error) {
	if len(b) != 4 { //nolint:gomnd
		return 0, errors.New("len(b) != 4")
	}
	return binary.LittleEndian.Uint32(b), nil
}

func MustUint32From4Bytes(b []byte) uint32 {
	ret, err := Uint32From4Bytes(b)
	if err != nil {
		panic(err)
	}
	return ret
}

func ReadUint32(r io.Reader, pval *uint32) error {
	var tmp4 [4]byte
	_, err := r.Read(tmp4[:])
	if err != nil {
		return err
	}
	*pval = MustUint32From4Bytes(tmp4[:])
	return nil
}

func WriteUint32(w io.Writer, val uint32) error {
	_, err := w.Write(Uint32To4Bytes(val))
	return err
}

//////////////////// int64 \\\\\\\\\\\\\\\\\\\\

func Int64To8Bytes(val int64) []byte {
	return Uint64To8Bytes(uint64(val))
}

func Int64From8Bytes(b []byte) (int64, error) {
	ret, err := Uint64From8Bytes(b)
	return int64(ret), err
}

func ReadInt64(r io.Reader, pval *int64) error {
	var tmp8 [8]byte
	_, err := r.Read(tmp8[:])
	if err != nil {
		return err
	}
	*pval = int64(binary.LittleEndian.Uint64(tmp8[:]))
	return nil
}

func WriteInt64(w io.Writer, val int64) error {
	_, err := w.Write(Uint64To8Bytes(uint64(val)))
	return err
}

//////////////////// uint64 \\\\\\\\\\\\\\\\\\\\

func Uint64To8Bytes(val uint64) []byte {
	var tmp8 [8]byte
	binary.LittleEndian.PutUint64(tmp8[:], val)
	return tmp8[:]
}

func Uint64From8Bytes(b []byte) (uint64, error) {
	if len(b) != 8 { //nolint:gomnd
		return 0, errors.New("len(b) != 8")
	}
	return binary.LittleEndian.Uint64(b), nil
}

func MustUint64From8Bytes(b []byte) uint64 {
	ret, err := Uint64From8Bytes(b)
	if err != nil {
		panic(err)
	}
	return ret
}

func ReadUint64(r io.Reader, pval *uint64) error {
	var tmp8 [8]byte
	_, err := r.Read(tmp8[:])
	if err != nil {
		return err
	}
	*pval = binary.LittleEndian.Uint64(tmp8[:])
	return nil
}

func WriteUint64(w io.Writer, val uint64) error {
	_, err := w.Write(Uint64To8Bytes(val))
	return err
}

//////////////////// bytes, uint16 length \\\\\\\\\\\\\\\\\\\\

const MaxUint16 = int(^uint16(0))

func ReadBytes16(r io.Reader) ([]byte, error) {
	var length uint16
	err := ReadUint16(r, &length)
	if err != nil {
		return nil, err
	}
	if length == 0 {
		return []byte{}, nil
	}
	ret := make([]byte, length)
	_, err = r.Read(ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func WriteBytes16(w io.Writer, data []byte) error {
	if len(data) > MaxUint16 {
		panic("WriteBytes16: too long data")
	}
	err := WriteUint16(w, uint16(len(data)))
	if err != nil {
		return err
	}
	if len(data) != 0 {
		_, err = w.Write(data)
	}
	return err
}

//////////////////// bytes, uint32 length \\\\\\\\\\\\\\\\\\\\

const MaxUint32 = int(^uint32(0))

func ReadBytes32(r io.Reader) ([]byte, error) {
	var length uint32
	err := ReadUint32(r, &length)
	if err != nil {
		return nil, err
	}
	if length == 0 {
		return []byte{}, nil
	}
	ret := make([]byte, length)
	_, err = r.Read(ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func WriteBytes32(w io.Writer, data []byte) error {
	if len(data) > MaxUint32 {
		panic("WriteBytes32: too long data")
	}
	err := WriteUint32(w, uint32(len(data)))
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

//////////////////// bool \\\\\\\\\\\\\\\\\\\\

func ReadBoolByte(r io.Reader, cond *bool) error {
	var b [1]byte
	_, err := r.Read(b[:])
	if err != nil {
		return err
	}
	*cond = b[0] == 0xFF
	if !*cond && b[0] != 0x00 {
		return errors.New("ReadBoolByte: unexpected value")
	}
	return nil
}

func WriteBoolByte(w io.Writer, cond bool) error {
	var b [1]byte
	if cond {
		b[0] = 0xFF
	}
	_, err := w.Write(b[:])
	return err
}

//////////////////// time \\\\\\\\\\\\\\\\\\\\

func ReadTime(r io.Reader, ts *time.Time) error {
	var nano uint64
	err := ReadUint64(r, &nano)
	if err != nil {
		return err
	}
	*ts = time.Unix(0, int64(nano))
	return nil
}

func WriteTime(w io.Writer, ts time.Time) error {
	return WriteUint64(w, uint64(ts.UnixNano()))
}

//////////////////// string, uint16 length \\\\\\\\\\\\\\\\\\\\

func ReadString16(r io.Reader) (string, error) {
	ret, err := ReadBytes16(r)
	if err != nil {
		return "", err
	}
	return string(ret), err
}

func WriteString16(w io.Writer, str string) error {
	return WriteBytes16(w, []byte(str))
}

//////////////////// string array, uint16 length \\\\\\\\\\\\\\\\\\\\

func ReadStrings16(r io.Reader) ([]string, error) {
	var size uint16
	if err := ReadUint16(r, &size); err != nil {
		return nil, nil
	}
	ret := make([]string, size)
	var err error
	for i := range ret {
		if ret[i], err = ReadString16(r); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func WriteStrings16(w io.Writer, strs []string) error {
	if len(strs) > MaxUint16 {
		panic("WriteStrings16: too long array")
	}
	if err := WriteUint16(w, uint16(len(strs))); err != nil {
		return err
	}
	for _, s := range strs {
		if err := WriteString16(w, s); err != nil {
			return err
		}
	}
	return nil
}

//////////////////// hash \\\\\\\\\\\\\\\\\\\\

func ReadHashValue(r io.Reader, h *hashing.HashValue) error {
	n, err := r.Read(h[:])
	if err != nil {
		return err
	}
	if n != hashing.HashSize {
		return errors.New("error while reading hash")
	}
	return nil
}

//////////////////// marshaled \\\\\\\\\\\\\\\\\\\\

// ReadMarshaled supports kyber.Point, kyber.Scalar and similar.
func ReadMarshaled(r io.Reader, val encoding.BinaryUnmarshaler) error {
	var err error
	var bin []byte
	if bin, err = ReadBytes16(r); err != nil {
		return err
	}
	return val.UnmarshalBinary(bin)
}

// WriteMarshaled supports kyber.Point, kyber.Scalar and similar.
func WriteMarshaled(w io.Writer, val encoding.BinaryMarshaler) error {
	var err error
	var bin []byte
	if bin, err = val.MarshalBinary(); err != nil {
		return err
	}
	return WriteBytes16(w, bin)
}

func ReadOutputID(r io.Reader, oid *ledgerstate.OutputID) error {
	n, err := r.Read(oid[:])
	if err != nil {
		return err
	}
	if n != ledgerstate.OutputIDLength {
		return fmt.Errorf("error while reading output ID: read %v bytes, expected %v bytes",
			n, ledgerstate.OutputIDLength)
	}
	return nil
}
