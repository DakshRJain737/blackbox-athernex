package serial

import (
	"bufio"
	"log"
	"time"

	goserial "go.bug.st/serial"

	"github.com/blackbox/broker/config"
	"github.com/blackbox/broker/models"
	"github.com/blackbox/broker/schema"
)

// MessageHandler is called for every valid message received from serial.
type MessageHandler func(msg models.RawMessage, ts int64)

// ErrorHandler is called for every schema error.
type ErrorHandler func(err models.SchemaError)

// StartAll launches one goroutine per configured serial port.
// Each goroutine automatically reconnects if the port disappears.
func StartAll(ports []config.SerialPort, onMsg MessageHandler, onErr ErrorHandler) {
	for _, p := range ports {
		go readPort(p, onMsg, onErr)
	}
}

func readPort(p config.SerialPort, onMsg MessageHandler, onErr ErrorHandler) {
	mode := &goserial.Mode{BaudRate: p.BaudRate}

	for {
		log.Printf("[serial] opening %s (nodeId=%s baud=%d)", p.Port, p.NodeID, p.BaudRate)
		port, err := goserial.Open(p.Port, mode)
		if err != nil {
			log.Printf("[serial] failed to open %s: %v — retry in 3s", p.Port, err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Printf("[serial] connected %s", p.Port)
		scanner := bufio.NewScanner(port)

		for scanner.Scan() {
			ts := time.Now().UnixMilli() // timestamp on receipt, not on Arduino
			line := scanner.Bytes()

			msg, schErr := schema.Validate(line, p.NodeID, ts)
			if schErr != nil {
				onErr(*schErr)
				continue
			}
			// Ensure nodeID is always set to the config value if Arduino didn't send one
			if msg.NodeID == "" {
				msg.NodeID = p.NodeID
			}
			onMsg(*msg, ts)
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[serial] read error on %s: %v", p.Port, err)
		}
		port.Close()
		log.Printf("[serial] %s disconnected — reconnecting in 2s", p.Port)
		time.Sleep(2 * time.Second)
	}
}
