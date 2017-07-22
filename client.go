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

// Connect opens a connection to an Envisalink device.
//
// The Envisalink acts as a server for the TCP connection and the user
// application is the client. The Envisalink listens on port 4025 and will only
// accept one client connection on that port. Any subsequent connections will be
// denied. The Envisalink will close the connection if the client closes its
// side.
//
// To initiate a connection, the application must first start a session by
// establishing a TCP socket. Once established the TPI will send a 5053 command
// (See section 3.0 for a detailed description of the protocol) requesting a
// session password. The client should then, within 10 seconds, send 005 login
// request. The 005 command contains the password which is the same password to
// log into the Envisalink's local web page. Upon successful login, the
// Envisalink's TPI will respond with the session status command, 505, and
// whether the password was accepted or rejected. If a password is not received
// within 10 seconds, the TPI will issue a 5052 command and close the TCP
// socket. The socket will also be closed if the password fails. Once the
// password is accepted, the session is created and will continue until the TCP
// connection is dropped.
//
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
	log.Println("->", cmd)
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
				return fmt.Errorf("unknown system error %s", resp.Data)
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
	log.Println("<-", *cmd)
	switch cmd.Code {
	case CommandAck, CommandCommandError, CommandSystemError:
		select {
		case c.response <- *cmd:
		default:
		}
	case CommandLoginStatus:
		switch cmd.Data[0] {
		// 0 = Password provided was incorrect
		// 2 = Time out. You did not send a password within 10 seconds.
		case '0', '2':
			c.Disconnect()
		// 1 = Password Correct, session established
		case '1':
			c.Status()
		// 3 = Request for password, sent after socket setup
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
	case CommandTroubleOff:
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
