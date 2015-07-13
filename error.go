package nbnet

type ErrorType uint8

const (
	ErrorTypeWriteTimeout	ErrorType	= iota
	ErrorTypeReadTimeout
	ErrorTypeWrite
	ErrorTypeRead
	ErrorTypeWarning
	ErrorTypeFatal
	ErrorTypeTotal
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
