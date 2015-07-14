package nbnet

import (
	"net"
	"sync"
	"time"
	"bytes"
)

const PackageSizeMax = 256000000 //= 256 mb
const PackageSizeMin = 4 //=4 b

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

func pack4(p []byte) (result int32) {
	result |= int32(p[0]) << 24
	result |= int32(p[1]) << 16
	result |= int32(p[2]) << 8
	result |= int32(p[3])
	return
}

func unpack4(i int32) []byte {
	return []byte{byte((i >> 24) & 0xFF), 
				byte((i >> 16) & 0xFF), 
				byte((i >> 8) & 0xFF), 
				byte(i & 0xFF)}
}

func connectionRoutine(connection net.Conn, reading, writing chan []byte, errors chan error, quit chan int,
	deadlineRead, deadlineWrite, sleepDuration time.Duration, waitGroup *sync.WaitGroup, bufferSize int) {
	if waitGroup != nil {
		//connection creator wishes the connection routine to signal the waitgroup
		//when the routine is finished
		defer waitGroup.Done()
	}

	localBuffer := make([]byte, bufferSize)		//buffer to receive data into
	totalBuffer := make([]byte, 0, bufferSize)	//buffer that increases in size as package is built up
	expectedSize := int32(-1)					//expected size of the incoming package

	for {
		//check if there is something to write
		select {
		case task := <- writing:
			//new writing task
			packet := make([]byte, 0, len(task) + 4)
			packet = append(packet, unpack4(int32(len(task)))...)
			packet = append(packet, task...)

			total := 0
			for total < len(packet) {
				//write all data
				connection.SetWriteDeadline(time.Now().Add(deadlineWrite))
				written, err := connection.Write(packet)
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
		case <- quit:
			//thread should quit
			return
		default:
			//check if we can receive data from the current connection
			connection.SetReadDeadline(time.Now().Add(deadlineRead))

			for {
				curBytesRead, err := connection.Read(localBuffer)

				if curBytesRead != 0 {
					totalBuffer = append(totalBuffer, localBuffer[:curBytesRead]...)

					if expectedSize == -1 {
						//only attempt to retrieve the size if there is enough data
						if len(totalBuffer) >= 4 {
							expectedSize = pack4(totalBuffer)
							totalBuffer = totalBuffer[4:]
						}
					}
					
					//is there enough data for a single package
					for expectedSize != -1 && int32(len(totalBuffer)) >= expectedSize {
						//there is, return the current buffer section
						reading <- totalBuffer[:expectedSize]
						expectedOld := expectedSize
						
						if int32(len(totalBuffer)) - expectedSize >= 4 {
							//we can extract a new buffer size
							expectedSize = pack4(totalBuffer[expectedSize:])
							totalBuffer = totalBuffer[expectedOld + 4:]
						} else {
							//no new buffer size can be extracted
							expectedSize = -1
							totalBuffer = totalBuffer[expectedOld:]
						}
					}
				}

				if err != nil {
					//check what kind of error is returned
					switch err := err.(type) { //this shadow the previous err definition
					case net.Error:
						if err.Timeout() {
							//net timeout error. If any bytes were read this is
							//not a problem at all
							time.Sleep(sleepDuration)
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
func (c *Connection) Send(message []byte, encr Encrypter, sign Signer, state EncryptionState) error {
	if encr != nil {
		//encryption should be performed
		processed, err := encr.Encrypt(state, message)
		
		if err != nil {
			return ErrorEmbedded{ErrorTypeFatal, "Connection", "Failed to encrypt message", err}
		}
		
		if sign != nil {
			//signing should be performed as well
			var buffer bytes.Buffer
			buffer.Write(processed)
			
			processed, err = sign.Sign(state, processed)
			
			if err != nil {
				return ErrorEmbedded{ErrorTypeFatal, "Connection", "Failed to sign message after encrypting", err}
			}
			
			buffer.Write(processed)
			
			//send encrypted and signed message through channel
			c.writing <- buffer.Bytes()
		} else {
			//encryption is performed, but signing should not be
			c.writing <- processed
		}
	} else {
		if sign != nil {
			//signing should be performed, encryption is not occurring
			var buffer bytes.Buffer
			buffer.Write(message)
			
			processed, err := sign.Sign(state, message)
			
			if err != nil {
				return ErrorEmbedded{ErrorTypeFatal, "Connection", "Failed to sign message", err}
			}
			
			buffer.Write(processed)
			
			//send signed message through channel
			c.writing <- buffer.Bytes()
		} else {
			//only send message, without encryption and without signing
			c.writing <- message
		}
	}
	
	return nil
}

//Connection.Receive(...) will receive any data, if it is available, and return
//errors if they ocurred in the connection routine. Three possible cases of
//return values are possible:
// 1) non-nil, true, nil: Data is returned from the routine, no error ocurred
// 2) nil, false, nil: No data is returned, no error ocurred
// 3) nil, false, non-nil: An error has been returned from the routine
func (c *Connection) Receive(decr Decrypter, sign Signer, state EncryptionState) ([]byte, bool, error) {
	select {
	case retrieved := <-c.reading:
		//received new data, decrypt it if required
		if decr != nil {
			processed, err := decr.Decrypt(state, retrieved)
			
			if err != nil {
				return nil, false, ErrorEmbedded{ErrorTypeFatal, "Connection", "Failed to decrypt received message", err}
			}
			
			if sign != nil {
				//verify the result using the signer
				ok, err := sign.Verify(state, retrieved, processed)
				
				switch {
				case err != nil:
					//error while verifying message
					return nil, false, ErrorEmbedded{ErrorTypeFatal, "Connection", "Failed to verify received encrypted message", err}
				case ok:
					//message was decrypted and verified to be correct
					return processed, true, nil
				default:
					//message was decrypted but probably forged/incorrectly received
					return processed, true, ErrorEmbedded{ErrorTypeInconsistent, "Connection", "Verification of received encrypted message failed", err}
				}
			} else {
				return processed, true, nil
			}
		}

		//if this code is reached the message should not be decrypted
		if sign != nil {
			//but it should be verified
			ok, err := sign.Verify(state, nil, retrieved)
			
			switch {
			case err != nil:
				//error while verifying message
				return nil, false, ErrorEmbedded{ErrorTypeFatal, "Connection", "Failed to verify received message", err}
			case ok:
				//message was verified to be correct
				return retrieved, true, nil
			default:
				//verification of the message failed
				return retrieved, true, ErrorEmbedded{ErrorTypeInconsistent, "Connection", "Verification of received message failed", err}
			}
		}

		//if this code is reached the message should be decrypted nor verified
		return retrieved, true, nil
	case recvError := <- c.errors:
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
