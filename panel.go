package etpi

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
)

// Panel is the primary interface for connecting to an alarm panel with the Envisalink TPI module.
//
// Usage:
//
//     import "github.com/lazyeights/etpi"
//
//     etpiAddr := "192.168.1.24:4025"
//     pwd := "user"
//     code := "12345"
//
//     panel := etpi.NewPanel()
//     panel.OnZoneEvent(handleZone)
//     panel.OnPartitionEvent(handlePartition)
//     panel.OnKeypadEvent(handleKeypad)
//
//     if err := panel.Connect(etpiAddr, pwd, code); err != nil {
//     	panic("could not connect to Envisalink")
//     }
//     defer panel.Disconnect()
//
//     status := panel.Status()
//     fmt.Printf("%+v\n", status)
//
type Panel interface {

	// Connect opens the connection to the Envisalink panel, logs in using
	// the supplied password, and upon a successful connection will set the
	// correct date/time of the alarm system.
	Connect(host string, pwd string, code string) error

	// Disconnect closes the connection to the Envisalink panel.
	Disconnect()

	// Arm attempts to arm a partition according to the supplied mode
	// (e.g., Stay, Away).
	Arm(partition int, mode ArmMode) error

	// Disarm attempts to disarm a partition.
	Disarm(partition int) error

	// SetTime sets the time for the alarm panel.
	SetTime(time.Time) error

	// OnPartitionEvent sets a calledback for whenever a partition event
	// occurs.
	OnPartitionEvent(func(int, PartitionStatus))

	// OnZoneEvent sets a calledback for whenever a zone event occurs.
	OnZoneEvent(func(int, ZoneStatus))

	// OnKeypadEvent sets a callback for whenever a keypad event occurs.
	OnKeypadEvent(func(KeypadStatus))

	// Status returns the current partition, zone, and keypad status.
	Status() *PanelStatus

	// Poll queries the Envisalink module to send its latest update.
	Poll() error
}

type ArmMode int

const (
	ArmAway = iota + 1
	ArmStay
	ArmNoEntryDelay
)

type PanelStatus struct {
	Zone      []ZoneStatus
	Partition []PartitionStatus
	Keypad    KeypadStatus
}

type ZoneStatus int

const UnknownStatus = 0

const (
	ZoneStatusAlarm = iota + 1
	ZoneStatusTamper
	ZoneStatusFault
	ZoneStatusOpen
	ZoneStatusRestored
)

func (z ZoneStatus) String() string {
	switch z {
	case ZoneStatusAlarm:
		return "ALARM"
	case ZoneStatusTamper:
		return "TAMPER"
	case ZoneStatusFault:
		return "FAULT"
	case ZoneStatusOpen:
		return "OPEN"
	case ZoneStatusRestored:
		return "RESTORED"
	default:
		return "UNKNOWN"
	}
}

type PartitionStatus int

const (
	PartitionStatusReady = iota + 1
	PartitionStatusNotReady
	PartitionStatusArmedAway
	PartitionStatusArmedStay
	PartitionStatusArmedZeroEntryAway
	PartitionStatusArmedZeroEntryStay
	PartitionStatusAlarm
	PartitionStatusDisarmed
	PartitionStatusExitDelay
	PartitionStatusEntryDelay
	PartitionStatusFailedToArm
	PartitionStatusBusy
)

func (p PartitionStatus) String() string {
	switch p {
	case PartitionStatusReady:
		return "DISARMED_READY"
	case PartitionStatusNotReady:
		return "DISARMED_NOT_READY"
	case PartitionStatusArmedAway:
		return "ARMED_AWAY"
	case PartitionStatusArmedStay:
		return "ARMED_STAY"
	case PartitionStatusArmedZeroEntryAway:
		return "ARMED_ZERO_ENTRY_AWAY"
	case PartitionStatusArmedZeroEntryStay:
		return "ARMED_ZERO_ENTRY_STAY"
	case PartitionStatusAlarm:
		return "ALARM"
	case PartitionStatusDisarmed:
		return "DISARMED"
	case PartitionStatusExitDelay:
		return "EXIT_DELAY"
	case PartitionStatusEntryDelay:
		return "ENTRY_DELAY"
	case PartitionStatusFailedToArm:
		return "FAILED_TO_ARM"
	case PartitionStatusBusy:
		return "BUSY"
	default:
		return "UNKNOWN"
	}
}

type KeypadStatus struct {
	Backlight bool
	Fire      bool
	Program   bool
	Trouble   bool
	Bypass    bool
	Memory    bool
	Armed     bool
	Ready     bool
}

type panel struct {
	conn        Client
	status      *PanelStatus
	code        string
	wait        chan struct{}
	ready       bool
	onZone      func(int, ZoneStatus)
	onPartition func(int, PartitionStatus)
	onKeypad    func(KeypadStatus)
}

// NewPanel creates a new Panel interface.
func NewPanel() Panel {
	status := &PanelStatus{
		Zone:      make([]ZoneStatus, 64),
		Partition: make([]PartitionStatus, 8),
	}
	return &panel{status: status}
}

func (p *panel) Connect(host string, pwd string, code string) error {
	conn := NewClient()

	conn.HandleZoneState(p.handleZone)
	conn.HandlePartitionState(p.handlePartition)
	conn.HandleKeypadState(p.handleKeypad)

	if err := conn.Connect(host, pwd, code); err != nil {
		return err
	}
	p.conn = conn
	p.code = code
	p.wait = make(chan struct{})

	<-p.wait

	t := time.Now()
	log.Println("setting system time to", t.Format(time.Stamp))
	if err := p.SetTime(t); err != nil {
		log.Println("error:", err)
	}

	p.ready = true

	return nil
}

func (p *panel) Disconnect() {
	if p.conn != nil {
		p.conn.Disconnect()
		p.conn = nil
	}
}

func (p *panel) Status() *PanelStatus {
	return p.status
}

func (p *panel) handleZone(zone int, status ZoneStatus) {
	p.status.Zone[zone-1] = status
	if p.ready && p.onZone != nil {
		p.onZone(zone, status)
	}
}

func (p *panel) handlePartition(partition int, status PartitionStatus) {
	p.status.Partition[partition-1] = status
	if p.ready && p.onPartition != nil {
		p.onPartition(partition, status)
	}
}

func (p *panel) handleKeypad(status KeypadStatus) {
	p.status.Keypad = status
	if p.ready && p.onKeypad != nil {
		p.onKeypad(status)
	}
	select {
	case p.wait <- struct{}{}:
	default:
	}
}

func (p *panel) SetTime(t time.Time) error {
	data := t.Format("1504010206")
	return p.conn.Send(Command{Code: CommandSetTimeAndDate, Data: data})
}

func (p *panel) Arm(partition int, mode ArmMode) error {
	if partition < 1 || partition > 8 {
		return errors.New("invalid partition")
	}
	data := strconv.Itoa(partition)
	switch mode {
	case ArmAway:
		p.conn.Send(Command{Code: CommandPartitionArmControlAway, Data: data})
	case ArmStay:
		p.conn.Send(Command{Code: CommandPartitionArmControlStay, Data: data})
	case ArmNoEntryDelay:
		p.conn.Send(Command{Code: CommandPartitionArmControlZeroEntry, Data: data})
	}

	return nil
}

func (p *panel) Disarm(partition int) error {
	if partition < 1 || partition > 8 {
		return errors.New("invalid partition")
	}
	data := fmt.Sprintf("%d%s", partition, p.code)
	return p.conn.Send(Command{Code: CommandPartitionDisarmControl, Data: data})
}

func (p *panel) OnZoneEvent(f func(int, ZoneStatus)) {
	p.onZone = f
}

func (p *panel) OnPartitionEvent(f func(int, PartitionStatus)) {
	p.onPartition = f
}

func (p *panel) OnKeypadEvent(f func(KeypadStatus)) {
	p.onKeypad = f
}

func (p *panel) Poll() error {
	return p.conn.Status()
}
