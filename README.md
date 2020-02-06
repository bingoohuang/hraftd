# hraftd

hraftd is a reference example use of the Hashicorp Raft. 

This project is rewritten from [otoolep/hraftd](https://github.com/otoolep/hraftd) and this [blog post](http://www.philipotoole.com/building-a-distributed-key-value-store-using-raft/)..

[Hashicorp](https://hashicorp.com/) provide a [nice implementation](https://github.com/hashicorp/raft) 
of the [Raft](https://raft.github.io/) consensus protocol, 
and it’s at the heart of [InfluxDB](http://www.influxdb.com/), [consul](http://www.consul.io/)
(amongst [other systems](https://godoc.org/github.com/hashicorp/raft?importers)). 

![image](https://user-images.githubusercontent.com/1940588/73939073-a0485780-4923-11ea-9b61-73fd344660bd.png)

Raft is a _distributed consensus protocol_, meaning its purpose is to ensure 
that a set of nodes -- a cluster -- agree on the state of some arbitrary state machine, 
even when nodes are vulnerable to failure and network partitions. 
Distributed consensus is a fundamental concept when it comes to building fault-tolerant systems.

A simple example system like hraftd makes it easy to study the Raft consensus protocol in general, 
and Hashicorp's Raft implementation in particular. It can be run on Linux, OSX, and Windows.

## Reading and writing keys

The reference implementation is a very simple in-memory key-value store. 
You can set a key by sending a request to the HTTP bind address (which defaults to `localhost:11000`):

```bash
curl -XPOST localhost:11000/key -d '{"foo": "bar"}'
```

You can read the value for a key like so:

```bash
curl -XGET localhost:11000/key/foo
```

## Running hraftd

Starting and running a hraftd cluster is easy. Download hraftd like so:

`go get github.com/bingoohuang/hraftd/cmd/hraftd`

Run your first hraftd node like so:

`hraftd`

You can now set a key and read its value back:

```bash
curl -XPOST localhost:11000/key -d '{"user1": "batman"}'
curl localhost:11000/key/user1
```

### Bring up a cluster locally

Let's bring up 2 more nodes, so we have a 3-node cluster. That way we can tolerate the failure of 1 node:

```bash
hraftd -haddr :11000 -raddr :12000 -join :11000
hraftd -haddr :11001 -raddr :12001 -join :11000
hraftd -haddr :11002 -raddr :12002 -join :11000
```

This example shows each hraftd node running on the same host, so each node must listen on different ports. 
This would not be necessary if each node ran on a different host.

This tells each new node to join the existing node. Once joined, each node now knows about the key:

```bash
curl localhost:11000/key/user1 localhost:11001/key/user1 localhost:11002/key/user1
```

Furthermore you can add a second key:

```bash
curl -XPOST localhost:11000/key -d '{"user2": "robin"}'
```

Confirm that the new key has been set like so:

```bash
$ curl localhost:11000/key/user2 localhost:11001/key/user2 localhost:11002/key/user2
{"user2":"robin"}{"user2":"robin"}{"user2":"robin"}
```

```bash
$ curl localhost:11000/raft/stats localhost:11001/raft/stats localhost:11002/raft/stats |jq
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100   532  100   532    0     0   519k      0 --:--:-- --:--:-- --:--:--  519k
100   544  100   544    0     0   531k      0 --:--:-- --:--:-- --:--:--  531k
100   544  100   544    0     0   531k      0 --:--:-- --:--:-- --:--:--  531k
```

output:

```json
{
  "applied_index": "10",
  "commit_index": "10",
  "fsm_pending": "0",
  "last_contact": "0",
  "last_log_index": "10",
  "last_log_term": "13",
  "last_snapshot_index": "0",
  "last_snapshot_term": "0",
  "latest_configuration": "[{Suffrage:Voter ID:192.168.10.101:12000 Address::12000} {Suffrage:Voter ID:192.168.10.101:12001 Address::12001} {Suffrage:Voter ID:192.168.10.101:12002 Address::12002}]",  "latest_configuration_index": "0",
  "num_peers": "2",
  "protocol_version": "3",
  "protocol_version_max": "3",
  "protocol_version_min": "0",
  "snapshot_version_max": "1",
  "snapshot_version_min": "0",
  "state": "Leader",
  "term": "13"
}
```

```json
{
  "applied_index": "10",
  "commit_index": "10",
  "fsm_pending": "0",
  "last_contact": "36.160584ms",
  "last_log_index": "10",
  "last_log_term": "13",
  "last_snapshot_index": "0",
  "last_snapshot_term": "0",
  "latest_configuration": "[{Suffrage:Voter ID:192.168.10.101:12000 Address::12000} {Suffrage:Voter ID:192.168.10.101:12001 Address::12001} {Suffrage:Voter ID:192.168.10.101:12002 Address::12002}]",  "latest_configuration_index": "0",
  "num_peers": "2",
  "protocol_version": "3",
  "protocol_version_max": "3",
  "protocol_version_min": "0",
  "snapshot_version_max": "1",
  "snapshot_version_min": "0",
  "state": "Follower",
  "term": "13"
}
```

```json
{
  "applied_index": "10",
  "commit_index": "10",
  "fsm_pending": "0",
  "last_contact": "15.108323ms",
  "last_log_index": "10",
  "last_log_term": "13",
  "last_snapshot_index": "0",
  "last_snapshot_term": "0",
  "latest_configuration": "[{Suffrage:Voter ID:192.168.10.101:12000 Address::12000} {Suffrage:Voter ID:192.168.10.101:12001 Address::12001} {Suffrage:Voter ID:192.168.10.101:12002 Address::12002}]",  "latest_configuration_index": "0",
  "num_peers": "2",
  "protocol_version": "3",
  "protocol_version_max": "3",
  "protocol_version_min": "0",
  "snapshot_version_max": "1",
  "snapshot_version_min": "0",
  "state": "Follower",
  "term": "13"
}
```

### Multi-node Clustering realistically

What follows is a detailed example of running a multi-node hraftd cluster.

Imagine you have 3 machines, with the IP addresses 192.168.0.1, 192.168.0.2, and 192.168.0.3 respectively. 
Let's also assume that each machine can reach the other two machines using these addresses.

You should start the first node like so:

```
hraftd -haddr 192.168.0.1:11000 -raddr 192.168.0.1:12000
```

This way the node is listening on an address reachable from the other nodes. This node will start up and become leader of a single-node cluster.

Next, start the second node as follows:

```
hraftd -haddr 192.168.0.2:11000 -raddr 192.168.0.2:12000 -join 192.168.0.1:11000
```

Finally, start the third node as follows:

```
hraftd -haddr 192.168.0.3:11000 -raddr 192.168.0.3:12000 -join 192.168.0.2:11000
```

Specifically using ports 11000 and 12000 is not required. You can use other ports if you wish.

Note how each node listens on its own address, but joins to the address of the leader node. 
The second and third nodes will start, join the with leader at `192.168.0.2:11000`, and a 3-node cluster will be formed.


#### Stale reads

Because any node will answer a GET request, and nodes may "fall behind" updates, stale reads are possible. Again, 
hraftd is a simple program, for the purpose of demonstrating a distributed key-value store.
If you are particularly interested in learning more about issue, you should check out [rqlite](https://github.com/rqlite/rqlite). 
rqlite allows the client to control [read consistency](https://github.com/rqlite/rqlite/blob/master/DOC/CONSISTENCY.md), 
allowing the client to trade off read-responsiveness and correctness.

Read-consistency support could be ported to hraftd if necessary.

### Tolerating failure

Kill the leader process and watch one of the other nodes be elected leader. 
The keys are still available for query on the other nodes, and you can set keys on the new leader. 
Furthermore, when the first node is restarted, it will rejoin the cluster and learn about any updates that occurred while it was down.

A 3-node cluster can tolerate the failure of a single node, but a 5-node cluster can tolerate the failure of two nodes. 
But 5-node clusters require that the leader contact a larger number of nodes before any change 
e.g. setting a key's value, can be considered committed.

### Leader-forwarding

Automatically forwarding requests to set keys to the current leader is not implemented. 
The client must always send requests to change a key to the leader or an error will be returned.

## Production use of Raft

For a production-grade example of using Hashicorp's Raft implementation, 
to replicate a SQLite database, check out [rqlite](https://github.com/rqlite/rqlite).


## Resources

1. [基于hashicorp/raft的分布式一致性实战教学](https://zhuanlan.zhihu.com/p/58048906)
1. [raft可视化](http://thesecretlivesofdata.com/raft/)
1. [Leto - 基于raft快速实现一个key-value存储系统](https://xiking.win/2018/07/30/implement-key-value-store-using-raft/)