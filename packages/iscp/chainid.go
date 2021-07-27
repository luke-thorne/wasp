// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package iscp

import (
	"io"

	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/hashing"
	"golang.org/x/xerrors"
)

// ChainID represents the global identifier of the chain
// It is wrapped AliasAddress, an address without a private key behind
type ChainID struct {
	*ledgerstate.AliasAddress
}

// NewChainID creates new chain ID from alias address
func NewChainID(addr *ledgerstate.AliasAddress) *ChainID {
	return &ChainID{addr}
}

// ChainIDFromAddress creates a chainIDD from alias address. Returns and error if not an alias address type
func ChainIDFromAddress(addr ledgerstate.Address) (*ChainID, error) {
	alias, ok := addr.(*ledgerstate.AliasAddress)
	if !ok {
		return nil, xerrors.New("chain id must be an alias address")
	}
	return &ChainID{alias}, nil
}

// ChainIDFromBase58 constructor decodes base58 string to the ChainID
func ChainIDFromBase58(b58 string) (*ChainID, error) {
	alias, err := ledgerstate.AliasAddressFromBase58EncodedString(b58)
	if err != nil {
		return nil, err
	}
	return &ChainID{alias}, nil
}

func ChainIDFromMarshalUtil(mu *marshalutil.MarshalUtil) (*ChainID, error) {
	aliasAddr, err := ledgerstate.AliasAddressFromMarshalUtil(mu)
	if err != nil {
		return nil, err
	}
	return &ChainID{aliasAddr}, nil
}

// ChainIDFromBytes reconstructs a ChainID from its binary representation.
func ChainIDFromBytes(data []byte) (*ChainID, error) {
	alias, _, err := ledgerstate.AliasAddressFromBytes(data)
	if err != nil {
		return nil, err
	}
	return &ChainID{alias}, nil
}

// RandomChainID creates a random chain ID.
func RandomChainID(seed ...[]byte) *ChainID {
	var h hashing.HashValue
	if len(seed) > 0 {
		h = hashing.HashData(seed[0])
	} else {
		h = hashing.RandomHash(nil)
	}
	return &ChainID{ledgerstate.NewAliasAddress(h[:])}
}

func (chid *ChainID) Equals(chid1 *ChainID) bool {
	return chid.AliasAddress.Equals(chid1.AliasAddress)
}

func (chid *ChainID) Clone() (ret *ChainID) {
	return &ChainID{chid.AliasAddress.Clone().(*ledgerstate.AliasAddress)}
}

func (chid *ChainID) Base58() string {
	return chid.AliasAddress.Base58()
}

// String human readable form (base58 encoding)
func (chid *ChainID) String() string {
	return "$/" + chid.Base58()
}

func (chid *ChainID) AsAddress() ledgerstate.Address {
	return chid.AliasAddress
}

func (chid *ChainID) AsAliasAddress() *ledgerstate.AliasAddress {
	return chid.AliasAddress
}

func (chid *ChainID) Read(r io.Reader) error {
	var buf [ledgerstate.AddressLength]byte
	if n, err := r.Read(buf[:]); err != nil || n != ledgerstate.AddressLength {
		return xerrors.Errorf("error while parsing address (err=%v)", err)
	}
	alias, _, err := ledgerstate.AliasAddressFromBytes(buf[:])
	if err != nil {
		return err
	}
	chid.AliasAddress = alias
	return nil
}

func (chid *ChainID) Write(w io.Writer) error {
	_, err := w.Write(chid.AliasAddress.Bytes())
	return err
}
