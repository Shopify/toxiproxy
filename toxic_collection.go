package toxiproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/Shopify/toxiproxy/stream"
	"github.com/Shopify/toxiproxy/toxics"
)

// ToxicCollection contains a list of toxics that are chained together. Each proxy
// has its own collection. A hidden noop toxic is always maintained at the beginning
// of each chain so toxics have a method of pausing incoming data (by interrupting
// the preceding toxic).
type ToxicCollection struct {
	sync.Mutex

	noop   *toxics.ToxicWrapper
	proxy  *Proxy
	chain  [][]*toxics.ToxicWrapper
	links  map[string]*ToxicLink
}

func NewToxicCollection(proxy *Proxy) *ToxicCollection {
	collection := &ToxicCollection{
		noop: &toxics.ToxicWrapper{
			Toxic: new(toxics.NoopToxic),
			Type:  "noop",
		},
		proxy:  proxy,
		chain:  make([][]*toxics.ToxicWrapper, stream.NumDirections),
		links:  make(map[string]*ToxicLink),
	}
	for dir := range collection.chain {
		collection.chain[dir] = make([]*toxics.ToxicWrapper, 1, toxics.Count()+1)
		collection.chain[dir][0] = collection.noop
	}
	return collection
}

func (c *ToxicCollection) ResetToxics() {
	c.Lock()
	defer c.Unlock()

	// Remove all but the first noop toxic
	for dir := range c.chain {
		for len(c.chain[dir]) > 1 {
			c.chainRemoveToxic(c.chain[dir][1])
		}
	}
}

func (c *ToxicCollection) GetToxic(name string) *toxics.ToxicWrapper {
	c.Lock()
	defer c.Unlock()

	return c.findToxicByName(name)
}

func (c *ToxicCollection) GetToxicArray() []toxics.Toxic {
	c.Lock()
	defer c.Unlock()

	result := make([]toxics.Toxic, 0)
	for dir := range c.chain {
		for i, toxic := range c.chain[dir] {
			if i == 0 {
				// Skip the first noop toxic, it should not be visible
				continue
			}
			result = append(result, toxic)
		}
	}
	return result
}

func (c *ToxicCollection) AddToxicJson(data io.Reader) (*toxics.ToxicWrapper, error) {
	c.Lock()
	defer c.Unlock()

	var buffer bytes.Buffer

	// Default to a downstream toxic with a toxicity of 1.
	wrapper := &toxics.ToxicWrapper{
		Stream:   "downstream",
		Toxicity: 1.0,
		Toxic:    new(toxics.NoopToxic),
	}

	err := json.NewDecoder(io.TeeReader(data, &buffer)).Decode(wrapper)
	if err != nil {
		return nil, joinError(err, ErrBadRequestBody)
	}

	switch strings.ToLower(wrapper.Stream) {
	case "downstream":
		wrapper.Direction = stream.Downstream
	case "upstream":
		wrapper.Direction = stream.Upstream
	default:
		return nil, ErrInvalidStream
	}
	if wrapper.Name == "" {
		wrapper.Name = fmt.Sprintf("%s_%s", wrapper.Type, wrapper.Stream)
	}

	if toxics.New(wrapper) == nil {
		return nil, ErrInvalidToxicType
	}

	found := c.findToxicByName(wrapper.Name)
	if found != nil {
		return nil, ErrToxicAlreadyExists
	}

	// Parse attributes because we now know the toxics type.
	attrs := &struct {
		Attributes interface{} `json:"attributes"`
	}{
		wrapper.Toxic,
	}
	err = json.NewDecoder(&buffer).Decode(attrs)
	if err != nil {
		return nil, joinError(err, ErrBadRequestBody)
	}

	c.chainAddToxic(wrapper)
	return wrapper, nil
}

func (c *ToxicCollection) UpdateToxicJson(name string, data io.Reader) (*toxics.ToxicWrapper, error) {
	c.Lock()
	defer c.Unlock()

	toxic := c.findToxicByName(name)
	if toxic != nil {
		attrs := &struct {
			Attributes interface{} `json:"attributes"`
			Toxicity   float32     `json:"toxicity"`
		}{
			toxic.Toxic,
			toxic.Toxicity,
		}
		err := json.NewDecoder(data).Decode(attrs)
		if err != nil {
			return nil, joinError(err, ErrBadRequestBody)
		}
		toxic.Toxicity = attrs.Toxicity

		c.chainUpdateToxic(toxic)
		return toxic, nil
	}
	return nil, ErrToxicNotFound
}

func (c *ToxicCollection) RemoveToxic(name string) error {
	c.Lock()
	defer c.Unlock()

	toxic := c.findToxicByName(name)
	if toxic != nil {
		c.chainRemoveToxic(toxic)
		return nil
	}
	return ErrToxicNotFound
}

func (c *ToxicCollection) StartLink(name string, input io.Reader, output io.WriteCloser, direction stream.Direction) {
	c.Lock()
	defer c.Unlock()

	link := NewToxicLink(c.proxy, c, direction)
	link.Start(name, input, output)
	c.links[name] = link
}

func (c *ToxicCollection) RemoveLink(name string) {
	c.Lock()
	defer c.Unlock()
	delete(c.links, name)
}

// All following functions assume the lock is already grabbed
func (c *ToxicCollection) findToxicByName(name string) *toxics.ToxicWrapper {
	for dir := range c.chain {
		for i, toxic := range c.chain[dir] {
			if i == 0 {
				// Skip the first noop toxic, it has no name
				continue
			}
			if toxic.Name == name {
				return toxic
			}
		}
	}
	return nil
}

func (c *ToxicCollection) chainAddToxic(toxic *toxics.ToxicWrapper) {
	dir := toxic.Direction
	toxic.Index = len(c.chain[dir])
	c.chain[dir] = append(c.chain[dir], toxic)

	// Asynchronously add the toxic to each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		if link.direction == dir {
			group.Add(1)
			go func(link *ToxicLink) {
				defer group.Done()
				link.AddToxic(toxic)
			}(link)
		}
	}
	group.Wait()
}

func (c *ToxicCollection) chainUpdateToxic(toxic *toxics.ToxicWrapper) {
	c.chain[toxic.Direction][toxic.Index] = toxic

	// Asynchronously update the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		if link.direction == toxic.Direction {
			group.Add(1)
			go func(link *ToxicLink) {
				defer group.Done()
				link.UpdateToxic(toxic)
			}(link)
		}
	}
	group.Wait()
}

func (c *ToxicCollection) chainRemoveToxic(toxic *toxics.ToxicWrapper) {
	dir := toxic.Direction
	c.chain[dir] = append(c.chain[dir][:toxic.Index], c.chain[dir][toxic.Index+1:]...)
	for i := toxic.Index; i < len(c.chain[dir]); i++ {
		c.chain[dir][i].Index = i
	}

	// Asynchronously remove the toxic from each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		if link.direction == dir {
			group.Add(1)
			go func(link *ToxicLink) {
				defer group.Done()
				link.RemoveToxic(toxic)
			}(link)
		}
	}
	group.Wait()

	toxic.Index = -1
}
