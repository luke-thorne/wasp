package trie

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPrefix(t *testing.T) {
	t.Run("nil and nil", func(t *testing.T) {
		require.EqualValues(t, 0, prefixLen(nil, nil))
	})
	t.Run("nil and not nil", func(t *testing.T) {
		require.EqualValues(t, 0, prefixLen(nil, []byte("abc")))
	})
	t.Run("not nil and nil", func(t *testing.T) {
		require.EqualValues(t, 0, prefixLen([]byte("abc"), nil))
	})
	t.Run("equal", func(t *testing.T) {
		require.EqualValues(t, 3, prefixLen([]byte("abc"), []byte("abc")))
	})
	t.Run("one longer", func(t *testing.T) {
		require.EqualValues(t, 3, prefixLen([]byte("abc"), []byte("abcde")))
	})
	t.Run("common prefix", func(t *testing.T) {
		require.EqualValues(t, 3, prefixLen([]byte("abcde"), []byte("abcfgh")))
	})
	t.Run("different", func(t *testing.T) {
		require.EqualValues(t, 0, prefixLen([]byte("abcd"), []byte("efgh")))
	})
}
