package nbnet

type ErrorType uint16

const (
	ErrorTypeWriteTimeout	ErrorType	= iota	//used when writing data and a timeout occurs
	ErrorTypeReadTimeout						//user when reading and a timeout occurs
	ErrorTypeWrite	//used when a non-timeout error occurs while writing
	ErrorTypeRead	//used when a non-timeout error occurs while reading
	ErrorTypeInconsistent	//used when an inconsistent packet is retrieved
	ErrorTypeWarning	//used when an error is not fatal to the program operation
	ErrorTypeNotFound //used when an element is not found
	ErrorTypeFatal	//used when the error is fatal to the program operation
	ErrorTypeTotal	//the total number of ErrorTypeXXX constants
)

const (
	stringErrorTypeWriteTimeout = "Write timeout"
	stringErrorTypeReadTimeout = "Read timeout"
	stringErrorTypeWrite = "Write"
	stringErrorTypeRead = "Read"
	stringErrorTypeWarning = "Warning"
	stringErrorTypeNotFound = "Not found"
	stringErrorTypeFatal = "Fatal"
	stringErrorTypeUnknown = "UNKNOWN"
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
	case ErrorTypeNotFound:
		return stringErrorTypeNotFound
	case ErrorTypeFatal:
		return stringErrorTypeFatal
	}
	
	//default value
	return stringErrorTypeUnknown
}

type Error struct {
	EType	ErrorType
	Source  string
	Message string
}

func (e Error) Error() string {
	return e.Source + "<" + e.EType.String() + ">: " + e.Message
}

type ErrorEmbedded struct {
	EType	ErrorType
	Source   string
	Message  string
	Embedded error
}

func (e ErrorEmbedded) Error() string {
	result := ""

	if e.Embedded != nil {
		result += e.Embedded.Error() + "\n"
	}

	result += e.Source + "<" + e.EType.String() + ">: " + e.Message
	return result
}
