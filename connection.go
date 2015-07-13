package nbnet

import (
	"net"
	"sync"
	"time"
)

type Connection struct {
	connection    net.Conn
	deadlineRead  time.Duration
	deadlineWrite time.Duration
	sleepDuration time.Duration
	reading       chan []byte
	writing       chan []byte
	errors        chan error
	quit          chan int
}

func connectionRoutine(connection net.Conn, reading, writing chan []byte, errors chan error, quit chan int,
	deadlineRead, deadlineWrite, sleepDuration time.Duration, waitGroup *sync.WaitGroup, bufferSize int) {
	if waitGroup != nil {
		//connection creator wishes the connection routine to signal the waitgroup
		//when the routine is finished
		defer waitGroup.Done()
	}

	localBuffer := make([]byte, bufferSize)
	totalBuffer := make([]byte, 0, bufferSize)

	for {
		//check if there is something to write
		select {
		case task := <-writing:
			//new writing task
			connection.SetWriteDeadline(time.Now().Add(deadlineWrite))

			total := 0
			for total < len(task) {
				//write all data
				written, err := connection.Write(task)
				if err != nil {
					//figure out what kind of error is received, as this might
					//have an impact on how the user managing this routine has
					//to response to the error
					switch err := err.(type) {
					case net.Error:
						//net error, check if it is a timeout
						if err.Timeout() {
							errors <- ErrorEmbedded{ErrorTypeWriteTimeout, "connectionRoutine", "Net timeout error occurred while writing data", err}
						} else {
							errors <- ErrorEmbedded{ErrorTypeWrite, "connectionRoutine", "Net error occurred while writing data", err}
						}
					default:
						errors <- ErrorEmbedded{ErrorTypeWrite, "connectionRoutine", "Generic error occurred while writing data", err}
					}
					break
				}

				total += written
			}
		case <-quit:
			//thread should quit
			connection.Close()
			return
		default:
			//check if we can receive data from the current connection
			connection.SetReadDeadline(time.Now().Add(deadlineRead))
			totalBytesRead := 0

			for {
				bytesRead, err := connection.Read(localBuffer)
				totalBytesRead += bytesRead

				if bytesRead != 0 {
					totalBuffer = append(totalBuffer, localBuffer[:bytesRead]...)
				}

				if err != nil {
					//check what kind of error is returned
					switch err := err.(type) { //this shadow the previous err definition
					case net.Error:
						if err.Timeout() {
							//net timeout error. If any bytes were read this is
							//not a problem at all
							if totalBytesRead != 0 {
								//full package read, return the package
								//TODO: Figure out when this is actually not an error
								//and this is an error.
								reading <- totalBuffer
							} else {
								//no data read and a timeout error, this is the
								//cue to let the current routine sleep
								time.Sleep(sleepDuration)
							}
						} else {
							errors <- ErrorEmbedded{ErrorTypeRead, "connectionRoutine", "A net non-timeout error occurred while receiving data", err}
						}
					default:
						errors <- ErrorEmbedded{ErrorTypeRead, "connectionRoutine", "A generic error ocurred while receiving data", err}
					}
					//whatever happened, this is the cue to stop reading
					break
				}
			}

			if totalBytesRead != 0 {
				//Create a new buffer (the old one is being sent back to the main
				//routine as a slice) with the same capacity as the old one
				totalBuffer = make([]byte, 0, cap(totalBuffer))
			}
		}
	}
}

//newConnection will create a new Connection type. User specified arguments and
//their meaning are:
//	- channelSize: The channel size (if exceeded, go assumes a routine deadlock)
//		of the reading, writing and error channels.
//	- bufferSize: Reading from net.Conn implies multiple calls to the
//		io.Reader.Read(...) method. The temporary buffer passed to this function
//		has size 'bufferSize'. This temporary buffer is consecutively copied to
//		a total buffer which has an initial capacity of 'bufferSize'.
//	- deadlineRead: The duration before a net.Conn.Read call is assumed to
//		result in a timeout. Note that this time includes actually reading data,
//		so do not make it too small. ~30 ms seems to be okay.
//	- deadlineWrite: The duration before a net.Conn.Write call is assumed to
//		have timed out. The time should be large enough to allow writing data.
//		~30 ms seems to be okay
//	- sleep: If net.Conn.Read times out and no data is received (the connection
//		is idle), this duration specifies how long the routine should sleep
func newConnection(connection net.Conn, channelSize, bufferSize int, deadlineRead, deadlineWrite, sleep time.Duration, waitGroup *sync.WaitGroup) (*Connection, error) {
	//check the provided values for errors
	if channelSize < 1 {
		return nil, Error{ErrorTypeFatal, "newConnection", "Channel size is too small"}
	}
	
	if bufferSize < 1 {
		return nil, Error{ErrorTypeFatal, "newConnection", "Buffer size is too small"}
	}
	
	if deadlineRead <= 0 {
		return nil, Error{ErrorTypeFatal, "newConnection", "Reading deadline duration is too small"}
	}
	
	if deadlineWrite <= 0 {
		return nil, Error{ErrorTypeFatal, "newConnection", "Writing deadline duration is too small"}
	}
	
	//Create new connection
	newConnection := &Connection{connection, deadlineRead, deadlineWrite, sleep,
		make(chan []byte, channelSize), make(chan []byte, channelSize), make(chan error, channelSize), make(chan int)}

	//create the go routine belonging to the new connection
	go connectionRoutine(newConnection.connection, newConnection.reading, newConnection.writing,
		newConnection.errors, newConnection.quit, deadlineRead, deadlineWrite, sleep, waitGroup, bufferSize)

	return newConnection, nil
}

//Connection.Send(...) will push the data to-be-sent onto the channel that will
//be read by the connection routine. The routine will actually send the data.
//Hence make sure that the data is being retained in memory until it is sent
func (c *Connection) Send(p []byte) error {
	//send the provided data through the channel
	c.writing <- p
	return nil
}

//Connection.Receive(...) will receive any data, if it is available, and return
//errors if they ocurred in the connection routine. Three possible cases of
//return values are possible:
// 1) non-nil, true, nil: Data is returned from the routine, no error ocurred
// 2) nil, false, nil: No data is returned, no error ocurred
// 3) nil, false, non-nil: An error has been returned from the routine
func (c *Connection) Receive() ([]byte, bool, error) {
	select {
	case retrieved := <-c.reading:
		//received new data
		return retrieved, true, nil
	case recvError := <-c.errors:
		//received an error from the connection routine
		return nil, false, recvError
	default:
		return nil, false, nil
	}
}

//Connection.closeConnection(...) will close the associated net.Conn type and
//will signal the connection routine to quit. I'm aware of the unneccesary suffix,
//but the 'close' word is already reserved by go, and its a non-exported function
//anyway
func (c *Connection) closeConnection() {
	c.quit <- 1
}
