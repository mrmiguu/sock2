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
	Route    = "/317d37b0edc7bd7cbd25d97f53a16ce5"
	RteSep   = '\t'
	PktSep   = '▼'

	sock Socket
)

// Add adds a channel with a route to the standard socket.
func Add(ch interface{}, route ...string) {
	sock.Add(ch, route...)
}

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

	cons_mu sync.Mutex
	cons    map[*websocket.Conn]byte

	typRteChs_mu sync.Mutex
	typRteChs    map[string]map[string][][2]reflect.Value

	types_mu sync.Mutex
	types    map[string]reflect.Type
}

func (sock *Socket) init() {
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

	sock.typRteChs = make(map[string]map[string][][2]reflect.Value)
	sock.types = make(map[string]reflect.Type)

	if sock.IsClient {
		sock.initClient()
	} else {
		sock.initServer()
	}
}

func (sock *Socket) initClient() {
	wsOrWSS := "ws://"
	if sock.Secure {
		wsOrWSS = "wss://"
	}
	sock.ws = js.Global.Get("WebSocket").New(wsOrWSS + sock.Addr + sock.Route)
	sock.ws.Set("binaryType", "arraybuffer")
	println("ws open!")
	sock.ws.Set("onmessage", func(e *js.Object) {
		go func(pkt []byte) {
			// println("pkt=" + string(pkt))
			if err := sock.read(pkt, nil); err != nil {
				println(err.Error())
			}
		}(js.Global.Get("Uint8Array").New(e.Get("data")).Interface().([]byte))
	})
	sock.ws.Set("onclose", func() {
		println("ws close!")
	})
	c := make(chan byte, 1)
	sock.ws.Set("onopen", func() { c <- 0 })
	<-c
}

func (sock *Socket) initServer() {
	sock.mux = new(http.ServeMux)
	sock.cons = make(map[*websocket.Conn]byte)

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
		println(sock.Route + " !")
		con, err := upgr.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sock.cons_mu.Lock()
		sock.cons[con] = 0
		sock.cons_mu.Unlock()
		defer func() {
			sock.cons_mu.Lock()
			delete(sock.cons, con)
			sock.cons_mu.Unlock()
		}()
		for {
			// println("pkt...")
			mt, pkt, err := con.ReadMessage()
			if err != nil {
				println(err.Error())
				return
			} else if mt != websocket.BinaryMessage {
				println("message not binary")
				return
			} else if err := sock.read(pkt, con); err != nil {
				println(err.Error())
				return
			}
			// println("pkt=" + string(pkt))
		}
	})

	go http.ListenAndServe(sock.Addr, sock.mux)
}

// Add adds a channel with a route to the socket.
func (sock *Socket) Add(ch interface{}, route ...string) {
	sock.initOnce.Do(sock.init)

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
	typ := ele.String()
	sock.types_mu.Lock()
	if _, found := sock.types[typ]; !found {
		gob.Register(reflect.Zero(ele).Interface())
	}
	sock.types[typ] = ele
	sock.types_mu.Unlock()

	rte := sock.join(route)

	sock.typRteChs_mu.Lock()
	defer sock.typRteChs_mu.Unlock()

	rteChs, found := sock.typRteChs[typ]
	if !found {
		rteChs = make(map[string][][2]reflect.Value)
		sock.typRteChs[typ] = rteChs
	}

	i := len(rteChs[rte])
	r := reflect.MakeChan(t, 0)
	rteChs[rte] = append(rteChs[rte], [2]reflect.Value{r, c})

	go func() {
		for {
			// println("[" + typ + "][" + rte + "][" + itoa(i) + "] selecting")
			chosen, v, recvOK := reflect.Select([]reflect.SelectCase{
				{Dir: reflect.SelectRecv, Chan: r},
				{Dir: reflect.SelectRecv, Chan: c},
			})
			if !recvOK {
				println("[" + typ + "][" + rte + "][" + itoa(i) + "] broke")
				return
			}
			switch chosen {
			case 0:
				// println("[" + typ + "][" + rte + "][" + itoa(i) + "] read")
				c.Send(v)
			case 1:
				// println("[" + typ + "][" + rte + "][" + itoa(i) + "] write")
				var buf bytes.Buffer
				if err := gob.NewEncoder(&buf).EncodeValue(v); err != nil {
					panic("sock.encode: " + err.Error())
				}
				if err := sock.write(typ, rte, i, buf.Bytes()); err != nil {
					panic("sock.write: " + err.Error())
				}
			}
		}
	}()
}

func (sock *Socket) read(pkt []byte, con *websocket.Conn) error {
	p := bytes.Split(pkt, []byte(string([]rune{sock.PktSep})))
	if len(p) != 4 || len(p[0]) == 0 {
		return errors.New("invalid packet")
	}
	typ, rte, i, b := string(p[0]), string(p[1]), btoi(p[2]), p[3]

	sock.typRteChs_mu.Lock()
	defer sock.typRteChs_mu.Unlock()

	rteChs, found := sock.typRteChs[typ]
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
	r := chs[i][0]

	sock.types_mu.Lock()
	ele := sock.types[typ]
	sock.types_mu.Unlock()

	// println("<-[" + typ + "][" + rte + "][" + itoa(i) + "]...")
	v := reflect.New(ele).Elem()
	if err := gob.NewDecoder(bytes.NewReader(b)).DecodeValue(v); err != nil {
		return err
	}
	r.Send(v)
	// println("<-[" + typ + "][" + rte + "][" + itoa(i) + "]!")

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

	sock.cons_mu.Lock()
	defer sock.cons_mu.Unlock()

	// TODO: only write to cons requesting
	for con := range sock.cons {
		if err := con.WriteMessage(websocket.BinaryMessage, pkt); err != nil {
			println("sock.write: " + err.Error())
		}
	}

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
