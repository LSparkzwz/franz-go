// Package kadm provides a helper Kafka admin client around a *kgo.Client.
//
// This package is meant to cover the common use cases for dropping into an
// "admin" like interface for Kafka. As with any admin client, this package
// must make opinionated decisions on what to provide and what to hide. The
// underlying Kafka protocol gives more detailed information in responses, or
// allows more fine tuning in requests, but most of the time, these details are
// unnecessary.
//
// By virtue of making opinionated decisions, this package cannot satisfy every
// need for requests and responses. If you need more control than this admin
// client provides, you can use the kmsg package directly.
//
// This package contains a lot of types, but the main two types type to know
// are Client and ShardErrors. Every other type is used for inputs or outputs
// to methods on the client.
//
// The Client type is a simple small wrapper around a *kgo.Client that exists
// solely to namespace methods. The ShardErrors type is a bit more complicated.
// When issuing requests, under the hood some of these requests actually need
// to be mapped to brokers and split, issuing different pieces of the input
// request to different brokers. The *kgo.Client handles this all internally,
// but (if using RequestSharded as directed), returns each response to each of
// these split requests individually. Each response can fail or be successful.
// This package goes one step further and merges these failures into one meta
// failure, ShardErrors. Any function that returns ShardErrors is documented as
// such, and if a function returns a non-nil ShardErrors, it is possible that
// the returned data is actually valid and usable. If you care to, you can log
// / react to the partial failures and continue using the partial successful
// result. This is in contrast to other clients, which either require to to
// request individual brokers directly, or they completely hide individual
// failures, or they completely fail on any individual failure.
//
// For methods that list or describe things, this package often completely
// fails responses on auth failures. If you use a method that accepts two
// topics, one that you are authorized to and one that you are not, you will
// not receive a partial successful response. Instead, you will receive an
// AuthError. Methods that do *not* fail on auth errors are explicitly
// documented as such.
//
// Users may often find it easy to work with lists of topics or partitions.
// Rather than needing to build deeply nested maps directly, this package has a
// few helper types that are worth knowing:
//
//     TopicsList  - a slice of topics and their partitions
//     TopicsSet   - a set of topics, each containing a set of partitions
//     Partitions  - a slice of partitions
//     OffsetsList - a slice of offsets
//     Offsets     - a map of offsets
//
// These types are meant to be easy to build and use, and can be used as the
// starting point for other types.
package kadm

import (
	"sort"

	"github.com/LSparkzwz/franz-go/pkg/kgo"
)

// Client is an admin client.
//
// This is a simple wrapper around a *kgo.Client to provide helper admin methods.
type Client struct {
	cl *kgo.Client

	timeoutMillis int32
}

// NewClient returns an admin client.
func NewClient(cl *kgo.Client) *Client {
	return &Client{cl, 15000} // 15s timeout default, matching kmsg
}

// NewOptClient returns a new client directly from kgo options. This is a
// wrapper around creating a new *kgo.Client and then creating an admin client.
func NewOptClient(opts ...kgo.Opt) (*Client, error) {
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return NewClient(cl), nil
}

// Close closes the underlying *kgo.Client.
func (cl *Client) Close() {
	cl.cl.Close()
}

// SetTimeoutMillis sets the timeout to use for requests that have a timeout,
// overriding the default of 15,000 (15s).
//
// Not all requests have timeouts. Most requests are expected to return
// immediately or are expected to deliberately hang. The following requests
// have timeout fields:
//
//     Produce
//     CreateTopics
//     DeleteTopics
//     DeleteRecords
//     CreatePartitions
//     ElectLeaders
//     AlterPartitionAssignments
//     ListPartitionReassignments
//     UpdateFeatures
//
// Not all requests above are supported in the admin API.
func (cl *Client) SetTimeoutMillis(millis int32) {
	cl.timeoutMillis = millis
}

// StringPtr is a shortcut function to aid building configs for creating or
// altering topics.
func StringPtr(s string) *string {
	return &s
}

// BrokerDetail is a type alias for kgo.BrokerMetadata.
type BrokerDetail = kgo.BrokerMetadata

// BrokerDetails contains the details for many brokers.
type BrokerDetails []BrokerDetail

// NodeIDs returns the IDs of all nodes.
func (ds BrokerDetails) NodeIDs() []int32 {
	var all []int32
	for _, d := range ds {
		all = append(all, d.NodeID)
	}
	return int32s(all)
}

// Partition is a partition for a topic.
type Partition struct {
	Topic     string // Topic is the topic for this partition.
	Partition int32  // Partition is this partition's number.
}

// Offset is an offset for a topic.
type Offset struct {
	Topic       string
	Partition   int32
	At          int64  // Offset is the partition to set.
	LeaderEpoch int32  // LeaderEpoch is the broker leader epoch of the record at this offset.
	Metadata    string // Metadata, if non-empty, is used for offset commits.
}

// Partitions wraps many partitions.
type Partitions []Partition

// TopicsSet returns these partitions as TopicsSet.
func (ps Partitions) TopicsSet() TopicsSet {
	s := make(TopicsSet)
	for _, p := range ps {
		s.Add(p.Topic, p.Partition)
	}
	return s
}

// TopicsList returns these partitions as sorted TopicsList.
func (ps Partitions) TopicsList() TopicsList {
	return ps.TopicsSet().Sorted()
}

// OffsetsList wraps many offsets and is a helper for building Offsets.
type OffsetsList []Offset

// Into returns this list as the non-list Offsets. All fields in each Offset
// must be set properly.
func (l OffsetsList) Into() Offsets {
	os := make(Offsets)
	for _, o := range l {
		os.Add(o)
	}
	return os
}

// Offsets wraps many offsets and is the type used for offset functions.
type Offsets map[string]map[int32]Offset

// Lookup returns the offset at t and p and whether it exists.
func (os Offsets) Lookup(t string, p int32) (Offset, bool) {
	if len(os) == 0 {
		return Offset{}, false
	}
	ps := os[t]
	if len(ps) == 0 {
		return Offset{}, false
	}
	o, exists := ps[p]
	return o, exists
}

// Add adds an offset for a given topic/partition to this Offsets map.
//
// If the partition already exists, the offset is only added if:
//
//     - the new leader epoch is higher than the old, or
//     - the leader epochs equal, and the new offset is higher than the old
//
// If you would like to add offsets forcefully no matter what, use the Delete
// method before this.
func (os *Offsets) Add(o Offset) {
	if *os == nil {
		*os = make(map[string]map[int32]Offset)
	}
	ot := (*os)[o.Topic]
	if ot == nil {
		ot = make(map[int32]Offset)
		(*os)[o.Topic] = ot
	}

	prior, exists := ot[o.Partition]
	if !exists || prior.LeaderEpoch < o.LeaderEpoch ||
		prior.LeaderEpoch == o.LeaderEpoch && prior.At < o.At {
		ot[o.Partition] = o
	}
}

// Delete removes any offset at topic t and partition p.
func (os Offsets) Delete(t string, p int32) {
	if os == nil {
		return
	}
	ot := os[t]
	if ot == nil {
		return
	}
	delete(ot, p)
	if len(ot) == 0 {
		delete(os, t)
	}
}

// AddOffset is a helper to add an offset for a given topic and partition. The
// leader epoch field must be -1 if you do not know the leader epoch or if
// you do not have an offset yet.
func (os *Offsets) AddOffset(t string, p int32, o int64, leaderEpoch int32) {
	os.Add(Offset{
		Topic:       t,
		Partition:   p,
		At:          o,
		LeaderEpoch: leaderEpoch,
	})
}

// KeepFunc calls fn for every offset, keeping the offset if fn returns true.
func (os Offsets) KeepFunc(fn func(o Offset) bool) {
	for t, ps := range os {
		for p, o := range ps {
			if !fn(o) {
				delete(ps, p)
			}
		}
		if len(ps) == 0 {
			delete(os, t)
		}
	}
}

// DeleteFunc calls fn for every offset, deleting the offset if fn returns
// true.
func (os Offsets) DeleteFunc(fn func(o Offset) bool) {
	os.KeepFunc(func(o Offset) bool { return !fn(o) })
}

// Topics returns the set of topics and partitions currently used in these
// offsets.
func (os Offsets) TopicsSet() TopicsSet {
	s := make(TopicsSet)
	os.Each(func(o Offset) { s.Add(o.Topic, o.Partition) })
	return s
}

// Each calls fn for each offset in these offsets.
func (os Offsets) Each(fn func(Offset)) {
	for _, ps := range os {
		for _, o := range ps {
			fn(o)
		}
	}
}

// Into returns these offsets as a kgo offset map.
func (os Offsets) Into() map[string]map[int32]kgo.Offset {
	tskgo := make(map[string]map[int32]kgo.Offset)
	for t, ps := range os {
		pskgo := make(map[int32]kgo.Offset)
		for p, o := range ps {
			pskgo[p] = kgo.NewOffset().
				At(o.At).
				WithEpoch(o.LeaderEpoch)
		}
		tskgo[t] = pskgo
	}
	return tskgo
}

// Sorted returns the offsets sorted by topic and partition.
func (os Offsets) Sorted() []Offset {
	var s []Offset
	os.Each(func(o Offset) { s = append(s, o) })
	sort.Slice(s, func(i, j int) bool {
		return s[i].Topic < s[j].Topic ||
			s[i].Topic == s[j].Topic && s[i].Partition < s[j].Partition
	})
	return s
}

// OffsetsFromFetches returns Offsets for the final record in any partition in
// the fetches. This is a helper to enable committing an entire returned batch.
//
// This function looks at only the last record per partition, assuming that the
// last record is the highest offset (which is the behavior returned by kgo's
// Poll functions). The returned offsets are one past the offset contained in
// the records.
func OffsetsFromFetches(fs kgo.Fetches) Offsets {
	os := make(Offsets)
	fs.EachPartition(func(p kgo.FetchTopicPartition) {
		if len(p.Records) == 0 {
			return
		}
		r := p.Records[len(p.Records)-1]
		os.AddOffset(r.Topic, r.Partition, r.Offset+1, r.LeaderEpoch)
	})
	return os
}

// OffsetsFromRecords returns offsets for all given records, using the highest
// offset per partition. The returned offsets are one past the offset contained
// in the records.
func OffsetsFromRecords(rs ...kgo.Record) Offsets {
	os := make(Offsets)
	for _, r := range rs {
		os.AddOffset(r.Topic, r.Partition, r.Offset+1, r.LeaderEpoch)
	}
	return os
}

// TopicsSet is a set of topics and, per topic, a set of partitions.
//
// All methods provided for TopicsSet are safe to use on a nil (default) set.
type TopicsSet map[string]map[int32]struct{}

// Lookup returns whether the topic and partition exists.
func (s TopicsSet) Lookup(t string, p int32) bool {
	if len(s) == 0 {
		return false
	}
	ps := s[t]
	if len(ps) == 0 {
		return false
	}
	_, exists := ps[p]
	return exists
}

// Each calls fn for each topic / partition in the topics set.
func (s TopicsSet) Each(fn func(t string, p int32)) {
	for t, ps := range s {
		for p := range ps {
			fn(t, p)
		}
	}
}

// Add adds partitions for a topic to the topics set.
func (s *TopicsSet) Add(t string, ps ...int32) {
	if *s == nil {
		*s = make(map[string]map[int32]struct{})
	}
	existing := (*s)[t]
	if existing == nil {
		existing = make(map[int32]struct{}, len(ps))
		(*s)[t] = existing
	}
	for _, p := range ps {
		existing[p] = struct{}{}
	}
}

// Delete removes partitions from a topic from the topics set. If the topic
// ends up with no partitions, the topic is removed from the set.
func (s TopicsSet) Delete(t string, ps ...int32) {
	if s == nil || len(ps) == 0 {
		return
	}
	existing := s[t]
	if existing == nil {
		return
	}
	for _, p := range ps {
		delete(existing, p)
	}
	if len(existing) == 0 {
		delete(s, t)
	}
}

// Topics returns all topics in this set in sorted order.
func (s TopicsSet) Topics() []string {
	ts := make([]string, 0, len(s))
	for t := range s {
		ts = append(ts, t)
	}
	sort.Strings(ts)
	return ts
}

// TopicPartitions is a topic and partitions.
type TopicPartitions struct {
	Topic      string
	Partitions []int32
}

// TopicsList is a list of topics and partitions.
type TopicsList []TopicPartitions

// Each calls fn for each topic / partition in the topics list.
func (l TopicsList) Each(fn func(t string, p int32)) {
	for _, t := range l {
		for _, p := range t.Partitions {
			fn(t.Topic, p)
		}
	}
}

// Sorted returns this set as a list in topic-sorted order, with each topic
// having sorted partitions.
func (s TopicsSet) Sorted() TopicsList {
	l := make(TopicsList, 0, len(s))
	for t, ps := range s {
		tps := TopicPartitions{
			Topic:      t,
			Partitions: make([]int32, 0, len(ps)),
		}
		for p := range ps {
			tps.Partitions = append(tps.Partitions, p)
		}
		tps.Partitions = int32s(tps.Partitions)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Topic < l[j].Topic })
	return l
}

// IntoSet returns this list as a set.
func (l TopicsList) IntoSet() TopicsSet {
	s := make(TopicsSet)
	for _, t := range l {
		s.Add(t.Topic, t.Partitions...)
	}
	return s
}
