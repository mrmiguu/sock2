package sock2

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gopherjs/gopherjs/js"
	"github.com/gorilla/websocket"
)

var (
	itoa = strconv.Itoa
)

var (
	IsClient = js.Global != nil
	Secure   = false
	Addr     = "localhost:80"
	Root     = "www"
	Route    = "/43138aa2e4a54cecb4582dd548c2642b2e2ab69c"
	RteSep   = '\t'
	PktSep   = '\v'

	sock Socket
)

// Add adds a channel with a route to the standard socket.
func Add(ch interface{}, route ...string) {
	sock.Add(ch, route...)
}

type (
	types             map[string]reflect.Type
	channels          map[int]channel
	routeChannels     map[string]channels
	typeRouteChannels map[string]routeChannels
	connections       map[*websocket.Conn]byte
)

type safe struct {
	mu sync.RWMutex
}

func (s *safe) lock() {
	s.mu.Lock()
}
func (s *safe) unlock() {
	s.mu.Unlock()
}
func (s *safe) rlock() {
	s.mu.RLock()
}
func (s *safe) runlock() {
	s.mu.RUnlock()
}

type channel struct {
	reflect.Value
	connections
}

type safeTRC struct {
	safe
	typeRouteChannels
}
type safeTypes struct {
	safe
	types
}

// Socket is a line of communication between remote endpoints.
type Socket struct {
	initOnce sync.Once

	IsClient bool
	Secure   bool
	Addr     string
	Root     string
	Route    string
	RteSep   rune
	PktSep   rune

	ws  *js.Object
	mux *http.ServeMux

	safeTRC
	safeTypes
}

func (sock *Socket) init() {
	sock.initOnce.Do(func() {
		if !sock.IsClient {
			sock.IsClient = IsClient
		}
		if !sock.Secure {
			sock.Secure = Secure
		}
		if len(sock.Addr) == 0 {
			sock.Addr = Addr
		}
		if len(sock.Root) == 0 {
			sock.Root = Root
		}
		if len(sock.Route) == 0 {
			sock.Route = Route
		}
		if sock.RteSep == 0 {
			sock.RteSep = RteSep
		}
		if sock.PktSep == 0 {
			sock.PktSep = PktSep
		}

		sock.typeRouteChannels = make(typeRouteChannels)
		sock.types = make(types)

		if sock.IsClient {
			sock.initClient()
		} else {
			sock.initServer()
		}
	})
}

func (sock *Socket) initClient() {
	wsOrWSS := "ws://"
	if sock.Secure {
		wsOrWSS = "wss://"
	}
	sock.ws = js.Global.Get("WebSocket").New(wsOrWSS + sock.Addr + sock.Route)
	sock.ws.Set("binaryType", "arraybuffer")
	sock.ws.Set("onmessage", func(e *js.Object) {
		go func(pkt []byte) {
			// println("pkt=" + string(pkt))
			if err := sock.read(pkt, nil); err != nil {
				println("sock.read: " + err.Error())
			}
		}(js.Global.Get("Uint8Array").New(e.Get("data")).Interface().([]byte))
	})
	sock.ws.Set("onclose", func() {
		println("ws close!")
	})
	c := make(chan byte, 1)
	sock.ws.Set("onopen", func() { c <- 0 })
	<-c
	// println("ws open!")
}

func (sock *Socket) initServer() {
	sock.mux = new(http.ServeMux)

	if len(sock.Root) > 0 {
		if _, err := os.Stat(sock.Root); os.IsNotExist(err) {
			panic("root folder missing")
		}
		sock.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			ext := filepath.Ext(r.URL.Path)
			if ext == ".gz" {
				w.Header().Add("Content-Encoding", "gzip")
			}
			if i := strings.LastIndex(r.URL.Path, ext); i != -1 {
				ext = filepath.Ext(r.URL.Path[:i])
				if t := mime.TypeByExtension(ext); len(t) > 0 {
					w.Header().Add("Content-Type", t)
				}
			}
			http.ServeFile(w, r, sock.Root+r.URL.Path)
		})
	}

	upgr := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	sock.mux.HandleFunc(sock.Route, func(w http.ResponseWriter, r *http.Request) {
		// println(sock.Route + " !")
		con, err := upgr.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for {
			// println("pkt...")
			mt, pkt, err := con.ReadMessage()
			if err != nil { // TODO: clean up cons to-be in typRteChs
				println("sock.con: " + err.Error())
				break
			} else if mt != websocket.BinaryMessage {
				println("sock: message not binary")
				break
			} else if err := sock.read(pkt, con); err != nil {
				println("sock.read: " + err.Error())
				break
			}
			// println("pkt=" + string(pkt))
		}
		sock.safeTRC.lock()
		defer sock.safeTRC.unlock()
		for typ, rteChs := range sock.typeRouteChannels {
			for rte, chs := range rteChs {
				for i, ch := range chs {
					if _, found := ch.connections[con]; found {
						println("deleting connection [" + typ + "][" + rte + "][" + itoa(i) + "]")
						delete(ch.connections, con)
					}
				}
			}
		}
	})

	go http.ListenAndServe(sock.Addr, sock.mux)
}

// Add adds a channel with a route to the socket.
func (sock *Socket) Add(ch interface{}, route ...string) {
	sock.init()

	c := reflect.ValueOf(ch)
	t := reflect.TypeOf(ch)

	switch {
	case ch == nil:
		panic("sock.Add: nil chan")
	case c.Kind() != reflect.Chan:
		panic("sock.Add: not a chan")
	case t.ChanDir() != reflect.BothDir:
		panic("sock.Add: bad chan dir")
	}

	ele := t.Elem()
	isErr := isErr(ele)
	if !isErr && ele.Kind() == reflect.Interface {
		panic("sock.Add: interface chan")
	}

	typ := ele.String()
	sock.safeTypes.lock()
	if _, found := sock.types[typ]; !isErr && !found {
		gob.Register(reflect.Zero(ele).Interface())
	}
	sock.types[typ] = ele
	sock.safeTypes.unlock()

	rte := sock.join(route)

	sock.safeTRC.lock()
	defer sock.safeTRC.unlock()

	rteChs, found := sock.typeRouteChannels[typ]
	if !found {
		rteChs = make(routeChannels)
		sock.typeRouteChannels[typ] = rteChs
	}

	chs, found := rteChs[rte]
	if !found {
		chs = make(channels)
		rteChs[rte] = chs
	}
	r := reflect.MakeChan(t, 0)
	i := len(chs)
	chs[i] = channel{r, make(connections)}

	if sock.IsClient {
		println("adding connection [" + typ + "][" + rte + "][" + itoa(i) + "]")
		if err := sock.write(typ, rte, i, []byte{}); err != nil { // connect
			panic("sock.write: " + err.Error())
		}
	}

	go func() {
		for {
			// println("[" + typ + "][" + rte + "][" + itoa(i) + "] selecting")
			chosen, v, recvOK := reflect.Select([]reflect.SelectCase{
				{Dir: reflect.SelectRecv, Chan: r},
				{Dir: reflect.SelectRecv, Chan: c},
			})
			if !recvOK {
				sock.safeTRC.lock()
				defer sock.safeTRC.unlock()
				rteChs := sock.typeRouteChannels[typ]
				chs := rteChs[rte]
				println("deleting all connections [" + typ + "][" + rte + "][" + itoa(i) + "]")
				delete(chs, i)
				if len(chs) == 0 {
					println("deleting all connections [" + typ + "][" + rte + "]")
					delete(rteChs, rte)
					if len(rteChs) == 0 {
						println("deleting all connections [" + typ + "]")
						delete(sock.typeRouteChannels, typ)
					}
				}
				return
			}
			switch chosen {
			case 0:
				// println("[" + typ + "][" + rte + "][" + itoa(i) + "] read")
				c.Send(v)
			case 1:
				// println("[" + typ + "][" + rte + "][" + itoa(i) + "] write")
				var b []byte
				if !isErr {
					var buf bytes.Buffer
					if err := gob.NewEncoder(&buf).EncodeValue(v); err != nil {
						panic("sock.encode: " + err.Error())
					}
					b = buf.Bytes()
				} else {
					b = []byte{}
					if err, ok := v.Interface().(error); ok && err != nil {
						b = []byte(err.Error())
					}
				}
				if err := sock.write(typ, rte, i, b); err != nil {
					panic("sock.write: " + err.Error())
				}
			}
		}
	}()
}

func (sock *Socket) read(pkt []byte, con *websocket.Conn) error {
	p := bytes.Split(pkt, []byte(string([]rune{sock.PktSep})))
	if len(p) != 4 {
		return errors.New("invalid packet")
	}
	typ, rte, i, b := string(p[0]), string(p[1]), btoi(p[2]), p[3]

	sock.safeTRC.lock()
	rteChs, found := sock.typeRouteChannels[typ]
	if !found {
		sock.safeTRC.unlock()
		return errors.New("[" + typ + "] not found")
	}
	chs, found := rteChs[rte]
	if !found {
		sock.safeTRC.unlock()
		return errors.New("[" + typ + "][" + rte + "] not found")
	}
	if i >= len(chs) {
		sock.safeTRC.unlock()
		return errors.New("[" + typ + "][" + rte + "][" + itoa(i) + "] not found")
	}
	r := chs[i]
	if con != nil {
		if _, found := r.connections[con]; !found {
			println("adding connection [" + typ + "][" + rte + "][" + itoa(i) + "]")
			r.connections[con] = 0
			sock.safeTRC.unlock()
			return nil
		}
	}
	sock.safeTRC.unlock()

	sock.safeTypes.rlock()
	ele := sock.types[typ]
	sock.safeTypes.runlock()

	isErr := isErr(ele)
	if isErr {
		var err error
		if len(b) > 0 {
			err = errors.New(string(b))
		}
		r.Interface().(chan error) <- err
		return nil
	}

	v := reflect.New(ele).Elem()
	if err := gob.NewDecoder(bytes.NewReader(b)).DecodeValue(v); err != nil {
		return err
	}

	r.Send(v)
	return nil
}

func (sock *Socket) write(typ, rte string, i int, b []byte) (err error) {
	defer catch(&err)

	pkt := bytes.Join([][]byte{[]byte(typ), []byte(rte), itob(i), b}, []byte(string([]rune{sock.PktSep})))

	if sock.IsClient {
		// println("[" + typ + "][" + rte + "][" + itoa(i) + "] <- '" + string(b) + "'...")
		sock.ws.Call("send", pkt)
		// println("[" + typ + "][" + rte + "][" + itoa(i) + "] <- '" + string(b) + "'!")
		return
	}

	sock.safeTRC.rlock()
	defer sock.safeTRC.runlock()

	rteChs, found := sock.typeRouteChannels[typ]
	if !found {
		return errors.New("[" + typ + "] not found")
	}
	chs, found := rteChs[rte]
	if !found {
		return errors.New("[" + typ + "][" + rte + "] not found")
	}
	if i >= len(chs) {
		return errors.New("[" + typ + "][" + rte + "][" + itoa(i) + "] not found")
	}
	r := chs[i]

	if typ == "error" {
		println(`con.WriteMessage...`, len(r.connections))
	}
	for con := range r.connections {
		if err := con.WriteMessage(websocket.BinaryMessage, pkt); err != nil {
			println("sock.write: " + err.Error())
		}
	}
	if typ == "error" {
		println(`con.WriteMessage!`, len(r.connections))
	}
	// println("[" + typ + "][" + rte + "][" + itoa(i) + "] connections written to")

	return
}

func (sock *Socket) join(route []string) string {
	return strings.Join(route, string([]rune{sock.RteSep}))
}

func catch(err *error) {
	r := recover()
	if r == nil {
		return
	}
	switch e := r.(type) {
	case *js.Error:
		*err = e
	case string:
		*err = errors.New(e)
	case error:
		*err = e
	default:
		panic("unknown panic")
	}
}

func btoi(b []byte) int {
	return int(binary.BigEndian.Uint64(b))
}
func itob(i int) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(i))
	return b[:]
}

func isErr(t reflect.Type) bool {
	return t.Kind() == reflect.Interface && t.Implements(reflect.TypeOf((*error)(nil)).Elem())
}
