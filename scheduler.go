package main

import (
	"container/heap"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultSchedulingCapacity = 10
	periodicStats             = false
	periodicStatsDuration     = time.Second * time.Duration(60)
	activationDuration        = time.Millisecond * time.Duration(1)
)

type Scheduler struct {
	log      *logrus.Entry
	allitems map[Scheduable]*SchedulerData
	dsched   ScheduleHeap
	updates  chan SchedUpdate
	shutdown chan bool
	stats    chan bool
	wg       sync.WaitGroup
}

func NewScheduler(log *logrus.Logger) *Scheduler {
	sched := new(Scheduler)
	sched.log = log.WithField("module", "scheduler")
	sched.allitems = make(map[Scheduable]*SchedulerData)
	sched.dsched = make(ScheduleHeap, 0, defaultSchedulingCapacity)
	sched.updates = make(chan SchedUpdate)
	sched.shutdown = make(chan bool)
	sched.stats = make(chan bool)
	heap.Init(&sched.dsched)
	return sched
}

func (s *Scheduler) firstTime() time.Time {
	if s.dsched.Len() == 0 {
		return time.Now()
	}
	return s.dsched[0].when
}

func (s *Scheduler) add(d Scheduable, when time.Time) {
	s.log.Debugf("Request to add event for %v (%v)", when, when.Sub(time.Now()))
	if _, ok := s.allitems[d]; !ok {
		sdata := new(SchedulerData)
		sdata.when = when
		sdata.object = d
		heap.Push(&s.dsched, sdata)
		s.allitems[d] = sdata
	}
}

func (s *Scheduler) remove(d Scheduable) {
	s.log.Debugf("Request to remove event")
	if sdata, ok := s.allitems[d]; ok {
		heap.Remove(&s.dsched, sdata.index)
		delete(s.allitems, d)
	}
}

func (s *Scheduler) printStats() {
	if periodicStats {
		s.log.Infof("Heap Length is %d", len(s.dsched))
		s.log.Infof("Map Length is %d", len(s.allitems))
	} else {
		s.log.Debugf("Heap Length is %d", len(s.dsched))
		s.log.Debugf("Map Length is %d", len(s.allitems))
	}
}

func (s *Scheduler) runtime() {
	defer s.wg.Done()

	s.log.Info("Runtime started")

	for {
		s.printStats()
		if s.dsched.Len() > 0 {
			// Wait on current items
			s.log.Debug("Waiting for event times")
			select {
			case update := <-s.updates:
				switch update.event {
				case SchedUpdateTypeAdd:
					s.add(update.d, update.when)
				case SchedUpdateTypeRem:
					s.remove(update.d)
				}
			case <-time.After(time.Until(s.firstTime())):
				if time.Now().Sub(s.firstTime()) < activationDuration {
					s.log.Debug("Acting on scheduler event")
					sdata := heap.Pop(&s.dsched).(*SchedulerData)
					delete(s.allitems, sdata.object)
					sdata.object.SchedulerAction()
				}
			case <-s.shutdown:
				return
			case <-s.stats:
			}
		} else {
			// No items currently
			s.log.Debug("Waiting for add or remove event")
			select {
			case update := <-s.updates:
				switch update.event {
				case SchedUpdateTypeAdd:
					s.add(update.d, update.when)
				case SchedUpdateTypeRem:
					s.remove(update.d)
				}
			case <-s.shutdown:
				return
			case <-s.stats:
			}
		}

	}
}

func (s *Scheduler) monitor() {
	defer s.wg.Done()
	s.log.Info("Monitor started")

	if periodicStats {
		for {
			select {
			case <-s.shutdown:
				return
			case <-time.After(periodicStatsDuration):
				s.stats <- true
			}
		}
	} else {
		select {
		case <-s.shutdown:
			return
		}

	}
}

func (s *Scheduler) Start() {
	s.wg.Add(2)
	go s.runtime()
	go s.monitor()
}

func (s *Scheduler) Stop() {
	s.shutdown <- true
	s.shutdown <- true
	s.wg.Wait()
}

func (s *Scheduler) Add(d Scheduable, when time.Duration) {
	s.updates <- SchedUpdate{
		event: SchedUpdateTypeAdd,
		d:     d,
		when:  time.Now().Add(when),
	}
}

func (s *Scheduler) Cancel(d Scheduable) {
	s.updates <- SchedUpdate{
		event: SchedUpdateTypeRem,
		d:     d,
	}
}

func (s *Scheduler) QueueLen() int {
	return len(s.dsched)
}
