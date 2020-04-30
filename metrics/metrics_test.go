package metrics

import (
	"testing"
	"time"
)

var duration3m, _ = time.ParseDuration("3m")

func eventEquals(e1 Event, e2 Event) bool {
	return e1.ProxyName == e2.ProxyName
}

func TestRegisterEvent_Simple(t *testing.T) {
	resetData()
	InitSettings("10m", 1)
	event := Event{Time: time.Now(), ProxyName: "e"}

	registerEvent(event)

	if eventList.Len() != 1 || !eventEquals(eventList.Front().Value.(Event), event) {
		t.Error("eventList does not contain exactly the one register event")
	}
}

func TestRegisterEvent_AllowOne_AddTwo_LastOneRemains(t *testing.T) {
	resetData()
	InitSettings("10m", 1)
	event1 := Event{Time: time.Now(), ProxyName: "e1"}
	event2 := Event{Time: time.Now(), ProxyName: "e2"}

	registerEvent(event1)
	registerEvent(event2)

	if eventList.Len() != 1 || !eventEquals(eventList.Front().Value.(Event), event2) {
		t.Error("eventList does not contain exactly the last register event, len =", eventList.Len())
	}
}

func TestRegisterEvent_AllowTwoMinutes_AddObsolete_EmptyList(t *testing.T) {
	resetData()
	InitSettings("2m", 10)

	event1 := Event{Time: time.Now().Add(-duration3m), ProxyName: "e1"}

	registerEvent(event1)

	if eventList.Len() != 0 {
		t.Error("eventList should be empty, len =", eventList.Len())
	}
}

func TestRegisterEvent_AllowTwoMinutes_AddObsoleteAndNew_OnlyNewRemains(t *testing.T) {
	resetData()
	InitSettings("2m", 10)

	event1 := Event{Time: time.Now().Add(-duration3m), ProxyName: "e1"}
	event2 := Event{Time: time.Now(), ProxyName: "e2"}

	registerEvent(event1)
	registerEvent(event2)

	if eventList.Len() != 1 || !eventEquals(eventList.Front().Value.(Event), event2) {
		t.Error("eventList does not contain exactly the one registered event, len =", eventList.Len())
	}
}

func TestGetMetricEvents_ReturnsAllEvents(t *testing.T) {
	resetData()
	InitSettings("2m", 10)

	event1 := Event{Time: time.Now(), ProxyName: "e1"}
	event2 := Event{Time: time.Now(), ProxyName: "e2"}
	event3 := Event{Time: time.Now(), ProxyName: "e3"}
	registerEvent(event1)
	registerEvent(event2)
	registerEvent(event3)

	events := GetMetricEvents()

	if len(events.Data) != 3 ||
		!eventEquals(events.Data[0], event1) ||
		!eventEquals(events.Data[1], event2) ||
		!eventEquals(events.Data[2], event3) {
		t.Error("the events do not match exactly the registered events, events =", events.Data)
	}
}

func TestGetMetricEventsStartingFrom_ReturnsAllEventsAfterLocation(t *testing.T) {
	// Arrange
	resetData()
	InitSettings("2m", 10)

	event1 := Event{Time: time.Now(), ProxyName: "e1"}
	event2 := Event{Time: time.Now(), ProxyName: "e2"}
	event3 := Event{Time: time.Now(), ProxyName: "e3"}
	event4 := Event{Time: time.Now(), ProxyName: "e4"}

	registerEvent(event1)
	registerEvent(event2)

	events := GetMetricEvents()

	registerEvent(event3)
	registerEvent(event4)

	// Act
	events = GetMetricEventsStartingFrom(events.Location)

	// Assert
	if len(events.Data) != 2 ||
		!eventEquals(events.Data[0], event3) ||
		!eventEquals(events.Data[1], event4) {
		t.Error("the events do not match exactly the new registered events, events =", events.Data)
	}
}

func TestGetMetricEventsStartingFrom_NoNewEvents_EmptyList(t *testing.T) {
	// Arrange
	resetData()
	InitSettings("2m", 10)

	event1 := Event{Time: time.Now(), ProxyName: "e1"}
	event2 := Event{Time: time.Now(), ProxyName: "e2"}

	registerEvent(event1)
	registerEvent(event2)

	events := GetMetricEvents()

	// Act
	events = GetMetricEventsStartingFrom(events.Location)

	// Assert
	if len(events.Data) != 0 {
		t.Error("the event list is not empty, events =", events.Data)
	}
}

func TestGetMetrics_Simple(t *testing.T) {
	resetData()
	InitSettings("2m", 10)

	registerEvent(Event{Time: time.Now(), ProxyName: "proxy1"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy2"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy3"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy1"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy1"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy3"})

	metrics := GetMetrics()

	if len(metrics) != 3 ||
		metrics["proxy1"] != 3 ||
		metrics["proxy2"] != 1 ||
		metrics["proxy3"] != 2 {
		t.Error("the metrics do not match exactly the registered events, metrics =", metrics)
	}
}

func TestGetMetrics_SomeEventsExpired_ExpiredEventsStillCount(t *testing.T) {
	resetData()
	InitSettings("2m", 3)

	registerEvent(Event{Time: time.Now().Add(-duration3m), ProxyName: "proxy1"})
	registerEvent(Event{Time: time.Now().Add(-duration3m), ProxyName: "proxy2"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy3"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy1"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy1"})
	registerEvent(Event{Time: time.Now(), ProxyName: "proxy3"})

	metrics := GetMetrics()

	if len(metrics) != 3 ||
		metrics["proxy1"] != 3 ||
		metrics["proxy2"] != 1 ||
		metrics["proxy3"] != 2 {
		t.Error("the metrics do not match exactly the registered events, metrics =", metrics)
	}
}

func TestGetMetrics_NoEvents_EmptyMetrics(t *testing.T) {
	resetData()
	InitSettings("2m", 3)

	metrics := GetMetrics()

	if len(metrics) != 0 {
		t.Error("the metrics are not empty, metrics =", metrics)
	}
}
