package main

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultSchedulingCapacity = 10
	activationDuration        = time.Millisecond * time.Duration(1)
)

type SchedulerData struct {
	when     time.Time
	index    int
	callback func(d *Device)
}

type DeviceSchedule []*Device

func (h DeviceSchedule) Len() int           { return len(h) }
func (h DeviceSchedule) Less(i, j int) bool { return h[i].when.Before(h[j].when) }
func (h DeviceSchedule) Swap(i, j int) {
	h[i].index, h[j].index = h[j].index, h[i].index
	h[i], h[j] = h[j], h[i]
}

func (h *DeviceSchedule) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*Device))
	x.(*Device).index = len(*h) - 1
}

func (h *DeviceSchedule) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	x.index = -1
	return x
}

type SchedUpdateType int

const (
	SchedUpdateTypeAdd SchedUpdateType = iota
	SchedUpdateTypeRem
)

type SchedUpdate struct {
	event SchedUpdateType
	d     *Device
}

type Scheduler struct {
	dsched   DeviceSchedule
	updates  chan SchedUpdate
	shutdown chan bool
	wg       sync.WaitGroup
}

func NewScheduler() *Scheduler {
	sched := new(Scheduler)
	sched.dsched = make(DeviceSchedule, 0, defaultSchedulingCapacity)
	sched.updates = make(chan SchedUpdate)
	sched.shutdown = make(chan bool)
	heap.Init(&sched.dsched)
	return sched
}

func (s *Scheduler) firstTime() time.Time {
	if s.dsched.Len() == 0 {
		return time.Now()
	}
	return s.dsched[0].SchedulerData.when
}

func (s *Scheduler) add(d *Device) {
	fmt.Printf("Adding schedule for %v\n", s.dsched)
	heap.Push(&s.dsched, d)
}

func (s *Scheduler) remove(d *Device) {
	heap.Remove(&s.dsched, d.SchedulerData.index)
}

func (s *Scheduler) runtime() {
	defer s.wg.Done()
	logrus.Print("Started runtime. Scheduler length is ", s.dsched.Len())

	for {
		if s.dsched.Len() > 0 {
			// Wait on current items
			fmt.Println("Waiting for times up")
			select {
			case update := <-s.updates:
				logrus.Printf("Adding scheduled event")
				switch update.event {
				case SchedUpdateTypeAdd:
					s.add(update.d)
				case SchedUpdateTypeRem:
					s.remove(update.d)
				}
			case <-time.After(time.Until(s.firstTime())):
				if time.Now().Sub(s.firstTime()) < activationDuration {
					dev := heap.Pop(&s.dsched)
					dev.(*Device).callback(dev.(*Device))
				}
			case <-s.shutdown:
				return
			}
		} else {
			// No items currently
			fmt.Println("Waiting for first add event")
			select {
			case update := <-s.updates:
				fmt.Printf("Adding scheduled event\n")
				switch update.event {
				case SchedUpdateTypeAdd:
					s.add(update.d)
				case SchedUpdateTypeRem:
					s.remove(update.d)
				}
			case <-s.shutdown:
				return
			}
		}

	}
}

func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.runtime()
}
func (s *Scheduler) Stop() {
	s.shutdown <- true
	s.wg.Wait()
}

func (s *Scheduler) Add(d *Device, when time.Duration, callback func(d *Device)) {
	fmt.Printf("In scheduler Add fn\n")
	d.SchedulerData.when = time.Now().Add(when)
	d.SchedulerData.callback = callback
	fmt.Printf("In scheduler Add fn: set vars\n")
	s.updates <- SchedUpdate{
		SchedUpdateTypeAdd,
		d,
	}
}

func (s *Scheduler) Cancel(d *Device) {
	s.updates <- SchedUpdate{
		SchedUpdateTypeRem,
		d,
	}
}
