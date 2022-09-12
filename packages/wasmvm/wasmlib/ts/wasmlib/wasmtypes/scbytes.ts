// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import * as wasmtypes from "./index";

export function bytesCompare(lhs: u8[], rhs: u8[]): i32 {
    const size = (lhs.length < rhs.length) ? lhs.length : rhs.length;
    for (let i = 0; i < size; i++) {
        if (lhs[i] != rhs[i]) {
            return (lhs[i] > rhs[i]) ? 1 : -1
        }
    }
    return (lhs.length > size) ? 1 : (rhs.length > size) ? -1 : 0;
}

// \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\

export function bytesDecode(dec: wasmtypes.WasmDecoder): u8[] {
    return dec.bytes();
}

export function bytesEncode(enc: wasmtypes.WasmEncoder, value: u8[]): void {
    enc.bytes(value);
}

export function bytesFromBytes(buf: u8[]): u8[] {
    return buf;
}

export function bytesToBytes(buf: u8[]): u8[] {
    return buf;
}

export function bytesFromString(value: string): u8[] {
    return wasmtypes.hexDecode(value);
}

export function bytesToString(value: u8[]): string {
    return wasmtypes.hexEncode(value);
}

// \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\

export class ScImmutableBytes {
    proxy: wasmtypes.Proxy;

    constructor(proxy: wasmtypes.Proxy) {
        this.proxy = proxy;
    }

    exists(): bool {
        return this.proxy.exists();
    }

    toString(): string {
        return bytesToString(this.value());
    }

    value(): u8[] {
        return bytesFromBytes(this.proxy.get());
    }
}

// \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\ // \\

export class ScMutableBytes extends ScImmutableBytes {
    delete(): void {
        this.proxy.delete();
    }

    setValue(value: u8[]): void {
        this.proxy.set(bytesToBytes(value));
    }
}
