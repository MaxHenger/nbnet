package nbnet

import (
	"bytes"
	"net"
	"strconv"
	"sync"
	"time"
)

//wrapListener is a dummy interface wrapping around the net.Listener interface
//and also implementing an interface method capable of setting the listener
//deadline. The net.Listener interface is implemented by net.TCPListener and
//net.UnixListener but does not expose the SetDeadline(...) method.
type wrapListener interface {
	net.Listener
	SetDeadline(time.Time) error
}

//wrapTCPListener implements the wrapListener interface and wraps around an
//instance of net.TCPListener
type wrapTCPListener struct {
	l *net.TCPListener
}

func (tcp wrapTCPListener) Accept() (net.Conn, error) {
	return tcp.l.Accept()
}

func (tcp wrapTCPListener) Close() error {
	return tcp.l.Close()
}

func (tcp wrapTCPListener) Addr() net.Addr {
	return tcp.l.Addr()
}

func (tcp wrapTCPListener) SetDeadline(t time.Time) error {
	return tcp.l.SetDeadline(t)
}

//wrapUnixListener implements the wrapListener interface and wraps around an
//instance of net.UnixListener
type wrapUnixListener struct {
	l *net.UnixListener
}

func (unix wrapUnixListener) Accept() (net.Conn, error) {
	return unix.l.Accept()
}

func (unix wrapUnixListener) Close() error {
	return unix.l.Close()
}

func (unix wrapUnixListener) Addr() net.Addr {
	return unix.l.Addr()
}

func (unix wrapUnixListener) SetDeadline(t time.Time) error {
	return unix.l.SetDeadline(t)
}

func serverRoutine(connections chan net.Conn, errors chan error, quit chan int, listener wrapListener, deadline time.Duration, wg *sync.WaitGroup) {
	//enter an infinite loop
	if wg != nil {
		//user wants this server routine to call waitgroup.Done() to synchronize quitting
		defer wg.Done()
	}

	for {
		listener.SetDeadline(time.Now().Add(deadline))
		c, err := listener.Accept()

		if err == nil {
			connections <- c
		} else {
			switch err := err.(type) {
			case net.Error:
				//net errors are only important if it is
				if !err.Timeout() {
					//net error was not a timeout error, send it back to the server
					errors <- ErrorEmbedded{ErrorTypeWarning, "serverRoutine", "Received a non-timeout net error", err}
				}
			default:
				//non-net error
				errors <- ErrorEmbedded{ErrorTypeWarning, "serverRoutine", "Received a non-net error", err}
			}
		}

		//check if a quit message is received
		select {
		case <-quit:
			return
		default:
			//make the select call non-blocking
		}
	}
}

type serverListener struct {
	protocol Protocol
	address  string
	listener wrapListener
	quit     chan int
}

type Server struct {
	//All variables relating to the creation and management of listeners
	listeners          []serverListener
	listenerErrors     chan error
	deadlineListen     time.Duration
	newConnections     chan net.Conn
	waitGroupListeners sync.WaitGroup

	//All variables relating to the creation and management of connections
	connections          []*Connection
	deadlineRead         time.Duration
	deadlineWrite        time.Duration
	sleepDuration        time.Duration
	channelSize          int
	bufferSize           int
	waitGroupConnections sync.WaitGroup
}

//NewServer(...) will create a new server type to create routines that will
//listen for certain ip-adresses on specified ports. The arguments to the
//function call will influence the behaviour of the server and mean the following:
// 	- deadlineListen: The duration for which the listener routine will listen
//		to an incomming connection before considering the listening to have timed
//		out.
//	- channelListen: The channel size (if it is exceeded go will assume a
//		deadlock to have occurred) of the connections that will be returned to
//		the server type by the listening routine.
//	- deadlineRead: See documentation for 'NewClient(...)'
//	- deadlineWrite: See documentation for 'NewClient(...)'
//	- sleepDuration: See documentation for 'NewClient(...)'
//	- channelConn: See documentation for 'NewClient(...)', argument 'channelSize'
//	- bufferCon: See documentation for 'NewClient(...)', argument 'bufferSize'
//
//In case a new server is succesfully created the function will not return an
//error and a non-nill *Server type.
func NewServer(deadlineListen time.Duration, channelListen int,
	deadlineRead, deadlineWrite, sleepDuration time.Duration, channelConn, bufferConn int) (*Server, error) {
	//ensure input arguments are valid
	if channelListen < 1 {
		return nil, Error{ErrorTypeFatal, "NewServer", "Channel size for listener-created new connections is too small"}
	}

	if channelConn < 1 {
		return nil, Error{ErrorTypeFatal, "NewServer", "Channel size for connection-retrieved data is too small"}
	}

	if bufferConn < 1 {
		//this value should really be larger anyway, but we're catching absolutely impossible values here
		return nil, Error{ErrorTypeFatal, "NewServer", "Buffer size for retrieve data from a connection is too small"}
	}

	return &Server{nil, make(chan error, channelListen), deadlineListen, make(chan net.Conn, channelListen), sync.WaitGroup{},
		nil, deadlineRead, deadlineWrite, sleepDuration, channelConn, bufferConn, sync.WaitGroup{}}, nil
}

//Listen(...) will spawn a new listening routine on which the Server will look
//for any incoming connections. The function arguments mean the following:
//	- protocol: A Protocol type specifying with which protcol to listen for
//		incoming connections
//	- ip: A pointer to a net.IP type specifying which IP-address to listen for.
//		In case this value is set to nil the server will accept any incoming
//		client irregardless of the ip-address
//	- port: The port on which the server is supposed to listen
//
//In case the listener is succesfully created the function will return an error
//value of 'nil'
func (s *Server) Listen(protocol Protocol, ip net.IP, port int) error {
	//check input arguments for errors
	if !isValidProtocol(protocol) {
		return Error{ErrorTypeFatal, "NewServer", "Invalid protocol specified"}
	}

	if port < 0 {
		return Error{ErrorTypeFatal, "NewServer", "Invalid listening port specified"}
	}

	//convert IP and port to string
	var buffer bytes.Buffer

	if ip != nil {
		buffer.WriteString(ip.String())
	}

	buffer.WriteString(":")
	buffer.WriteString(strconv.Itoa(port))

	//listen on a new port
	if isTCPProtocol(protocol) {
		//resolve address into tcp address
		tcpAddress, err := net.ResolveTCPAddr(protocolMap[protocol], buffer.String())

		if err != nil {
			return ErrorEmbedded{ErrorTypeFatal, "Server", "Failed to resolve TCP address", err}
		}

		//create new TCP port
		newListener, err := net.ListenTCP(protocolMap[protocol], tcpAddress)

		//check if the connection was handled correctly
		if err != nil {
			return ErrorEmbedded{ErrorTypeFatal, "Server", "Failed to listen on a new TCP port", err}
		}

		//new TCP port
		s.listeners = append(s.listeners, serverListener{protocol,
			buffer.String(), wrapTCPListener{newListener}, make(chan int)})
		newEntry := &s.listeners[len(s.listeners)-1]

		s.waitGroupListeners.Add(1)
		go serverRoutine(s.newConnections, s.listenerErrors, newEntry.quit, newEntry.listener,
			s.deadlineListen, &s.waitGroupListeners)
	} else {
		//resolve address into unix address
		unixAddress, err := net.ResolveUnixAddr(protocolMap[protocol], buffer.String())

		if err != nil {
			return ErrorEmbedded{ErrorTypeFatal, "Server", "Failed to resolve unix address", err}
		}

		//create new Unix port
		newListener, err := net.ListenUnix(protocolMap[protocol], unixAddress)

		//check if the connection was created correctly
		if err != nil {
			return ErrorEmbedded{ErrorTypeFatal, "Server", "Failed to listen on a new Unix port", err}
		}

		//new Unix port
		s.listeners = append(s.listeners, serverListener{protocol,
			buffer.String(), wrapUnixListener{newListener}, make(chan int)})
		newEntry := &s.listeners[len(s.listeners)-1]

		s.waitGroupListeners.Add(1)
		go serverRoutine(s.newConnections, s.listenerErrors, newEntry.quit, newEntry.listener,
			s.deadlineListen, &s.waitGroupListeners)
	}

	return nil
}

//Update(...) should be called regularly by the main program to ensure none of
//the channels will overflow. This function can return any of three possible
//return value combinations:
// 1) non-nil, true, nil: A listener has connected a new client, which is being
//		returned as the first return value
// 2) nil, false, nil: Nothing has happened: No new client and no error
// 3) nil, false, non-nil: One of the listening routines has experienced an
//		error and has returned this error value
func (s *Server) Update() (*Connection, bool, error) {
	//loop through all listeners
	select {
	case conn := <-s.newConnections:
		//new connection received, process it into a nbnet connection
		s.waitGroupConnections.Add(1)

		newConnection, err := newConnection(conn, s.channelSize, s.bufferSize, s.deadlineRead,
			s.deadlineWrite, s.sleepDuration, &s.waitGroupConnections)

		if err != nil {
			//Error occurred while creating the new nbnet connection
			return nil, false, ErrorEmbedded{ErrorTypeFatal, "Client", "Unable to create a connection instance", err}
		}

		s.connections = append(s.connections, newConnection)

		return newConnection, true, nil
	case recvError := <-s.listenerErrors:
		//error received from the listening thread, return it
		return nil, false, recvError
	default:
		//no new connection
		return nil, false, nil
	}
}

//CloseConnection(...) will close the single specified connection and its associated
//routine.
func (s *Server) CloseConnection(c *Connection) error {
	//lazy, inefficient loopup. But the connection list is not intended to grow
	//very large
	for i, v := range s.connections {
		if v == c {
			//found the connection of interest
			if !v.isClosed {
				v.closeConnection()
			}

			v.connection.Close()
			s.connections = append(s.connections[0:i], s.connections[i+1:]...)
			return nil
		}
	}

	return Error{ErrorTypeNotFound, "Server", "Failed to close a connection"}
}

//Close(...) will close any connections, listeners and their associated routines.
//This function used synchronizing WaitGroup types to ensure that all routines
//have finished before this function returns.
func (s *Server) Close() {
	//signal all connection routines to close down
	for _, v := range s.connections {
		if !v.isClosed {
			v.closeConnection()
		} else {
		}
	}

	//signal all listening routines to close down
	for _, v := range s.listeners {
		v.quit <- 0
	}

	//wait for all routines to finish
	s.waitGroupConnections.Wait()
	s.waitGroupListeners.Wait()

	//close the net.Conn and wrapListener instances
	for _, v := range s.connections {
		v.connection.Close()
	}

	for _, v := range s.listeners {
		v.listener.Close()
	}
}
