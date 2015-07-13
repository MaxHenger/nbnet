package nbnet

import (
	"testing"
)

func TestValidProtocol(t *testing.T) {
	//test some invalid and some valid protocol values
	for i := ProtocolTCP - 5; i < ProtocolTCP; i++ {
		if isValidProtocol(Protocol(i)) {
			t.Errorf("isValidProtocol(%d) should have been considered invalid\n", i)
		}
	}

	//test some valid protocols
	for i := ProtocolTCP; i < ProtocolTotal; i++ {
		if !isValidProtocol(Protocol(i)) {
			t.Errorf("isValidProtocol(%v) should have been considered valid\n", Protocol(i))
		}
	}

	//test some more invalid protocols
	for i := ProtocolTotal; i < ProtocolTotal+5; i++ {
		if isValidProtocol(Protocol(i)) {
			t.Errorf("isValidProtocol(%v) should have been considered invalid\n", i)
		}
	}
}

func TestIsTCPProtocol(t *testing.T) {
	//test the TCP protocols
	for i := ProtocolTCP; i < ProtocolTotal; i++ {
		shouldBe := i <= ProtocolTCP6

		if isTCPProtocol(i) != shouldBe {
			t.Errorf("Expected isTCPProtocol(%v) to be %b", Protocol(i), shouldBe)
		}
	}
}

func TestISUnixProtocol(t *testing.T) {
	//test the unix protocols
	for i := ProtocolTCP; i < ProtocolTotal; i++ {
		shouldBe := i >= ProtocolUnix

		if isUnixProtocol(i) != shouldBe {
			t.Errorf("Expected isUnixProtocol(%v) to be %b", Protocol(i), shouldBe)
		}
	}
}
