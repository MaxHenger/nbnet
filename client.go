package nbnet

import (
	"bytes"
	"net"
	"strconv"
	"sync"
	"time"
)

type clientConnection struct {
	protocol   Protocol
	address    string
	connection *Connection
}

type Client struct {
	deadlineRead  time.Duration
	deadlineWrite time.Duration
	sleepDuration time.Duration
	channelSize   int
	bufferSize    int
	connections   []*clientConnection
	waitGroup     sync.WaitGroup
}

//NewClient(...) should be used to create a new Client type. The client is a
//managing container capable of creating new internet connections. At the
//moment only the TCP connections have been proven to work, no testing has been
//performed on the Unix-protocol connections. The meaning of the arguments are
//as following:
//	- deadlineRead: This is the duration used during reading in the connection
//		routine before the process is considered to have timed out. This value
//		is not just the period in which the routine is waiting on the connected
//		server, this period also includes the actual reading. ~30 ms seems an
//		OK value
//	- deadlineWrite: This is the duration used during writing in the connection
//		routine. This value should be large enough to send all data. ~30 ms
//		seems an OK value.
//	- sleepDuration: When the client has nothing to write and the attempt to
//		read data resulted in a timeout, then it will assume the connection is
//		idle. Upon which the routine will sleep for the specified duration.
//	- channelSize: The channel size (if it is exceeded go will asume a deadlock
//		to be ocurring) of the connection's read, write and error channels (used
//		to send and receive byte slices, and errors)
//	- bufferSize: Reading from net.Conn implies multiple calls to the
//		io.Reader.Read(...) method. The temporary buffer passed to this function
//		has size 'bufferSize'. This temporary buffer is consecutively copied to
//		a total buffer which has an initial capacity of 'bufferSize'.
func NewClient(deadlineRead, deadlineWrite, sleepDuration time.Duration, channelSize, bufferSize int) *Client {
	return &Client{deadlineRead, deadlineWrite, sleepDuration, channelSize, bufferSize, nil, sync.WaitGroup{}}
}

//Client.ConnectIP(...) will attempt to create a new connection to the specified
//IP-address and port. It will do so using the specified protocol. In case the
//function succeeds, implying the error return value is nil, the returned
//connection pointer can be used to send and receive data.
func (c *Client) ConnectIP(protocol Protocol, ip *net.IP, port int) (*Connection, error) {
	//check protocol
	if !isValidProtocol(protocol) {
		return nil, Error{ErrorTypeFatal, "Client", "Invalid protocol specified"}
	}

	connection := &clientConnection{}
	connection.protocol = protocol

	//construct string address from provided arguments
	var buffer bytes.Buffer

	if ip != nil {
		buffer.WriteString(ip.String())
	}

	buffer.WriteString(":")
	buffer.WriteString(strconv.Itoa(port))

	connection.address = buffer.String()

	//dial out and create a new nbnet connection
	netConnection, err := net.Dial(protocolMap[protocol], connection.address)

	if err != nil {
		return nil, ErrorEmbedded{ErrorTypeFatal, "Client", "Unable to dial out", err}
	}
	
	//create nbnet connection instance
	c.waitGroup.Add(1)

	connection.connection, err = newConnection(netConnection, c.channelSize, c.bufferSize,
		c.deadlineRead, c.deadlineWrite, c.sleepDuration, &c.waitGroup)
		
	if err != nil {
		return nil, ErrorEmbedded{ErrorTypeFatal, "Client", "Unable to create connection instance", err}
	}

	//new connection created, add it to the array
	c.connections = append(c.connections, connection)

	return connection.connection, nil
}

//CloseConnection(...) will close a single connection and the associated routine
func (c *Client) CloseConnection(con *Connection) error {
	//find the connection
	for i, v := range c.connections {
		if v.connection == con {
			//found the connection, close and remove it
			v.connection.closeConnection()
			c.waitGroup.Add(-1)
			v.connection.connection.Close()
			c.connections = append(c.connections[:i], c.connections[i+1:]...)
			return nil
		}
	}
	
	//did not find the connection
	return Error{ErrorTypeNotFound, "Client", "Failed to close a connection"}
}

//Client.Close(...) will send messages to all connections created through this
//managing type and wait for all of the routines to finish.
func (c *Client) Close() error {
	//loop through all created connections
	for _, val := range c.connections {
		//signal the thread to quit
		val.connection.closeConnection()
	}

	//wait for all routines to finish
	c.waitGroup.Wait()
	
	//close the con.Conn connection instances
	for _, v := range c.connections {
		v.connection.connection.Close()
	}

	return nil
}
