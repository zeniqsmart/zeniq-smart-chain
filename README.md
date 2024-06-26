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
  - `watcher-speedup` will fetch epoch history from other smart nodes


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
  watcher-speedup = true
  zeniqsmart-rpc-url = "https://smart3.zeniq.network:9545"
  # consensus-relevant: [[mainnetHeight>=184464,n>=6,>=n*1200],...]
  cc-rpc-epochs = [ [184464, 6, 7200], [185184, 144, 172800], [185472, 1008, 1209600] ]
  cc-rpc-fork-block = 11000011
  ```

New app.toml settings:

- `cc-rpc-epochs` see below
- `cc-rpc-fork-block` see below
- `blocks-behind = 0`, 0 being default, can be set other than 0 to keep a running backup.
  The sync alway keeps the given number of blocks behind the current head of validators.
- 'update-of-ads-log = false', can be set true temporarily to create text files
  `updateOfADS<height>.txt` in the ~/.zeniqsmartd/data/app folder
  showing which 'key=value/old' changes (keys that got a new value over an old one)
  happened at that height.
  This overlaps with `with-syncdb`, which stores in a db but without the old value.


Start via 

```
zeniqsmartd start
```

`~/.zeniqsmartd/data` contains the network data.
The sync takes 3-9 days.

### CCRPC

`zeniqsmartd` implements crosschain in one direction
from Zeniq mainchain to the smart chain
by calling the Zeniq RPC function "crosschain"
regularly per epoch:

- `cc-rpc-epochs` defines at what mainchain height which epoch length in blocks starts getting valid.
  n=1008 blocks would be about 1 week corresponding to one epoch length.
- `cc-rpc-fork-block` is the smart height when this feature is activated in `zeniqsmartd`
  Make this a nn earlier than the first main height in `cc-rpc-epochs`.

In Zeniq mainchain a transaction with a special output's scriptPubKey
`OP_FALSE OP_VERIFY OP_RETURN <agentdata>`
that makes the input unusable in mainchain, i.e. burns it there,
is called a crosschain transaction further down.

The steps taken by `zeniqsmartd` are these:

::

  n=1008           # epoch
  2*n              # 2016 fetch
  nn_n = 10*60/3   # every 3 seconds smart, every 10 min main
  nnmin = 2*n*nn_n # search back to find cc tx, a failure would be very probably with the min value
  nn = 3*nnmin     # a safer value
  nn/n == 1200     # in cc-rpc-epochs=[[main,n,nn]] make nn = 1200*n


