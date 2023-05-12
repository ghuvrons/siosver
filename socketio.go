package siosver

type Manager struct {
	server          *Server
	sockets         map[string]*Socket // key: namespace
	bufferingsocket *Socket
}

func newManager(server *Server) *Manager {
	return &Manager{
		server:  server,
		sockets: map[string]*Socket{}, // key: namespaces
	}
}
