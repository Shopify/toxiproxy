package metrics

import (
	"container/list"
	"fmt"
	"go/types"
	"time"
)

var timeToKeep time.Duration
var maxNumOfEvents = 1 // default

var eventList *list.List
var lastId int

var incomingEventsChannel = make(chan Event, 100)
var eventsRequests = make(chan int)
var eventsResponse = make(chan EventsAndToken)
var metricsRequests = make(chan types.Nil)
var metricsResponse = make(chan map[string]uint64)

type Event struct {
	token     int
	Client    string    `json:"client"`
	Upstream  string    `json:"target"`
	Time      time.Time `json:"timestamp"`
	ProxyName string    `json:"proxyName"`
	EventType string    `json:"eventType"`
}

func (e Event) String() string {
	return fmt.Sprintf("Event {token: %d, proxy: %s}", e.token, e.ProxyName)
}

type EventsAndToken struct {
	Data  []Event `json:"data"`
	Token int     `json:"token,string"`
}

var messages map[string]uint64

// (re) initializes data. Except for first initialization, intended to be used only in tests.
// Not "thread-safe".
func resetData() {
	eventList = list.New()
	messages = make(map[string]uint64)
	lastId = 0
}

func init() {
	resetData()
	go metricsRoutine()
}

// Init the settings. This should be called before registering events
func InitSettings(maxTimeToKeep string, maxEvents int) {
	var err error
	timeToKeep, err = time.ParseDuration("-" + maxTimeToKeep)

	if err != nil {
		panic("Max metrics time is not a duration: " + maxTimeToKeep)
	}

	maxNumOfEvents = maxEvents
}

// The main requests handling routine. The routine makes sure that only one request
// (either external or RegisterEvent calls) is handled simultaneously.
func metricsRoutine() {
	for {
		select {
		case event := <-incomingEventsChannel:
			registerEvent(event)
		case <-metricsRequests:
			metricsResponse <- getMetrics()
		case token := <-eventsRequests:
			eventsResponse <- getMetricEventsStartingFrom(token)
		}
	}
}

// Report an events that should be collected by metrics.
//
// Note: this is an asynchronous operation, so calls to Get methods in this module
// might not reflect an event registration immediately
func RegisterEvent(event Event) {
	incomingEventsChannel <- event
}

func registerEvent(event Event) {
	messages[event.ProxyName] += 1
	event.token = lastId
	lastId++

	eventList.PushBack(event)

	// Cleanup stale events - more than max number or max age:

	for eventList.Len() > maxNumOfEvents {
		eventList.Remove(eventList.Front())
	}

	startTime := time.Now().Add(timeToKeep)

	for eventList.Len() > 0 && eventList.Front().Value.(Event).Time.Before(startTime) {
		eventList.Remove(eventList.Front())
	}
}

// Get general metrics (proxy name -> num of events related to the proxy) since launch
func GetMetrics() map[string]uint64 {
	metricsRequests <- types.Nil{}
	return <-metricsResponse
}

func getMetrics() map[string]uint64 {
	return messages
}

// Get all available events in history. The result includes a token tha can be used
// to call GetMetricEventsStartingFrom in order get only unread events.
func GetMetricEvents() EventsAndToken {
	return GetMetricEventsStartingFrom(-1)
}

// Get all unread events in history. The token parameter should be a result of a previous call.
// The result includes a token tha can be used in the following call in order get only unread events.
func GetMetricEventsStartingFrom(token int) EventsAndToken {
	eventsRequests <- token
	return <-eventsResponse
}

func getMetricEventsStartingFrom(token int) EventsAndToken {
	var e *list.Element
	var skippedCount = 0

	// skip seen elements
	for e = eventList.Front(); e != nil && e.Value.(Event).token <= token; e = e.Next() {
		skippedCount++
	}
	var result []Event
	for ; e != nil; e = e.Next() {
		result = append(result, e.Value.(Event))
	}

	if result == nil {
		result = []Event{}
	}
	var returnedToken int
	if eventList.Len() > 0 {
		returnedToken = eventList.Back().Value.(Event).token
	} else { // no events exist
		returnedToken = -1
	}

	return EventsAndToken{result, returnedToken}
}
