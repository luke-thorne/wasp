// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package requestargs

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/wasp/packages/downloader"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/stretchr/testify/require"
)

func TestRequestArguments1(t *testing.T) {
	r := New(nil)
	r.AddEncodeSimple("arg1", []byte("data1"))
	r.AddEncodeSimple("arg2", []byte("data2"))
	r.AddEncodeSimple("arg3", []byte("data3"))
	r.AddAsBlobRef("arg4", []byte("data4"))

	require.Len(t, r, 4)
	require.EqualValues(t, r["-arg1"], "data1")
	require.EqualValues(t, r["-arg2"], "data2")
	require.EqualValues(t, r["-arg3"], "data3")
	h := hashing.HashStrings("data4")
	require.EqualValues(t, r["*arg4"], h[:])

	data := r.Bytes()
	back, err := FromMarshalUtil(marshalutil.New(data))
	require.NoError(t, err)

	require.EqualValues(t, r.Bytes(), back.Bytes())

	require.Len(t, back, 4)
	require.EqualValues(t, back["-arg1"], "data1")
	require.EqualValues(t, back["-arg2"], "data2")
	require.EqualValues(t, back["-arg3"], "data3")
	require.EqualValues(t, back["*arg4"], h[:])
}

func TestRequestArguments3(t *testing.T) {
	r := New(nil)
	r.AddEncodeSimple("arg1", []byte("data1"))
	r.AddEncodeSimple("arg2", []byte("data2"))
	r.AddEncodeSimple("arg3", []byte("data3"))

	require.Len(t, r, 3)
	require.EqualValues(t, r["-arg1"], "data1")
	require.EqualValues(t, r["-arg2"], "data2")
	require.EqualValues(t, r["-arg3"], "data3")

	log := testlogger.NewLogger(t)
	reg := registry.NewRegistry(log, mapdb.NewMapDB())

	d, ok, err := r.SolidifyRequestArguments(reg)
	require.NoError(t, err)
	require.True(t, ok)

	dec := kvdecoder.New(d)
	var s1, s2, s3 string
	require.NotPanics(t, func() {
		s1 = dec.MustGetString("arg1")
		s2 = dec.MustGetString("arg2")
		s3 = dec.MustGetString("arg3")
	})
	require.Len(t, d, 3)
	require.EqualValues(t, "data1", s1)
	require.EqualValues(t, "data2", s2)
	require.EqualValues(t, "data3", s3)
}

func TestRequestArguments4(t *testing.T) {
	r := New(nil)
	r.AddEncodeSimple("arg1", []byte("data1"))
	r.AddEncodeSimple("arg2", []byte("data2"))
	r.AddEncodeSimple("arg3", []byte("data3"))
	data := []byte("data4")
	r.AddAsBlobRef("arg4", data)
	h := hashing.HashData(data)

	require.Len(t, r, 4)
	require.EqualValues(t, r["-arg1"], "data1")
	require.EqualValues(t, r["-arg2"], "data2")
	require.EqualValues(t, r["-arg3"], "data3")
	require.EqualValues(t, r["*arg4"], h[:])

	log := testlogger.NewLogger(t)
	reg := registry.NewRegistry(log, mapdb.NewMapDB())

	_, ok, err := r.SolidifyRequestArguments(reg, downloader.New(log, "http://some.fake.address.lt"))
	require.NoError(t, err)
	require.False(t, ok)
}

func TestRequestArguments5(t *testing.T) {
	r := New(nil)
	r.AddEncodeSimple("arg1", []byte("data1"))
	r.AddEncodeSimple("arg2", []byte("data2"))
	r.AddEncodeSimple("arg3", []byte("data3"))
	data := []byte("data4-data4-data4-data4-data4-data4-data4")
	hash := r.AddAsBlobRef("arg4", data)

	require.Len(t, r, 4)
	require.EqualValues(t, r["-arg1"], "data1")
	require.EqualValues(t, r["-arg2"], "data2")
	require.EqualValues(t, r["-arg3"], "data3")
	require.EqualValues(t, r["*arg4"], hash[:])

	log := testlogger.NewLogger(t)
	reg := registry.NewRegistry(log, mapdb.NewMapDB())

	// cannot solidify yet
	back, ok, err := r.SolidifyRequestArguments(reg)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, back)

	// add missing data to blob cache
	hback, err := reg.PutBlob(data)
	require.NoError(t, err)
	require.EqualValues(t, hash, hback)

	// now we can solidify
	back, ok, err = r.SolidifyRequestArguments(reg)
	require.NoError(t, err)
	require.True(t, ok)

	require.Len(t, back, 4)
	require.EqualValues(t, back["arg1"], "data1")
	require.EqualValues(t, back["arg2"], "data2")
	require.EqualValues(t, back["arg3"], "data3")
	require.EqualValues(t, back["arg4"], data)
}

const N = 50

func TestRequestArgumentsDeterminism(t *testing.T) {
	data := []byte("data4-data4-data4-data4-data4-data4-data4")
	perm := util.NewPermutation16(N, data).GetArray()

	darr1 := make([]string, N)
	darr2 := make([]string, N)
	for i := range darr1 {
		darr1[i] = fmt.Sprintf("arg%d", i)
	}
	for i := range darr2 {
		darr2[i] = darr1[perm[i]]
	}

	// add some args
	r1 := New(nil)
	for i, s := range darr1 {
		r1.AddEncodeSimple(kv.Key(s), []byte(darr2[i]))
	}
	r1.AddAsBlobRef("---", data)

	// add same args in different order
	r2 := New(nil)
	r2.AddAsBlobRef("---", data)
	for i := range darr1 {
		r2.AddEncodeSimple(kv.Key(darr1[perm[i]]), []byte(darr2[perm[i]]))
	}

	// hash should be deterministic; independent of order
	h1 := util.GetHashValue(r1)
	h2 := util.GetHashValue(r2)
	require.EqualValues(t, h1, h2)
}
