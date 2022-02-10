# Adding RPC Debug endpoints

In order for BlockScout to index the EVM properly, the [`debug` endpoints](https://geth.ethereum.org/docs/rpc/ns-debug) need to be implemented. 

For now, I think only 1 endpoint is required, the [traceBlock](https://geth.ethereum.org/docs/rpc/ns-debug#debug_traceblock) endpoint. If BlockScout still has errors, fix them until it doesn't.

It might make sense to use a special build flag to include this capability at build-time so the average node won't need to deal with the EVM debug overhead. Something like `evm_debug` should suffice.

# Tracers geth package

There is a tracer API interface that implements the debug API backend. If we can adapt the EVMChain struct to implement the tracers.Backend interface then we can simply create a tracers.API from the EVMChain and add it to the jsonrpc server under the debug namespace.