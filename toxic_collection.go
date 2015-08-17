package toxiproxy

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	"github.com/Shopify/toxiproxy/toxics"
)

type ToxicCollection struct {
	sync.Mutex

	noop   *toxics.ToxicWrapper
	proxy  *Proxy
	chain  []*toxics.ToxicWrapper
	toxics []*toxics.ToxicWrapper
	links  map[string]*ToxicLink
}

func NewToxicCollection(proxy *Proxy) *ToxicCollection {
	collection := &ToxicCollection{
		noop: &toxics.ToxicWrapper{
			Toxic: new(toxics.NoopToxic),
			Type:  "noop",
		},
		proxy:  proxy,
		chain:  make([]*toxics.ToxicWrapper, 1, toxics.Count()+1),
		toxics: make([]*toxics.ToxicWrapper, 0, toxics.Count()),
		links:  make(map[string]*ToxicLink),
	}
	collection.chain[0] = collection.noop
	return collection
}

func (c *ToxicCollection) ResetToxics() {
	c.Lock()
	defer c.Unlock()

	for _, toxic := range c.toxics {
		// TODO do this in bulk
		c.chainRemoveToxic(toxic)
	}
	c.toxics = c.toxics[:0]
}

func (c *ToxicCollection) GetToxic(name string) toxics.Toxic {
	c.Lock()
	defer c.Unlock()

	for _, toxic := range c.toxics {
		if toxic.Name == name {
			return toxic
		}
	}
	return nil
}

func (c *ToxicCollection) GetToxicMap() map[string]toxics.Toxic {
	c.Lock()
	defer c.Unlock()

	result := make(map[string]toxics.Toxic)
	for _, toxic := range c.toxics {
		result[toxic.Name] = toxic
	}
	return result
}

func (c *ToxicCollection) AddToxicJson(data io.Reader) (toxics.Toxic, error) {
	c.Lock()
	defer c.Unlock()

	var buffer bytes.Buffer

	wrapper := new(toxics.ToxicWrapper)
	err := json.NewDecoder(io.TeeReader(data, &buffer)).Decode(wrapper)
	if err != nil {
		return nil, joinError(err, ErrBadRequestBody)
	}
	if wrapper.Name == "" {
		wrapper.Name = wrapper.Type
	}

	if toxics.New(wrapper) == nil {
		return nil, ErrInvalidToxicType
	}

	for _, toxic := range c.toxics {
		if toxic.Name == wrapper.Name {
			return nil, ErrToxicAlreadyExists
		}
	}
	err = json.NewDecoder(&buffer).Decode(wrapper.Toxic)
	if err != nil {
		return nil, joinError(err, ErrBadRequestBody)
	}

	c.toxics = append(c.toxics, wrapper)
	c.chainAddToxic(wrapper)
	return wrapper.Toxic, nil
}

func (c *ToxicCollection) UpdateToxicJson(name string, data io.Reader) (toxics.Toxic, error) {
	c.Lock()
	defer c.Unlock()

	for _, toxic := range c.toxics {
		if toxic.Name == name {
			err := json.NewDecoder(data).Decode(toxic.Toxic)
			if err != nil {
				return nil, joinError(err, ErrBadRequestBody)
			}

			c.chainUpdateToxic(toxic)
			return toxic.Toxic, nil
		}
	}
	return nil, ErrToxicNotFound
}

func (c *ToxicCollection) RemoveToxic(name string) error {
	c.Lock()
	defer c.Unlock()

	for index, toxic := range c.toxics {
		if toxic.Name == name {
			c.toxics = append(c.toxics[:index], c.toxics[index+1:]...)

			c.chainRemoveToxic(toxic)
			return nil
		}
	}
	return ErrToxicNotFound
}

func (c *ToxicCollection) StartLink(name string, input io.Reader, output io.WriteCloser) {
	c.Lock()
	defer c.Unlock()

	link := NewToxicLink(c.proxy, c)
	link.Start(name, input, output)
	c.links[name] = link
}

func (c *ToxicCollection) RemoveLink(name string) {
	c.Lock()
	defer c.Unlock()
	delete(c.links, name)
}

// All following functions assume the lock is already grabbed
func (c *ToxicCollection) chainAddToxic(toxic *toxics.ToxicWrapper) {
	toxic.Index = len(c.chain)
	c.chain = append(c.chain, toxic)

	// Asynchronously add the toxic to each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		group.Add(1)
		go func(link *ToxicLink) {
			defer group.Done()
			link.AddToxic(toxic)
		}(link)
	}
	group.Wait()
}

func (c *ToxicCollection) chainUpdateToxic(toxic *toxics.ToxicWrapper) {
	c.chain[toxic.Index] = toxic

	// Asynchronously update the toxic in each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		group.Add(1)
		go func(link *ToxicLink) {
			defer group.Done()
			link.UpdateToxic(toxic)
		}(link)
	}
	group.Wait()
}

func (c *ToxicCollection) chainRemoveToxic(toxic *toxics.ToxicWrapper) {
	c.chain = append(c.chain[:toxic.Index], c.chain[toxic.Index+1:]...)
	for i := toxic.Index; i < len(c.chain); i++ {
		c.chain[i].Index = i
	}

	// Asynchronously remove the toxic from each link
	group := sync.WaitGroup{}
	for _, link := range c.links {
		group.Add(1)
		go func(link *ToxicLink) {
			defer group.Done()
			link.RemoveToxic(toxic)
		}(link)
	}
	group.Wait()

	toxic.Index = -1
}
