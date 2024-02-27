package matchers

import (
	"reflect"
	"sync"
)

// Matcher is the interface for all matcher types.
type Matcher interface {
	TryMatch([]byte) (bool, error)
}

var (
	MatcherRegistry map[string]Matcher
	registryMutex   sync.RWMutex
)

func RegisterMatcher(typeName string, matcher Matcher) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if MatcherRegistry == nil {
		MatcherRegistry = make(map[string]Matcher)
	}
	MatcherRegistry[typeName] = matcher
}

func New(matcherType string) Matcher {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	orig, ok := MatcherRegistry[matcherType]
	if !ok {
		return nil
	}
	matcher := reflect.New(reflect.TypeOf(orig).Elem()).Interface().(Matcher)
	return matcher
}

func init() {
	RegisterMatcher("httpRequestHeaderMatcher", new(HttpRequestHeaderMatcher))
}
