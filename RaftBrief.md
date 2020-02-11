# Raft

Fabric中Raft相关介绍翻译

作者 tinywell 日期 2019-04-17

From https://tinywell.com/2019/04/17/fabric-raft/

The go-to ordering service choice for production networks, the Fabric implementation of the established Raft protocol uses a “leader and follower” model, in which a leader is dynamically elected among the ordering nodes in a channel (this collection of nodes is known as the “consenter set”), and that leader replicates messages to the follower nodes. Because the system can sustain the loss of nodes, including leader nodes, as long as there is a majority of ordering nodes (what’s known as a “quorum”) remaining, Raft is said to be “crash fault tolerant” (CFT). In other words, if there are three nodes in a channel, it can withstand the loss of one node (leaving two remaining). If you have five nodes in a channel, you can lose two nodes (leaving three remaining nodes).

Raft 应是生产网络首选的共识算法，Fabric 对于 Raft 协议的实现采用了”领导者（leader）跟随者(follower)”模式，这个模式中领导者会由通道内的排序节点（这个通道内的排序节点集合即所谓的”共识集”）动态的选举产生，这个领导者会将消息复制到跟随者节点中。由于系统能够容忍节点（包括领导者节点）的丢失，只要系统中还保有大多数的排序节点(即所谓的”法定人数”)就可以，所以 Raft 被称之为”崩溃容错”(CFT)。换句话说，如果你的通道内有3个节点，那么系统可以容忍1个节点的丢失(留下剩余两个)。如果你的通道内有5个节点，那么你可以丢失2个节点(留下剩余3个)。

From the perspective of the service they provide to a network or a channel, Raft and the existing Kafka-based ordering service (which we’ll talk about later) are similar. They’re both CFT ordering services using the leader and follower design. If you are an application developer, smart contract developer, or peer administrator, you will not notice a functional difference between an ordering service based on Raft versus Kafka. However, there are a few major differences worth considering, especially if you intend to manage an ordering service:

从对提供给网络或者通道的服务的角度来看，Raft 和现有的基于 Kakfa 的排序服务(我们稍后会谈到)是差不多的。它们都是 CFT 类型的使用领导者跟随者设计模式的排序服务。如果你是一个应用开发者、智能合约开发者或者 peer 管理员，那么你将不会感受到基于 kafka 和 Raft 的排序服务之间的功能差异。但是，这里仍然有几点主要的不同值得考虑，尤其是如果你准备去管理一个排序服务的话：

Raft is easier to set up. Although Kafka has scores of admirers, even those admirers will (usually) admit that deploying a Kafka cluster and its ZooKeeper ensemble can be tricky, requiring a high level of expertise in Kafka infrastructure and settings. Additionally, there are many more components to manage with Kafka than with Raft, which means that there are more places where things can go wrong. And Kafka has its own versions, which must be coordinated with your orderers. With Raft, everything is embedded into your ordering node.

Raft 更容易部署。尽管 Kafka 有很多拥趸，即便这些拥趸也会承认部署一套 kafka 集群以及它依赖的 Zookeeper 非常的棘手，需要拥有 Kafka 的架构和配置方面高级的专业知识。另外，kafka 比 Raft 有更多的组件需要管理，这意味着会有更多的坑等着。而且 Kafka 有自己的版本管理，这需要你的排序服务配合调整。而使用 Raft，所有的功能都嵌入到排序节点功能中了。

Kafka and Zookeeper are not designed to be run across large networks. They are designed to be CFT but should be run in a tight group of hosts. This means that practically speaking you need to have one organization run the Kafka cluster. Given that, having ordering nodes run by different organizations when using Kafka (which Fabric supports) doesn’t give you much in terms of decentralization because the nodes will all go to the same Kafka cluster which is under the control of a single organization. With Raft, each organization can have its own ordering nodes, participating in the ordering service, which leads to a more decentralized system.

Kafka 和 Zookeeper 并不是针对跨大型网络运行而设计的。它们被设计为 CFT 但是必须在一个很紧密的集群中。这意味着实际上你需要一个单一组织运行 Kafka 集群。鉴于此，当那些由不同组织运行的排序节点使用 Kafka (fabric支持的)时并不能给你很多去中心化的感觉，因为所有的排序节点都要连到由某个单一组织控制的那个 Kafka 集群。而使用Raft，每个组织都能拥有自己的排序节点，参与到排序服务中，这让系统变得更加的去中心化。

Raft is supported natively. While Kafka-based ordering services are currently compatible with Fabric, users are required to get the requisite images and learn how to use Kafka and ZooKeeper on their own. Likewise, support for Kafka-related issues is handled through Apache, the open-source developer of Kafka, not Hyperledger Fabric. The Fabric Raft implementation, on the other hand, has been developed and will be supported within the Fabric developer community and its support apparatus.

Raft 是原生支持的。尽管基于 Kafka 的排序服务现在和fabric是兼容的，用户还是需要去获取依赖镜像并且学会如何使用 Kafka 和 Zookeeper。另外，针对 kafka 相关的问题的支持一直由 Apache 来处理 - 即 kafka 的开源开发者们，而不是超级账本 fabric 项目。另一方面，Fabric 的 Raft 实现，已经被 fabric 的开发社区开发并将会由社区以及其他支持资源提供支持。

Where Kafka uses a pool of servers (called “Kafka brokers”) and the admin of the orderer organization specifies how many nodes they want to use on a particular channel, Raft allows the users to specify which ordering nodes will be deployed to which channel. In this way, peer organizations can make sure that, if they also own an orderer, this node will be made a part of a ordering service of that channel, rather than trusting and depending on a central admin to manage the Kafka nodes.

Kafka 使用服务池(称之为”Kafka brokers”)，排序组织的管理员指定某个通道内需要多少节点，而 Raft 允许用户自己指定哪些排序节点部署到哪个通道。这种方式下，peer 组织就能够确保如果他们拥有排序节点，这个节点会被作为这个通道排序服务的一部分，而不是只能相信和依赖于一个中心化的管理员去管理 kafka 节点。

Raft is the first step toward Fabric’s development of a byzantine fault tolerant (BFT) ordering service. As we’ll see, some decisions in the development of Raft were driven by this. If you are interested in BFT, learning how to use Raft should ease the transition.

Raft 是迈向 Fabric 的拜占庭容错（BFT）共识服务的第一步。我们将会看到， Raft 开发过程中的一些决策正在向这个方向前进。如果你对 BFT 感兴趣，学习怎样使用 Raft 将会让这个跨度变容易。

Note: Similar to Solo and Kafka, a Raft ordering service can lose transactions after acknowledgement of receipt has been sent to a client. For example, if the leader crashes at approximately the same time as a follower provides acknowledgement of receipt. Therefore, application clients should listen on peers for transaction commit events regardless (to check for transaction validity), but extra care should be taken to ensure that the client also gracefully tolerates a timeout in which the transaction does not get committed in a configured timeframe. Depending on the application, it may be desirable to resubmit the transaction or collect a new set of endorsements upon such a timeout.

注意：和 Solo 、 Kafka 相似，Raft 排序服务也可能在交易回执发送给客户端之后出现丢失交易的情况。例如，领导者在跟随者发送回执大约差不多的时间崩溃。因此，应用客户端需要监听peer节点的交易提交事件（去检查交易合法性），但是需要额外注意的是要确保客户端也能容忍交易在配置的时间框架内没能成功提交的超时等待。出现这种超时时，可能需要重新提交一笔交易或者重新搜集背书。

## Raft concepts - Raft 相关概念

While Raft offers many of the same features as Kafka — albeit in a simpler and easier-to-use package — it functions substantially different under the covers from Kafka and introduces a number of new concepts, or twists on existing concepts, to Fabric.

尽管 Raft 提供了很多和 Kafka 一样的功能 - 用更小和更易用的包，它的底层功能和 Kafka 基本上不一样，它引入了很多新概念或者对 Fabric 已有的一些概念有些改变。

Log entry. The primary unit of work in a Raft ordering service is a “log entry”, with the full sequence of such entries known as the “log”. We consider the log consistent if a majority (a quorum, in other words) of members agree on the entries and their order, making the logs on the various orderers replicated.

日志条目。Raft 排序服务中的主要工作单元是”日志条目”，这些日志条目的完整序列被称为”日志”。我们记录同样的日志，如果大多数(即法定人数)成员同意这些条目的内容和它们的顺序，并将这些日志复制到其他排序节点上。

Consenter set. The ordering nodes actively participating in the consensus mechanism for a given channel and receiving replicated logs for the channel. This can be all of the nodes available (either in a single cluster or in multiple clusters contributing to the system channel), or a subset of those nodes.

共识集。排序节点积极参与到给定通道内的共识机制并接受此通道内的日志副本。这可以在所有的节点中实现(既可以是一个单一集群，也可以是贡献于系统通道的多个集群)，也可以是这些节点的子集。

Finite-State Machine (FSM). Every ordering node in Raft has an FSM and collectively they’re used to ensure that the sequence of logs in the various ordering nodes is deterministic (written in the same sequence).

有限状态机（FSM）。每个使用 Raft 的排序节点都有一个 FSM，总的来说它们被用来保障各个排序节点中的日志的一致性(同样的写入序列)。

Quorum. Describes the minimum number of consenters that need to affirm a proposal so that transactions can be ordered. For every consenter set, this is a majority of nodes. In a cluster with five nodes, three must be available for there to be a quorum. If a quorum of nodes is unavailable for any reason, the ordering service cluster becomes unavailable for both read and write operations on the channel, and no new logs can be committed.

法定人数。描述了一个交易能够被排序所需要的确认提案的最小的共识数量。对于每一个共识集，这个法定人数指”大多数”节点。在一个有5个节点的集群中，至少3个可用才能满足法定人数。如果有法定人数的节点因为任何原因不可用，那么这个通道内的排序服务集群就会既不能读也不能写，新的日志也不能被提交。

Leader. This is not a new concept — Kafka also uses leaders, as we’ve said — but it’s critical to understand that at any given time, a channel’s consenter set elects a single node to be the leader (we’ll describe how this happens in Raft later). The leader is responsible for ingesting new log entries, replicating them to follower ordering nodes, and managing when an entry is considered committed. This is not a special type of orderer. It is only a role that an orderer may have at certain times, and then not others, as circumstances determine.

领导者。这并不是一个新概念 - 我们说过，Kafka 也用到领导者 - 但是有一点需要知道：在任何给定时间里，一个通道的共识集只会选举出一个领导者节点(我们在后面会描述 raft 中这个过程是如何发生的)。 这个领导者负责收集新的日志条目，把它们复制给跟随者排序节点，并管理日志条目何时被提交。这并不是一个特殊”类型”的排序节点。它只是排序节点在某个特定时间内拥有同时别人不会拥有的一个角色，视情况而定。

Follower. Again, not a new concept, but what’s critical to understand about followers is that the followers receive the logs from the leader and replicate them deterministically, ensuring that logs remain consistent. As we’ll see in our section on leader election, the followers also receive “heartbeat” messages from the leader. In the event that the leader stops sending those message for a configurable amount of time, the followers will initiate a leader election and one of them will be elected the new leader.

跟随者。这同样不是一个新的概念，但是对于跟随者需要了解的是跟随者从领导者那里接收日志并严谨的复制他们，确保日志的一致性。我们将会在领导者选举章节看到，跟随者还会从领导者那里接收”心跳”消息。如果在一个配置的时间范围内领导者停止发送这些消息，跟随者们将会发起一次领导者选举然后它们中的一个会被选举为新的的领导者。

## Raft in a transaction flow - 交易流程中的 Raft

Every channel runs on a separate instance of the Raft protocol, which allows each instance to elect a different leader. This configuration also allows further decentralization of the service in use cases where clusters are made up of ordering nodes controlled by different organizations. While all Raft nodes must be part of the system channel, they do not necessarily have to be part of all application channels. Channel creators (and channel admins) have the ability to pick a subset of the available orderers and to add or remove ordering nodes as needed (as long as only a single node is added or removed at a time).

每个通道会运行在一个隔离的 Raft 实例上，这允许每个实例选举自己不同的领导者。这种构造允许在共识集群是由不同组织控制的排序节点组成的用户场景中进行更进一步的去中心化。虽然所有的 Raft 节点都必须包含在系统通道中，但他们并不需要都包含在应用通道中。通道创建者(和通道管理员)拥有选择可用排序节点子集的能力并且可以根据需要增加或移除排序节点(只要每次只增加会移除一个节点)

While this configuration creates more overhead in the form of redundant heartbeat messages and goroutines, it lays necessary groundwork for BFT.

尽管这种构造因冗余的心跳消息和 Goroutine 造成了过多的开销，但它为 BFT 打下了必要的基础。

In Raft, transactions (in the form of proposals or configuration updates) are automatically routed by the ordering node that receives the transaction to the current leader of that channel. This means that peers and applications do not need to know who the leader node is at any particular time. Only the ordering nodes need to know.

在 Raft 中，交易(以提案或者配置更新的形式)会被接收交易的排序节点自动路由到当前通道内的领导者哪里。这意味着 peer 节点和应用端并不需要知道当前时间内的领导者节点是哪个。只有排序节点需要知道这个。

When the orderer validation checks have been completed, the transactions are ordered, packaged into blocks, consented on, and distributed, as described in phase two of our transaction flow.

当排序服务的验证检查完成后，这些交易会被排序，打包到区块中，达成共识然后被分发，就像前面第二章交易流程(注：本文未翻译此节)里所描述的那样。

## Architectural notes - 架构点

### How leader election works in Raft - Raft 中如何进行 leader 选举

Although the process of electing a leader happens within the orderer’s internal processes, it’s worth noting how the process works.

尽管领导者选举的过程发生在排序服务的内部，这个过程如何发生仍然值得留意。

Raft nodes are always in one of three states: follower, candidate, or leader. All nodes initially start out as a follower. In this state, they can accept log entries from a leader (if one has been elected), or cast votes for leader. If no log entries or heartbeats are received for a set amount of time (for example, five seconds), nodes self-promote to the candidate state. In the candidate state, nodes request votes from other nodes. If a candidate receives a quorum of votes, then it is promoted to a leader. The leader must accept new log entries and replicate them to the followers.

一个 Raft 节点一般会处于三种状态中的一种：跟随者、候选者或者领导者。所有节点以”跟随者”身份启动。在这个状态，它们可以从领导者（如果有一个已经被选举出来了）那里接收日志条目，或者发出领导者选票。如果一段时间（比如5秒钟）内没有收到任何日志条目或者心跳消息，节点自发变更为”候选者”状态。在候选者状态下，节点从其他节点请求选票。一旦候选者收集到法定人数的选票，它就提升为”领导者”。领导者必须接收新的日志条目并将它们复制到其他跟随者。

For a visual representation of how the leader election process works, check out [The Secret Lives of Data](http://thesecretlivesofdata.com/raft/).

为了更直观的看到领导者选举的过程，可以参看[The Secret Lives of Data](http://thesecretlivesofdata.com/raft/)。

### Snapshots - 快照

If an ordering node goes down, how does it get the logs it missed when it is restarted?

如果排序一个节点挂掉了，它重启后如何获取错过的日志呢？

While it’s possible to keep all logs indefinitely, in order to save disk space, Raft uses a process called “snapshotting”, in which users can define how many bytes of data will be kept in the log. This amount of data will conform to a certain number of blocks (which depends on the amount of data in the blocks. Note that only full blocks are stored in a snapshot).

尽管可以无限保留所有日志，但为了节约磁盘空间，Raft 使用一种称作”快照”的机制，这样用户就可以定义日志中保留多少数据。这些数据会与一定数量的区块（有区块中的数据决定。注意只有完整的区块数据会存储到快照中）相匹配。

For example, let’s say lagging replica R1 was just reconnected to the network. Its latest block is 100. Leader L is at block 196, and is configured to snapshot at amount of data that in this case represents 20 blocks. R1 would therefore receive block 180 from L and then make a Deliver request for blocks 101 to 180. Blocks 180 to 196 would then be replicated to R1through the normal Raft protocol.

举个例子，一个叫R1的滞后副本节点刚刚重新接入到网络。它最后一个区块是100。领导者L现在最新区块是196，在这个场景中它的快照数据量是20个区块。因此R1会从L哪里收到区块180然后发起一个Deliver请求去请求区块101到180。而区块180到196会通过常规的 Raft 协议复制到R1上。