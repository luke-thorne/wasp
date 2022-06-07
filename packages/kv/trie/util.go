package trie

import (
	"io"
)

type byteCounter int

func (b *byteCounter) Write(p []byte) (n int, err error) {
	*b = byteCounter(int(*b) + len(p))
	return 0, nil
}

func Size(o interface{ Write(w io.Writer) error }) (int, error) {
	var ret byteCounter
	if err := o.Write(&ret); err != nil {
		return 0, err
	}
	return int(ret), nil
}

func MustSize(o interface{ Write(w io.Writer) error }) int {
	ret, err := Size(o)
	if err != nil {
		panic(err)
	}
	return ret
}

func commonPrefix(b1, b2 []byte) []byte {
	ret := make([]byte, 0)
	for i := 0; i < len(b1) && i < len(b2); i++ {
		if b1[i] != b2[i] {
			break
		}
		ret = append(ret, b1[i])
	}
	return ret
}

func assert(cond bool, err interface{}) {
	if !cond {
		panic(err)
	}
}
