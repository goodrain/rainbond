// This library implements a cron spec parser and runner.  See the README for
// more details.
package cron

import (
	"fmt"
	"log"
	"reflect"
	"runtime"
	"sort"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	entries  []*Entry
	indexes  map[string]int
	stop     chan struct{}
	add      chan *Entry
	del      chan string
	snapshot chan []*Entry
	running  bool
	ErrorLog *log.Logger
	location *time.Location
}

// Job is an interface for submitted cron jobs.
type Job interface {
	GetID() string
	Run()
}

// The Schedule describes a job's duty cycle.
type Schedule interface {
	// Return the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// Entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	// The ID is unique for Entry
	ID string

	// The schedule on which this job should be run.
	Schedule Schedule

	// The next time the job will run. This is the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next time.Time

	// The last time this job was run. This is the zero time if the job has never
	// been run.
	Prev time.Time

	// The Job to run.
	Job Job
}

// byTime is a wrapper for sorting the entry array by time
// (with zero time at the end).
type byTime []*Entry

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}

// New returns a new Cron job runner, in the Local time zone.
func New() *Cron {
	return NewWithLocation(time.Now().Location())
}

// NewWithLocation returns a new Cron job runner.
func NewWithLocation(location *time.Location) *Cron {
	return &Cron{
		entries:  nil,
		indexes:  make(map[string]int),
		add:      make(chan *Entry),
		del:      make(chan string),
		stop:     make(chan struct{}),
		snapshot: make(chan []*Entry),
		running:  false,
		ErrorLog: nil,
		location: location,
	}
}

// A wrapper that turns a func() into a cron.Job
type FuncJob func()

func (f FuncJob) GetID() string {
	return fmt.Sprintf("pointer[%v]", reflect.ValueOf(f).Pointer())
}
func (f FuncJob) Run() { f() }

// AddFunc adds or updates a func to the Cron to be run on the given schedule.
func (c *Cron) AddFunc(spec string, cmd func()) error {
	return c.AddJob(spec, FuncJob(cmd))
}

// AddJob adds or updates a Job to the Cron to be run on the given schedule.
func (c *Cron) AddJob(spec string, cmd Job) error {
	schedule, err := Parse(spec)
	if err != nil {
		return err
	}
	c.Schedule(schedule, cmd)
	return nil
}

// Schedule adds or updates a Job to the Cron to be run on the given schedule.
func (c *Cron) Schedule(schedule Schedule, cmd Job) {
	entry := &Entry{
		ID:       cmd.GetID(),
		Schedule: schedule,
		Job:      cmd,
	}
	if !c.running {
		if index, ok := c.indexes[entry.ID]; ok {
			c.entries[index] = entry
			return
		}
		c.entries, c.indexes[entry.ID] = append(c.entries, entry), len(c.entries)
		return
	}

	c.add <- entry
}

// DelFunc deletes a Job from the Cron.
func (c *Cron) DelFunc(cmd func()) {
	c.DelJob(FuncJob(cmd))
}

// DelJob deletes a Job from the Cron.
func (c *Cron) DelJob(cmd Job) {
	index, ok := c.indexes[cmd.GetID()]
	if !ok {
		return
	}

	if c.running {
		c.del <- cmd.GetID()
		return
	}

	c.entries = append(c.entries[:index], c.entries[index+1:]...)
	delete(c.indexes, cmd.GetID())
	return
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []*Entry {
	if c.running {
		c.snapshot <- nil
		x := <-c.snapshot
		return x
	}
	return c.entrySnapshot()
}

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
}

// Start the cron scheduler in its own go-routine, or no-op if already started.
func (c *Cron) Start() {
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

func (c *Cron) runWithRecovery(j Job) {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			c.logf("cron: panic running job: %v\n%s", r, buf)
		}
	}()
	j.Run()
}

// Rebuild the indexes
func (c *Cron) reIndex() {
	for i, count := 0, len(c.entries); i < count; i++ {
		c.indexes[c.entries[i].ID] = i
	}
}

// Run the scheduler.. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
	// Figure out the next activation times for each entry.
	now := time.Now().In(c.location)
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
	}

	timer := time.NewTimer(time.Minute)
	for {
		// Determine the next entry to run.
		sort.Sort(byTime(c.entries))
		c.reIndex()

		var effective time.Time
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			effective = now.AddDate(10, 0, 0)
		} else {
			effective = c.entries[0].Next
		}

		timer.Reset(effective.Sub(now))
		select {
		case now = <-timer.C:
			now = now.In(c.location)
			// Run every entry whose next time was this effective time.
			for _, e := range c.entries {
				if e.Next != effective {
					break
				}
				go c.runWithRecovery(e.Job)
				e.Prev = e.Next
				e.Next = e.Schedule.Next(now)
			}
			continue

		case newEntry := <-c.add:
			if index, ok := c.indexes[newEntry.ID]; ok {
				c.entries[index] = newEntry
			} else {
				c.entries, c.indexes[newEntry.ID] = append(c.entries, newEntry), len(c.entries)
			}
			newEntry.Next = newEntry.Schedule.Next(time.Now().In(c.location))

		case id := <-c.del:
			index, ok := c.indexes[id]
			if !ok {
				continue
			}

			c.entries = append(c.entries[:index], c.entries[index+1:]...)
			delete(c.indexes, id)

		case <-c.snapshot:
			c.snapshot <- c.entrySnapshot()

		case <-c.stop:
			timer.Stop()
			return
		}

		// 'now' should be updated after newEntry and snapshot cases.
		now = time.Now().In(c.location)
	}
}

// Logs an error to stderr or to the configured error log
func (c *Cron) logf(format string, args ...interface{}) {
	if c.ErrorLog != nil {
		c.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Stop stops the cron scheduler if it is running; otherwise it does nothing.
func (c *Cron) Stop() {
	if !c.running {
		return
	}
	c.stop <- struct{}{}
	c.running = false
}

// entrySnapshot returns a copy of the current cron entry list.
func (c *Cron) entrySnapshot() []*Entry {
	entries := []*Entry{}
	for _, e := range c.entries {
		entries = append(entries, &Entry{
			Schedule: e.Schedule,
			Next:     e.Next,
			Prev:     e.Prev,
			Job:      e.Job,
		})
	}
	return entries
}
