# Creating custom toxics

Creating a toxic is done by implementing the `Toxic` interface:

```go
type Toxic interface {
    Pipe(*toxics.ToxicStub)
}
```

The `Pipe()` function defines how data flows through the toxic, and is passed a
`ToxicStub` to operate on. A `ToxicStub` stores the input and output channels for
the toxic, as well as an interrupt channel that is used to pause operation of the
toxic.

The input and output channels in a `ToxicStub` send and receive `StreamChunk` structs,
which are similar to network packets. A `StreamChunk` contains a `byte[]` of stream
data, and a timestamp of when Toxiproxy received the data from the client or server.
This is used instead of just a plain `byte[]` so that toxics like latency can find out
how long a chunk of data has been waiting in the proxy.

Toxics are registered in an `init()` function so that they can be used by the server:
```go
func init() {
    toxics.Register("toxic_name", new(ExampleToxic))
}
```

In order to use your own toxics, you will need to compile your own binary. This can be
done by copying [toxiproxy.go](https://github.com/Shopify/toxiproxy/blob/master/cmd/toxiproxy.go)
into a new project and registering your toxic with the server. This will allow you to add toxics
without having to make a full fork of the project. If you think your toxics will be useful
to others, contribute them back with a Pull Request.

An example project for building a separate binary can be found here:  
https://github.com/xthexder/toxic-example

## A basic toxic

The most basic implementation of a toxic is the [noop toxic](https://github.com/Shopify/toxiproxy/blob/master/toxics/noop.go),
which just passes data through without any modifications.

```go
type NoopToxic struct{}

func (t *NoopToxic) Pipe(stub *toxics.ToxicStub) {
    for {
        select {
        case <-stub.Interrupt:
            return
        case c := <-stub.Input:
            if c == nil {
                stub.Close()
                return
            }
            stub.Output <- c
        }
    }
}
```

The above code reads from `stub.Input` in a loop, and passes the `StreamChunk` along to
`stub.Output`. Since reading from `stub.Input` will block until a chunk is available,
we need to check for interrupts as the same time.

Toxics will be interrupted whenever they are being updated, or possibly removed. This can
happen at any point within the `Pipe()` function, so all blocking operations (including sleep),
should be interruptible. When an interrupt is received, the toxic should return from the `Pipe()`
function after it has written any "in-flight" data back to `stub.Output`. It is important that
all data read from `stub.Input` is passed along to `stub.Output`, otherwise the stream will be
missing bytes and become corrupted.

When an `end of stream` is reached, `stub.Input` will return a `nil` chunk. Whenever a
nil chunk is returned, the toxic should call `Close()` on the stub, and return from `Pipe()`.

## Toxic configuration

Toxic configuration information can be stored in the toxic struct. The toxic will be json
encoded and decoded by the api, so all public fields will be api accessible.

An example of a toxic that uses configuration values is the [latency toxic](https://github.com/Shopify/toxiproxy/blob/master/toxics/latency.go)

```go
type LatencyToxic struct {
    Latency int64 `json:"latency"`
    Jitter  int64 `json:"jitter"`
}
```

These fields can be used inside the `Pipe()` function, but generally should not be written
to from the toxic. A separate instance of the toxic exists for each connection through the
proxy, and may be replaced when updated by the api. If state is required in your toxic, it
is better to use a local variable at the top of `Pipe()`, since struct fields are not
guaranteed to be persisted across interrupts.

## Toxic buffering

By default, toxics are not buffered. This means that writes to `stub.Output` will block until
either the endpoint or another toxic reads it. Since toxics are chained together, this means
not reading from `stub.Input` will block other toxics (and endpoint writes) from operating.
If this is not behavior you want your toxic to have, you can specify a buffer size for your
toxic's input. The [latency toxic](https://github.com/Shopify/toxiproxy/blob/master/toxics/latency.go)
uses this in order to prevent added latency from limiting the proxy bandwidth.

Specifying a buffer size is done by implementing the `BufferedToxic` interface, which adds the
`GetBufferSize()` function:

```go
func (t *LatencyToxic) GetBufferSize() int {
    return 1024
}
```

The unit used by `GetBufferSize()` is `StreamChunk`s. Chunks are generally anywhere from
1 byte, up to 32KB, so keep this in mind when thinking about how much buffering you need,
and how much memory you are comfortable with using.

## Stateful toxics

If a toxic needs to store extra information for a connection such as the number of bytes
transferred (See the [limit_data toxic](https://github.com/Shopify/toxiproxy/blob/master/toxics/limit_data.go)),
a state object can be created by implementing the `StatefulToxic` interface. This interface
defines the `NewState()` function that can create a new state object with default values set.

```go
func (t *ExampleToxic) NewState() interface{} {
    return &ExampleToxicState{
        BytesRemaining: t.BytesAllowed,
        SomeOtherState: true,
    }
}
```

When a stateful toxic is created, the state object will be stored on the `ToxicStub` and
can be accessed from `toxic.Pipe()`:

```go
state := stub.State.(*ExampleToxicState)
```

If necessary, some global state can be stored in the toxic struct, which will not be
instanced per-connection. These fields cannot have a custom default value set and will
not be thread-safe, so proper locking or atomic operations will need to be used.

## Bidirectional toxics

Regular toxics are limited to data flowing in a single direction, so they can't make decisions
for the `downstream` based on a request in the `upstream`. For things like protocol aware toxics
this is a problem.

Bidirectional toxics allow state to be shared for the `upstream` and `downstream` pipes in a single
toxic implementation. They also ensure direction-specific code is always run on the correct pipe
(a toxic that only works on the `upstream` can't be added to the `downstream`).

Creating a bidirectional toxic is done by implementing a second `Pipe()` function called `PipeUpstream()`.
The implementation is same as a regular toxic, and can be paired with other types such as a stateful toxic.

One use case of a bidirectional toxic is to mock out the backend server entirely, which is shown below:

```go
type EchoToxic struct {}

type EchoToxicState struct {
    Request          chan *stream.StreamChunk
}

// PipeUpstream handles the upstream direction
func (t *EchoToxic) PipeUpstream(stub *toxics.ToxicStub) {
    state := stub.State.(*EchoToxicState)

    for {
        select {
        case <-stub.Interrupt:
            return
        case c := <-stub.Input:
            if c == nil {
                // Close the downstream when the client closes
                close(state.Request)
                stub.Close()
                return
            }
            // Send the data to the downstream through the state object
            state.Request <- c
        }
    }
}

// Pipe() will only handle the downstream on a bidirectional toxic
func (t *EchoToxic) Pipe(stub *toxics.ToxicStub) {
    state := stub.State.(*EchoToxicState)

    for {
        select {
        case <-stub.Interrupt:
            return
        case c := <-state.Request: // Read from the upstream instead of the server
            if c == nil {
                stub.Close()
                return
            }
            stub.Output <- c
        }
    }
}

func (t *EchoToxic) NewState() interface{} {
    return &EchoToxicState{
        Request: make(chan *stream.StreamChunk),
    }
}
```

This example will loop back all data send to the server back to the client. Another use case seen
within toxiproxy is to filter http response modifications based on the request URL (See the
[http toxic](https://github.com/Shopify/toxiproxy/tree/master/toxics/http.go)).

## Using `io.Reader` and `io.Writer`

If your toxic involves modifying the data going through a proxy, you can use the `ChanReader`
and `ChanWriter` interfaces in the [stream package](https://github.com/Shopify/toxiproxy/tree/master/stream).
These allow reading and writing from the input and output channels as you would a normal data
stream such as a TCP socket.

An implementation of the noop toxic above using the stream package would look something like this:

```go
func (t *NoopToxic) Pipe(stub *toxics.ToxicStub) {
    buf := make([]byte, 32*1024)
    writer := stream.NewChanWriter(stub.Output)
    reader := stream.NewChanReader(stub.Input)
    reader.SetInterrupt(stub.Interrupt)
    for {
        n, err := reader.Read(buf)
        if err == stream.ErrInterrupted {
            writer.Write(buf[:n])
            return
        } else if err == io.EOF {
            stub.Close()
            return
        }
        writer.Write(buf[:n])
    }
}
```

See https://github.com/xthexder/toxic-example/blob/master/http.go for a full example of using
the stream package with Go's http package.
