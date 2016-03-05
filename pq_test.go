package swp

import (
	"container/heap"
	//"fmt"
	cv "github.com/glycerine/goconvey/convey"
	"testing"
	"time"
)

func Test005PriorityQueue(t *testing.T) {

	cv.Convey("given a priority queue of packet timeouts, the earliest timeout should sort first, and zero timeouts (cancelled) should sort last", t, func() {
		// Some items and their priorities.
		n := int64(10)
		txq := make([]*TxqSlot, n)
		pq := make(PriorityQueue, n)
		for k := range txq {
			i := int64(k)
			txq[i] = &TxqSlot{
				RetryDeadline: time.Unix(10+(n-i)-1, 0),
				Pack: &Packet{
					SeqNum: Seqno(i),
				},
			}
			pq[i] = &PqEle{
				slot:  txq[i],
				index: k,
			}
		}

		heap.Init(&pq)

		// Insert a new item and then modify its priority.
		item := &PqEle{
			slot: &TxqSlot{
				RetryDeadline: time.Time{},
				Pack: &Packet{
					SeqNum: Seqno(99987),
				},
			},
		}
		heap.Push(&pq, item)
		/*
			fmt.Printf("\n\n")
			for i := range pq {
				fmt.Printf(" at: %v, seqnum: %v\n", i, pq[i].slot.Pack.SeqNum)
			}
		*/
		p("with zero time, the TxnEle should sort to the end of the priority queue")
		cv.So(pq[n].slot.Pack.SeqNum, cv.ShouldEqual, 99987)

		p("and if we change that time to be non-zero and sooner than everyone else, we should sort first")
		item.slot.RetryDeadline = time.Unix(1, 0)
		pq.update(item, item.slot)

		// Take the items out; they arrive in decreasing priority order.
		j := 0
		for pq.Len() > 0 {
			item := heap.Pop(&pq).(*PqEle)
			if j == 0 {
				cv.So(item.slot.Pack.SeqNum, cv.ShouldEqual, 99987)
			}
			//fmt.Printf("%v: seqnum: %v\n", item.slot.RetryDeadline, item.slot.Pack.SeqNum)
			j++
		}
	})
}
