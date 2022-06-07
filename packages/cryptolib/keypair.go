package cryptolib

import (
	iotago "github.com/iotaledger/iota.go/v3"
)

type KeyPair struct {
	privateKey *PrivateKey
	publicKey  *PublicKey
}

// NewKeyPair creates a new key pair with a randomly generated seed
func NewKeyPair() *KeyPair {
	privateKey := NewPrivateKey()
	return NewKeyPairFromPrivateKey(privateKey)
}

func NewKeyPairFromSeed(seed Seed) *KeyPair {
	privateKey := NewPrivateKeyFromSeed(seed)
	return NewKeyPairFromPrivateKey(privateKey)
}

func NewKeyPairFromPrivateKey(privateKey *PrivateKey) *KeyPair {
	publicKey := privateKey.Public()
	return &KeyPair{
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

func (k *KeyPair) IsValid() bool {
	return k.privateKey.isValid()
}

func (k *KeyPair) Verify(message, sig []byte) bool {
	return k.publicKey.Verify(message, sig)
}

func (k *KeyPair) AsAddressSigner() iotago.AddressSigner {
	addrKeys := k.privateKey.AddressKeysForEd25519Address(k.publicKey.AsEd25519Address())
	return iotago.NewInMemoryAddressSigner(addrKeys)
}

func (k *KeyPair) GetPrivateKey() *PrivateKey {
	return k.privateKey
}

func (k *KeyPair) GetPublicKey() *PublicKey {
	return k.publicKey
}

func (k *KeyPair) Address() *iotago.Ed25519Address {
	return k.GetPublicKey().AsEd25519Address()
}
