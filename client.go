package etpi

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

type Client interface {
	Connect(string, string, string) error
	Disconnect()
	Send(Command) error
	Status() error
	HandleZoneState(func(int, ZoneStatus))
	HandlePartitionState(func(int, PartitionStatus))
	HandleKeypadState(func(KeypadStatus))
}

type client struct {
	conn net.Conn
	pwd  string
	code string
	sync.RWMutex
	response        chan Command
	handleZone      func(int, ZoneStatus)
	handlePartition func(int, PartitionStatus)
	handleKeypad    func(KeypadStatus)
}

func NewClient() Client {
	return &client{response: make(chan Command)}
}

func (c *client) Connect(host string, pwd string, code string) error {
	conn, err := net.DialTimeout("tcp", host, time.Second)
	if err != nil {
		return err
	}
	c.conn = conn
	c.pwd = pwd
	c.code = code
	go c.listen()
	return nil
}

func (c *client) Disconnect() {
	if c.conn != nil {
		c.conn.Close()
	}
}

var ErrCommandError = errors.New("command error, bad checksum")
var ErrAPICommandSyntaxError = errors.New("syntax error")
var ErrAPICommandPartitionError = errors.New("requested partition is out of bounds")
var ErrAPICommandNotSupported = errors.New("command not supported")
var ErrAPISystemNotArmed = errors.New("system not armed")
var ErrAPISystemNotReadytoArm = errors.New("system not ready, either not secure, in exit-delay, or already armed")
var ErrAPICommandInvalidLength = errors.New("invalid length")
var ErrAPIUserCodenotRequired = errors.New("user code not required")
var ErrAPIInvalidCharacters = errors.New("invalid characters")

func (c *client) Send(cmd Command) error {
	log.Println("-> ", cmd)
	err := c.write(cmd)
	if err != nil {
		return err
	}
	select {
	case resp := <-c.response:
		switch resp.Code {
		case CommandAck:
			return nil
		case CommandCommandError:
			return ErrCommandError
		case CommandSystemError:
			switch resp.Data {
			case "000":
				return nil
			case "020":
				return ErrAPICommandSyntaxError
			case "021":
				return ErrAPICommandPartitionError
			case "022":
				return ErrAPICommandNotSupported
			case "023":
				return ErrAPISystemNotArmed
			case "024":
				return ErrAPISystemNotReadytoArm
			case "025":
				return ErrAPICommandInvalidLength
			case "026":
				return ErrAPIUserCodenotRequired
			case "027":
				return ErrAPIInvalidCharacters
			default:
				return fmt.Errorf("unknwon system error %s", resp.Data)
			}
		}
	case <-time.After(time.Second):
	}

	return errors.New("timeout awaiting response")
}

func (c *client) write(cmd Command) error {
	c.Lock()
	defer c.Unlock()
	_, err := cmd.WriteTo(c.conn)
	return err
}

func (c *client) listen() {
	r := bufio.NewReader(c.conn)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return
		}
		c.handle(line)
	}
}

func (c *client) handle(p []byte) {
	cmd, err := NewCommandFromBytes(p)
	if err != nil {
		return
	}
	// log.Println("<- ", cmd)
	switch cmd.Code {
	case CommandAck, CommandCommandError, CommandSystemError:
		go func() {
			select {
			case c.response <- *cmd:
			default:
			}
		}()
	case CommandLoginStatus:
		switch cmd.Data[0] {
		case '0', '2':
			c.Disconnect()
		case '1':
			c.Status()
		case '3':
			c.login()
		}
	case CommandZoneAlarm:
		zone, _ := strconv.Atoi(string(cmd.Data[1:4]))
		c.handleZone(zone, ZoneStatusAlarm)
	case CommandZoneTamper:
		zone, _ := strconv.Atoi(string(cmd.Data[1:4]))
		c.handleZone(zone, ZoneStatusTamper)
	case CommandZoneFault:
		zone, _ := strconv.Atoi(cmd.Data)
		c.handleZone(zone, ZoneStatusFault)
	case CommandZoneOpen:
		zone, _ := strconv.Atoi(cmd.Data)
		c.handleZone(zone, ZoneStatusOpen)
	case CommandZoneRestored:
		zone, _ := strconv.Atoi(cmd.Data)
		c.handleZone(zone, ZoneStatusRestored)
	case CommandPartitionReady:
		partition, _ := strconv.Atoi(cmd.Data)
		c.handlePartition(partition, PartitionStatusReady)
	case CommandPartitionNotReady:
		partition, _ := strconv.Atoi(cmd.Data)
		c.handlePartition(partition, PartitionStatusNotReady)
	case CommandPartitionBusy:
		partition, _ := strconv.Atoi(cmd.Data)
		c.handlePartition(partition, PartitionStatusBusy)
	case CommandPartitionArmed:
		partition, _ := strconv.Atoi(cmd.Data[:1])
		mode, _ := strconv.Atoi(cmd.Data[1:2])
		switch mode {
		case 0:
			c.handlePartition(partition, PartitionStatusArmedAway)
		case 1:
			c.handlePartition(partition, PartitionStatusArmedStay)
		case 2:
			c.handlePartition(partition, PartitionStatusArmedZeroEntryAway)
		case 3:
			c.handlePartition(partition, PartitionStatusArmedZeroEntryStay)
		}
	case CommandPartitionDisarmed:
		partition, _ := strconv.Atoi(cmd.Data)
		c.handlePartition(partition, PartitionStatusDisarmed)
	case CommandPartitionAlarm:
		partition, _ := strconv.Atoi(cmd.Data)
		c.handlePartition(partition, PartitionStatusAlarm)
	case CommandPartitionExitDelay:
		partition, _ := strconv.Atoi(cmd.Data)
		c.handlePartition(partition, PartitionStatusExitDelay)
	case CommandPartitionEntryDelay:
		partition, _ := strconv.Atoi(cmd.Data)
		c.handlePartition(partition, PartitionStatusEntryDelay)
	case CommandPartitionSpecialClosing:
		// ignore
	case CommandKeypadLed:
		tmp, _ := hex.DecodeString(cmd.Data)
		state := tmp[0]
		status := KeypadStatus{
			Backlight: (state&0x80)>>7 == 1,
			Fire:      (state&0x40)>>6 == 1,
			Program:   (state&0x20)>>5 == 1,
			Trouble:   (state&0x10)>>4 == 1,
			Bypass:    (state&0x08)>>3 == 1,
			Memory:    (state&0x04)>>2 == 1,
			Armed:     (state&0x02)>>1 == 1,
			Ready:     (state & 0x01) == 1,
		}
		c.handleKeypad(status)
	case CommandCodeRequired:
		log.Println("code requested, sending response")
		cmd := Command{Code: CommandCode, Data: c.code}
		c.Send(cmd)
	default:
		log.Printf("error: APICommandNotSupported: %v\n", cmd)
	}
}

func (c *client) login() error {
	cmd := Command{Code: CommandLogin, Data: c.pwd}
	return c.Send(cmd)
}

func (c *client) Status() error {
	cmd := Command{Code: CommandStatusReport}
	return c.Send(cmd)
}

func (c *client) poll() error {
	cmd := Command{Code: CommandPoll}
	return c.Send(cmd)
}

func (c *client) HandleZoneState(f func(int, ZoneStatus)) {
	c.handleZone = f
}

func (c *client) HandlePartitionState(f func(int, PartitionStatus)) {
	c.handlePartition = f
}

func (c *client) HandleKeypadState(f func(KeypadStatus)) {
	c.handleKeypad = f
}
