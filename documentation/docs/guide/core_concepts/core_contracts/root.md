---
description: The root contract is the first smart contract deployed on the chain. It functions as a smart contract factory for the chain, and manages chain ownership and fees.
image: /img/logo/WASP_logo_dark.png
keywords:
- smart contracts
- core
- root
- initialization
- entry points
- fees
- ownership
- views
- reference
--- 
# The `root` Contract

The `root` contract is one of the [core contracts](overview.md) on each IOTA Smart Contracts
chain.

The `root` contract provides the following functions:

- It is the first smart contract deployed on the chain. Upon receiving the `init` request, bootstraps the state of the chain. Part of the state initialization is the deployment of all other core contracts.

- It functions as a smart contract factory for the chain: upon request, it deploys other smart contracts and maintains an on-chain registry of smart contracts in its state.

- The contract registry keeps a list of contract records, which contain their respective name, hname, description and creator.

---

## Entry Points

The following are the functions/entry points of the `root` contract. Some of
them may require authorisation, i.e. can only be invoked by a specific caller,
for example the _chain owner_.

### `init()`

The constructor. Automatically posted to the chain immediately after confirmation of the origin transaction, as the first call.

- Initializes base values of the chain according to parameters
- Sets the caller as the _chain owner_
- Sets chain fee color (default is _IOTA color_)
- Deploys all core contracts. The core contracts become part of the immutable state.
  It makes them callable just like any other smart contract deployed on the chain.

Cannot be called again, after the chain has been initialized.

### `deployContract(ph ProgramHash, ds Description, nm Name)`

Deploys a WASM smart contract on the chain, if the caller has deploy permission.
It expects the following parameters:

- `ph` the hash of the binary _blob_ (that has been previously stored in the [blob contract](blob.md)),
- `ds` description of the contract to be deployed,
- `nm` the name of the contract to be deployed. This is later used in the hashed form of _hname_.

### `grantDeployPermission(dp AgentID)`

The chain owner grants deploy permission to the agent ID `dp`.

### `revokeDeployPermission(dp AgentID)`

The chain owner revokes the deploy permission of the agent ID `dp`.

### `requireDeployPermissions(de DeployPermissionsEnabled)`

- `de`: true | false - whether permissions should be required to deploy a contract on the chain.

By default permissions are enabled (addresses need to be granted the right to deploy), but the chain owner can override this setting to allow anyone to deploy contracts on the chain.

---

## Views

Can be called directly. Calling a view does not modify the state of the smart
contract.

### `findContract(hn Hname)`

Returns the record for a given smart contract with Hname `hn` (if it exists).

### getContractRecords

Returns the list of all smart contracts deployed on the chain and related records.
