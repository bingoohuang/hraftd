package hraftd

import "reflect"

// DistributedItem is the data wrapper with NodeID.
type DistributedItem struct {
	Data   Identifier `json:"data"`
	NodeID NodeID     `json:"nodeID"`
}

// Identifier gives the ID getter.
type Identifier interface {
	ID() string
}

// Distributor is the role to charge the distribution among the hraft cluster nodes.
type Distributor struct {
	// sticky to the previous nodeID when redistribute every time.
	stickyMap map[string]NodeID
}

// NewDistributor makes a new Distributor.
func NewDistributor() *Distributor {
	d := &Distributor{
		stickyMap: make(map[string]NodeID),
	}

	return d
}

// Distribute do the distribution.
func (d *Distributor) Distribute(peers []Peer, data interface{}) []DistributedItem {
	d.cleanKeysNotIn(peers)

	dv := reflect.ValueOf(data)
	if dv.Type().Kind() != reflect.Slice {
		panic("data should be slice")
	}

	if !dv.Type().Elem().Implements(reflect.TypeOf((*Identifier)(nil)).Elem()) {
		panic("data should implements Identifier")
	}

	dataLen := dv.Len()
	// 预先计算每个节点可以安放的数量
	peersNumMap := makePeersMap(peers, dataLen)
	// 分配结果
	distributed := make([]DistributedItem, 0, dataLen)
	// 需要新分配的项目
	newItems := make([]Identifier, 0, dataLen)

	// 先保持粘滞
	for i := 0; i < dataLen; i++ {
		item := dv.Index(i).Interface().(Identifier)
		if nodeID := d.stickyMap[item.ID()]; nodeID != "" {
			distributed = append(distributed, DistributedItem{Data: item, NodeID: nodeID})
			peersNumMap[nodeID]--
		} else {
			newItems = append(newItems, item)
		}
	}

	// 再分配剩余
NextItem:
	for _, item := range newItems {
		for _, peer := range peers {
			leftNum := peersNumMap[peer.ID]
			if leftNum <= 0 {
				continue
			}

			d.stickyMap[item.ID()] = peer.ID
			distributed = append(distributed, DistributedItem{Data: item, NodeID: peer.ID})
			peersNumMap[peer.ID]--

			continue NextItem
		}
	}

	return distributed
}

func makePeersMap(peers []Peer, dataLen int) map[NodeID]int {
	peersNumMap := make(map[NodeID]int)

	for i := 0; i < dataLen; i++ {
		p := peers[i%len(peers)]
		peersNumMap[p.ID]++
	}

	return peersNumMap
}

func (d *Distributor) cleanKeysNotIn(peers []Peer) {
	peersMap := make(map[NodeID]bool)

	for _, p := range peers {
		peersMap[p.ID] = true
	}

	for id, nodeID := range d.stickyMap {
		if _, ok := peersMap[nodeID]; !ok {
			delete(d.stickyMap, id)
		}
	}
}
