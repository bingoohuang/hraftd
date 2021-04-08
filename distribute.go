package hraftd

import "reflect"

// DistributedApplier is the data applier with NodeID.
type DistributedApplier interface {
	Distribute(NodeID NodeID, item Identifier)
}

// Identifier gives the ID getter.
type Identifier interface {
	ID() string
}

// Distributor is the role to charge the distribution among the hraft cluster nodes.
type Distributor struct {
	// sticky to the previous nodeID when redistribute every time.
	StickyMap map[string]NodeID
}

// NewDistributor makes a new Distributor.
func NewDistributor() *Distributor {
	d := &Distributor{
		StickyMap: make(map[string]NodeID),
	}

	return d
}

// nolint:gochecknoglobals
var (
	distributedApplierType = reflect.TypeOf((*DistributedApplier)(nil)).Elem()
	identifierType         = reflect.TypeOf((*Identifier)(nil)).Elem()
)

// Distribute do the distribution.
func (d *Distributor) Distribute(peers []Peer, data, emptyReceiver interface{}) interface{} {
	rt := checkReceiverType(emptyReceiver)
	dv := d.checkDataType(data)

	d.cleanKeysNotIn(peers)

	dataLen := dv.Len()
	// 预先计算每个节点可以安放的数量
	peersNumMap := makePeersMap(peers, dataLen)
	// 分配结果
	distributed := reflect.MakeSlice(reflect.SliceOf(rt), 0, dataLen)
	// 需要新分配的项目
	newItems := make([]Identifier, 0, dataLen)

	// 先保持粘滞
	for i := 0; i < dataLen; i++ {
		item := dv.Index(i).Interface().(Identifier)
		if nodeID := d.StickyMap[item.ID()]; nodeID != "" {
			v := reflect.New(rt)
			a := v.Interface().(DistributedApplier)
			a.Distribute(nodeID, item)

			distributed = reflect.Append(distributed, v.Elem())
			peersNumMap[nodeID]--
		} else {
			newItems = append(newItems, item)
		}
	}

	// 再分配剩余
	for _, item := range newItems {
		for _, peer := range peers {
			if peersNumMap[peer.ID] <= 0 {
				continue
			}

			d.Put(item.ID(), peer.ID)

			v := reflect.New(rt)
			a := v.Interface().(DistributedApplier)
			a.Distribute(peer.ID, item)

			distributed = reflect.Append(distributed, v.Elem())
			peersNumMap[peer.ID]--

			break
		}
	}

	return distributed.Interface()
}

// Put puts the node ID related to id directly.
func (d *Distributor) Put(id string, nodeID NodeID) {
	d.StickyMap[id] = nodeID
}

func (d *Distributor) checkDataType(data interface{}) reflect.Value {
	dv := reflect.ValueOf(data)

	if dv.Type().Kind() != reflect.Slice {
		panic("data should be slice")
	}

	if !dv.Type().Elem().Implements(identifierType) {
		panic("data should implements Identifier")
	}

	return dv
}

func checkReceiverType(emptyReceiver interface{}) reflect.Type {
	rt := reflect.TypeOf(emptyReceiver)
	rtPtr := rt

	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	} else {
		rtPtr = reflect.PtrTo(rt)
	}

	if !rtPtr.Implements(distributedApplierType) {
		panic("receiver type should implement *DistributedApplier")
	}

	return rt
}

func makePeersMap(peers []Peer, dataLen int) map[NodeID]int {
	peersNumMap := make(map[NodeID]int)

	for i := 0; i < dataLen; i++ {
		p := peers[i%len(peers)]
		peersNumMap[p.ID]++
	}

	return peersNumMap
}

// CleanSticky cleans the sticky map state.
func (d *Distributor) CleanSticky() {
	d.StickyMap = make(map[string]NodeID)
}

func (d *Distributor) cleanKeysNotIn(peers []Peer) {
	peersMap := make(map[NodeID]bool)

	for _, p := range peers {
		peersMap[p.ID] = true
	}

	for id, nodeID := range d.StickyMap {
		if _, ok := peersMap[nodeID]; !ok {
			delete(d.StickyMap, id)
		}
	}
}
