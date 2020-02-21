# hraftd

hraftd is a reference usage of the Hashicorp Raft. Read more about [raft brief](RaftBrief.md).

[Hashicorp](https://hashicorp.com/) provide a [nice implementation](https://github.com/hashicorp/raft)
of the [Raft](https://raft.github.io/) consensus protocol,
and it’s at the heart of [InfluxDB](http://www.influxdb.com/), [consul](http://www.consul.io/)
(amongst [other systems](https://godoc.org/github.com/hashicorp/raft?importers)).

It is rewritten from [otoolep/hraftd](https://github.com/otoolep/hraftd) 
and this [blog post](http://www.philipotoole.com/building-a-distributed-key-value-store-using-raft/).

![image](https://user-images.githubusercontent.com/1940588/73939073-a0485780-4923-11ea-9b61-73fd344660bd.png)

Raft is a _distributed consensus protocol_, meaning its purpose is to ensure
that a set of nodes -- a cluster -- agree on the state of some arbitrary state machine,
even when nodes are vulnerable to failure and network partitions.

Distributed consensus is a fundamental concept when it comes to building fault-tolerant systems.

A simple example like hraftd makes it easy to study the Raft consensus protocol in general,
and Hashicorp's Raft implementation in particular. It can be run on Linux, OSX, and Windows.

## Reading and writing keys

The reference implementation is a very simple in-memory key-value store.
You can set/get a key by sending a request to the HTTP bind address (which defaults to `localhost:11000`):

```bash
curl -v -H "Content-Type: application/json" localhost:11000/hraftd/key -d '{"foo": "bar"}'
curl localhost:11000/hraftd/key/foo
```

or use [httpie](https://httpie.org/)

```bash
http :11000/hraftd/key foo=barst
http :11000/hraftd/key/foo
```

## Running hraftd

Starting and running a hraftd cluster is easy. Download hraftd like so:

`go get github.com/bingoohuang/hraftd/cmd/hraftd`

Run your first hraftd node like so:

`hraftd`

You can now set a key and read its value back:

```bash
curl -v -H "Content-Type: application/json" localhost:11000/hraftd/key -d '{"user1": "batman"}'
curl localhost:11000/hraftd/key/user1
```

### Bring up a cluster locally

Let's bring up 3-node cluster. That way we can tolerate the failure of 1 node:

```bash
hraftd --rjoin :11000 --haddr :11000
hraftd --rjoin :11000 --haddr :11001
hraftd --rjoin :11000 --haddr :11002
```

or user supervisord

```bash
supervisord -c supervisord.ini
```

This example shows each hraftd node running on the same host, so each node must listen on different ports.
This would not be necessary if each node ran on a different host.

This tells each new node to join the existing node. Once joined, each node now knows about the key:

`curl localhost:11000/hraftd/key/user1 localhost:11001/key/user1 localhost:11002/key/user1`

Furthermore, you can add a second key:

`curl localhost:11000/hraftd/key -d '{"user2": "robin"}'`

Confirm that the new key has been set like so:

```bash
$ curl localhost:11000/hraftd/key/user2 localhost:11001/key/user2 localhost:11002/key/user2
{"user2":"robin"}{"user2":"robin"}{"user2":"robin"}
$ curl localhost:11000/hraftd/raft/stats |jq
```

output:

```json
{
  "applied_index": "7",
  "commit_index": "7",
  "fsm_pending": "0",
  "last_contact": "3.685538ms",
  "last_log_index": "7",
  "last_log_term": "8",
  "last_snapshot_index": "0",
  "last_snapshot_term": "0",
  "latest_configuration": [
    {
      "id": "192.168.10.101:11001,192.168.10.101:12001",
      "address": "192.168.10.101:12001",
      "suffrage": "Voter"
    },
    {
      "id": "192.168.10.101:11003,192.168.10.101:12003",
      "address": "192.168.10.101:12003",
      "suffrage": "Voter"
    },
    {
      "id": "192.168.10.101:11004,192.168.10.101:12004",
      "address": "192.168.10.101:12004",
      "suffrage": "Voter"
    },
    {
      "id": "192.168.10.101:11002,192.168.10.101:12002",
      "address": "192.168.10.101:12002",
      "suffrage": "Voter"
    }
  ],
  "latest_configuration_index": "0",
  "num_peers": "2",
  "protocol_version": "3",
  "protocol_version_max": "3",
  "protocol_version_min": "0",
  "snapshot_version_max": "1",
  "snapshot_version_min": "0",
  "state": "Follower",
  "term": "8"
}
```


`curl localhost:11000/hraftd/raft/cluster | python -m json.tool`

or

`http :11000/hraftd/raft/cluster`

output:

```json
{
  "current": {
    "address": ":12000",
    "id": ":11000,:12000",
    "state": "Follower"
  },
  "leader": {
    "address": ":12002",
    "id": ":11002,:12002",
    "state": "Leader"
  },
  "servers": [
    {
      "address": ":12000",
      "id": ":11000,:12000",
      "state": "Follower"
    },
    {
      "address": ":12001",
      "id": ":11001,:12001",
      "state": "Follower"
    },
    {
      "address": ":12002",
      "id": ":11002,:12002",
      "state": "Leader"
    }
  ]
}
```

### Multi-node Clustering realistically

What follows is a detailed example of running a multi-node hraftd cluster.

Imagine you have 3 machines, with the IP addresses 192.168.0.1, 192.168.0.2, and 192.168.0.3 respectively.
Let's also assume that each machine can reach the other two machines using these addresses.

You should start the nodes (eg at 192.168.0.1,192.168.0.2,192.168.0.3) like so:

```
hraftd -haddr :11000 -rjoin 192.168.0.1:11000
```

Specifically using ports 11000 and 12000 is not required. You can use other ports if you wish.

Note how each node listens on its own address, but joins to the address of the leader node.
The second and third nodes will start, join the leader at `192.168.0.2:11000`, and a 3-node cluster will be formed.

```bash
$ lsof -i tcp:11000
COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
XiYouProx  5344 bingoo  115u  IPv4 0x8708c2e9ace67df5      0t0  TCP localhost:59712->localhost:irisa (ESTABLISHED)
hraftd    73100 bingoo    9u  IPv6 0x8708c2e9bc6d07ad      0t0  TCP *:irisa (LISTEN)
hraftd    73100 bingoo   11u  IPv6 0x8708c2e9bc6d0dcd      0t0  TCP localhost:irisa->localhost:59712 (ESTABLISHED)

$ ps -ef|grep raft
  502 73100 35405   0 10:39上午 ttys001    0:00.21 hraftd -haddr :11000 -raddr :12000 -rjoin :11000
  502 72406 46043   0 10:30上午 ttys008    0:02.41 hraftd -haddr :11001 -raddr :12001 -rjoin :11000
  502 72405 46094   0 10:30上午 ttys009    0:06.71 hraftd -haddr :11002 -raddr :12002 -rjoin :11000
```

remove node from cluster:

```bash
$ http -v DELETE :11001/raft/remove id=192.168.10.101:11004,192.168.10.101:12004
DELETE /raft/remove HTTP/1.1
Accept: application/json, */*
Accept-Encoding: gzip, deflate
Connection: keep-alive
Content-Length: 51
Content-Type: application/json
Host: localhost:11001
User-Agent: HTTPie/1.0.2

{
    "id": "192.168.10.101:11004,192.168.10.101:12004"
}

HTTP/1.1 200 OK
Content-Length: 34
Content-Type: application/json; charset=utf-8
Date: Thu, 13 Feb 2020 15:49:15 GMT

{
    "msg": "OK",
    "ok": true
}

```

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

1. [The Raft Consensus Algorithm，包括可视化，包括节点状态](https://raft.github.io/)
1. [Raft算法原理](https://www.codedump.info/post/20180921-raft/)
1. [基于 hashicorp/raft 的分布式一致性实战教学](https://zhuanlan.zhihu.com/p/58048906)
1. [raft 可视化](http://thesecretlivesofdata.com/raft/)
1. [Leto - 基于 raft 快速实现一个 key-value 存储系统](https://xiking.win/2018/07/30/implement-key-value-store-using-raft/)
1. [使用 Raft 实现 VIP 功能](https://zdyxry.github.io/2020/01/17/%E4%BD%BF%E7%94%A8-Raft-%E5%AE%9E%E7%8E%B0-VIP-%E5%8A%9F%E8%83%BD/)
