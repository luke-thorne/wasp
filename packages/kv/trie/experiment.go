package trie

type Nibbles []byte

const (
	OddEvenBitMask       = byte(0x01)
	LeafExtensionBitMask = byte(0x02)
)
