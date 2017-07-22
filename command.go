package etpi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	CommandPoll                         = "000"
	CommandStatusReport                 = "001"
	CommandLogin                        = "005"
	CommandSetTimeAndDate               = "010"
	CommandPartitionArmControlAway      = "030"
	CommandPartitionArmControlStay      = "031"
	CommandPartitionArmControlZeroEntry = "032"
	CommandPartitionDisarmControl       = "040"
	CommandCode                         = "200"
	CommandAck                          = "500"
	CommandCommandError                 = "501"
	CommandSystemError                  = "502"
	CommandLoginStatus                  = "505"
	CommandKeypadLed                    = "510"
	CommandZoneAlarm                    = "601"
	CommandZoneTamper                   = "603"
	CommandZoneFault                    = "605"
	CommandZoneOpen                     = "609"
	CommandZoneRestored                 = "610"
	CommandPartitionReady               = "650"
	CommandPartitionNotReady            = "651"
	CommandPartitionArmed               = "652"
	CommandPartitionDisarmed            = "655"
	CommandPartitionAlarm               = "654"
	CommandPartitionExitDelay           = "656"
	CommandPartitionEntryDelay          = "657"
	CommandPartitionBusy                = "673"
	CommandPartitionSpecialClosing      = "701"
	CommandTroubleOn                    = "840"
	CommandTroubleOff                   = "841"
	CommandCodeRequired                 = "900"
)

type Command struct {
	Code string
	Data string
}

func (c Command) String() string {
	var str string
	switch c.Code {
	case "000":
		str = "Poll"
	case "001":
		str = "StatusReport"
	case "005":
		str = "Login"
	case "010":
		str = "SetTimeAndDate"
	case "030":
		str = "PartitionArmControlAway"
	case "031":
		str = "PartitionArmControlStay"
	case "032":
		str = "PartitionArmControlZeroEntry"
	case "040":
		str = "PartitionDisarmControl"
	case "200":
		str = "Code"
	case "500":
		str = "ACK"
	case "501":
		str = "Error"
	case "502":
		str = "SystemError"
	case "505":
		str = "LoginStatus"
	case "510":
		str = "KeypadLed"
	case "601":
		str = "ZoneAlarm"
	case "603":
		str = "ZoneTamper"
	case "605":
		str = "ZoneFault"
	case "609":
		str = "ZoneOpen"
	case "610":
		str = "ZoneRestored"
	case "650":
		str = "PartitionReady"
	case "651":
		str = "PartitionNotReady"
	case "652":
		str = "PartitionArmed"
	case "655":
		str = "PartitionDisarmed"
	case "654":
		str = "PartitionAlarm"
	case "656":
		str = "PartitionExitDelay"
	case "657":
		str = "PartitionEntryDelay"
	case "673":
		str = "PartitionBusy"
	case "701":
		str = "PartitionSpecialClosing"
	case "840":
		str = "TroubleOn"
	case "841":
		str = "TroubleOff"
	case "900":
		str = "CodeRequired"
	default:
		str = "UNKNOWN"
	}
	return fmt.Sprintf("{%s : %s : %s}", c.Code, str, c.Data)
}

func NewCommandFromBytes(p []byte) (*Command, error) {
	if len(p) < 7 {
		return nil, errors.New("invalid command")
	}
	var tmp byte
	for _, b := range p[:len(p)-4] {
		tmp += b
	}
	if fmt.Sprintf("%02X", tmp) != string(p[len(p)-4:len(p)-2]) {
		return nil, errors.New("invalid checksum")
	}
	code := string(p[:3])
	data := string(p[3 : len(p)-4])
	cmd := &Command{Code: code, Data: data}
	return cmd, nil
}

func (cmd Command) WriteTo(w io.Writer) (int64, error) {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "%s%s", cmd.Code, cmd.Data)
	var chksum byte
	for _, b := range buf.Bytes() {
		chksum += b
	}
	fmt.Fprintf(buf, "%02X\r\n", chksum)
	return buf.WriteTo(w)
}
