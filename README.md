# Zeniqsmart Node

## zeniqd

This repository contains the code of the full node client of zeniqsmart,
an EVM&amp;Web3 compatible sidechain for Zeniq.

`zeniqsmartd` needs a `zeniqd`.

`~/.zeniq/zeniq.conf`

```
server=1
rpcbind=127.0.0.1
rpcallowip=127.0.0.1/0
rpcport=57319
rpcuser=zeniq
rpcpassword=zeniq123

```

`rpcuser, rpcpassword` must be the same as for `~/.zeniqsmartd/config/app.toml` further down.

Start `zeniqd` with

`./src/zeniqd -printtoconsole`

and let it sync.

## zeniqsmartd

```
zeniqsmartd init my_moniker_here
```

This creates:

```
~/.zeniqsmartd/config
~/.zeniqsmartd/data
```

`~/.zeniqsmartd/data` contains the network data.

For the first node, edit

```
~/.zeniqsmartd/config/app.toml
~/.zeniqsmartd/config/config.toml
~/.zeniqsmartd/config/genesis.json
```

For subsequent nodes copy and override above 3 files in, then edit.

- `~/.zeniqsmartd/config/config.toml` is read by the Tendermint library.

  One should change this line in each copy of `config.toml`:

    moniker = "my_moniker_here"

  There needs to be at least one seed in `config.toml`.
  In the following example the node ID is via `zeniqsmartd show-node-id` in `192.168.0.40`,
  generated from `~/.zeniqsmartd/config/node_key.json`
  (which was created with `zeniqsmartd init my_moniker_here`).

  ```
  seeds = "ae00280df2729f655f35016bf95162c6cf71ab50@192.168.0.40:28888"
  persistent_peers = "ae00280df2729f655f35016bf95162c6cf71ab50@192.168.0.40:28888"
  ```

- `~/.zeniqsmartd/config/app.toml` configures the Tendermind application, i.e. the `zeniqsmartd` proper.

  - `cc-rpc-epochs` and `cc-rpc-fork-block` are consensus-relevant and must be the same in all nodes


  ``` toml
  get_logs_max_results = 10000
  retain-blocks = -1
  retain_interval_blocks = 100
  use_litedb = false
  with-syncdb = false
  blocks_kept_ads = 10000
  blocks_kept_db = -1
  sig_cache_size = 20000
  trunk_cache_size = 200
  prune_every_n = 10
  recheck_threshold = 1000
  # there must be a zeniqd running (on the same machine)
  mainnet-rpc-url = "http://127.0.0.1:57319"
  mainnet-genesis-height = 121900
  mainnet-rpc-username = "zeniq"
  mainnet-rpc-password = "zeniq123"
  zeniqsmart-rpc-url = "https://smart3.zeniq.network:9545"
  # consensus-relevant:
  cc-rpc-epochs = [ [184464, 6, 7200], [185184, 144, 172800], [185472, 1008, 1209600, 9876543211] ]
  cc-rpc-fork-block = 11000011
  ```

New app.toml settings:

- `cc-rpc-epochs`:
  `zeniqsmartd` calls the `zeniqd` RPC function `crosschain` to fetch and account
  `OP_FALSE OP_VERIFY OP_RETURN <agentdata>` transactions from mainchain.
  This is called the `CCRPC` feature in `zeniqsmartd` (`cc-rpc-` in `app.toml`).

  Each entry has 3 or 4 numbers:

  - mainchain height
  - epoch length in mainchain blocks
  - how much delay in smart blocks before accounting an epoch
  - optionally a 4th `minimum` tells to filter `crosschain` by minimum in Satoshi

- `cc-rpc-fork-block` is the smart height when the `CCRPC` feature is activated in `zeniqsmartd`.

- `blocks-behind = 0`, 0 being default, can be set other than 0 to keep a running backup.
  The sync always keeps the given number of blocks behind the current head of validators.

- 'update-of-ads-log = false', can be set true temporarily to create text files
  `updateOfADS<height>.txt` in the ~/.zeniqsmartd/data/app folder
  showing which 'key=value/old' changes (keys that got a new value over an old one)
  happened at that height.
  This overlaps with `with-syncdb`, which stores in a db but without the old value.

Use zeniqsmartd's `zeniq_crosschainInfo` for infos on applied `crosschain` transactions.


