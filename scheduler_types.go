package main

import (
	"time"
)

type Scheduable interface {
	SchedulerAction()
}

type SchedulerData struct {
	when   time.Time
	index  int
	object Scheduable
}

type ScheduleHeap []*SchedulerData

func (h ScheduleHeap) Len() int {
	return len(h)
}
func (h ScheduleHeap) Less(i, j int) bool {
	return h[i].when.Before(h[j].when)
}
func (h ScheduleHeap) Swap(i, j int) {
	h[i].index, h[j].index = h[j].index, h[i].index
	h[i], h[j] = h[j], h[i]
}

func (h *ScheduleHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*SchedulerData))
	x.(*SchedulerData).index = len(*h) - 1
}

func (h *ScheduleHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	x.index = -1
	return x
}

type SchedUpdateType int

const (
	SchedUpdateTypeUnknown SchedUpdateType = iota
	SchedUpdateTypeAdd
	SchedUpdateTypeRem
)

func (u SchedUpdateType) String() string {
	switch u {
	case SchedUpdateTypeAdd:
		return "Add"
	case SchedUpdateTypeRem:
		return "Remove"
	}
	return "Unknown"
}

type SchedUpdate struct {
	event SchedUpdateType
	d     Scheduable
	when  time.Time
}
