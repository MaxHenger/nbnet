package nbnet

type Error struct {
	source  string
	message string
}

func (e Error) Error() string {
	return e.source + ": " + e.message
}

type ErrorEmbedded struct {
	source   string
	message  string
	embedded error
}

func (e ErrorEmbedded) Error() string {
	result := ""

	if e.embedded != nil {
		result += e.embedded.Error() + "\n"
	}

	result += e.source + ": " + e.message
	return result
}
