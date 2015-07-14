package nbnet

type ErrorType uint8

const (
	ErrorTypeWriteTimeout	ErrorType	= iota	//used when writing data and a timeout occurs
	ErrorTypeReadTimeout						//user when reading and a timeout occurs
	ErrorTypeWrite	//used when a non-timeout error occurs while writing
	ErrorTypeRead	//used when a non-timeout error occurs while reading
	ErrorTypeInconsistent	//used when an inconsistent packet is retrieved
	ErrorTypeWarning	//used when an error is not fatal to the program operation
	ErrorTypeFatal	//used when the error is fatal to the program operation
	ErrorTypeTotal	//the total number of ErrorTypeXXX constants
)

const (
	stringErrorTypeWriteTimeout = "Write timeout"
	stringErrorTypeReadTimeout = "Read timeout"
	stringErrorTypeWrite = "Write"
	stringErrorTypeRead = "Read"
	stringErrorTypeWarning = "Warning"
	stringErrorTypeFatal = "Fatal"
	stringErrorTypeUnknown = "UKNOWN"
)

func (et ErrorType) String() string {
	switch (et) {
	case ErrorTypeWriteTimeout:
		return stringErrorTypeWriteTimeout
	case ErrorTypeReadTimeout:
		return stringErrorTypeReadTimeout
	case ErrorTypeWrite:
		return stringErrorTypeWrite
	case ErrorTypeRead:
		return stringErrorTypeRead
	case ErrorTypeWarning:
		return stringErrorTypeWarning
	case ErrorTypeFatal:
		return stringErrorTypeFatal
	}
	
	//default value
	return stringErrorTypeUnknown
}

type Error struct {
	etype	ErrorType
	source  string
	message string
}

func (e Error) Error() string {
	return e.source + "<" + e.etype.String() + ">: " + e.message
}

type ErrorEmbedded struct {
	etype	ErrorType
	source   string
	message  string
	embedded error
}

func (e ErrorEmbedded) Error() string {
	result := ""

	if e.embedded != nil {
		result += e.embedded.Error() + "\n"
	}

	result += e.source + "<" + e.etype.String() + ">: " + e.message
	return result
}
