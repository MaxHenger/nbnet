package nbnet

//Protocol is the typedefinition of a c-style enumeration specifying the various
//allowed connection protocols
type Protocol int

//ProtocolXXX constants are the valid c-style enumerations that can be used to
//assign to a Protocol type (defined above)
const (
	ProtocolTCP Protocol = iota
	ProtocolTCP4
	ProtocolTCP6
	ProtocolUnix
	ProtocolUnixPacket
	ProtocolTotal
)

var protocolMap = []string{"tcp", "tcp4", "tcp6", "unix", "unixpacket"}

//isValidProtocol(...) will check a Protocol type for an erronous definition. If
//the definition is valid this function will return true, it will return false
//otherwise
func isValidProtocol(p Protocol) bool {
	return p >= ProtocolTCP && p < ProtocolTotal
}

//isTCPProtocol(...) will check if a Protocol type specifies a TCP connection.
//If so then this function will return true. If not then this function will
//return false
func isTCPProtocol(p Protocol) bool {
	return p >= ProtocolTCP && p <= ProtocolTCP6
}

//isUnixProtocol(...) will check if a Protocol type specifies a UNIX connection.
//If so then this function will return true. If not then this function will
//return false
func isUnixProtocol(p Protocol) bool {
	return p >= ProtocolUnix && p <= ProtocolUnixPacket
}
