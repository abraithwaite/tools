package broadcast

import (
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type msg struct {
	data []byte
	typ  int
}

type server struct {
	rooms map[string]*userlist
	ws    websocket.Upgrader
	l     sync.Mutex
}

type userlist struct {
	wl    sync.Mutex
	conns map[string]*websocket.Conn
	write chan msg
}

func NewWS() *server {
	return &server{
		rooms: make(map[string]*userlist),
		ws: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	room := mux.Vars(r)["room"]
	if room == "" {
		room = "default"
	}
	conn, err := s.ws.Upgrade(w, r, w.Header())
	if err != nil {
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
	}
	log.Println("conn:", conn.RemoteAddr().String())
	s.l.Lock()
	defer s.l.Unlock()

	var ul *userlist
	if ul = s.rooms[room]; ul == nil {
		ul = &userlist{
			conns: make(map[string]*websocket.Conn),
			write: make(chan msg, 100),
		}
		go func() {
			ul.Run()
			s.l.Lock()
			delete(s.rooms, room)
			s.l.Unlock()
		}()
		s.rooms[room] = ul
	}
	ul.addConn(conn)
	go ul.listen(conn, ul.write)

}

func (ul *userlist) listen(conn *websocket.Conn, c chan<- msg) {
	for {
		typ, rdr, err := conn.NextReader()
		if err != nil {
			log.Printf("Error calling next: %v", err)
			ul.delConn(conn)
			break
		}
		b, err := ioutil.ReadAll(rdr)
		if err != nil {
			log.Printf("Error reading: %v", err)
			ul.delConn(conn)
			break
		}
		// this will probably panic if we lose all clients
		// right as a new one is joining
		c <- msg{data: b, typ: typ}
	}
}

func (ul *userlist) addConn(c *websocket.Conn) {
	ul.wl.Lock()
	defer ul.wl.Unlock()
	ul.conns[c.RemoteAddr().String()] = c
}

func (ul *userlist) delConn(c *websocket.Conn) {
	ul.wl.Lock()
	delete(ul.conns, c.RemoteAddr().String())
	if len(ul.conns) == 0 {
		close(ul.write)
	}
	ul.wl.Unlock()
}

func (ul *userlist) Run() {
	for m := range ul.write {
		for _, conn := range ul.conns {
			log.Println("sending:", string(m.data))
			if err := conn.WriteMessage(m.typ, m.data); err != nil {
				ul.delConn(conn)
				log.Println(err)
			}
		}
	}
}
