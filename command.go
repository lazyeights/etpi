package etpi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

type Command struct {
	Code string
	Data string
}

const (
	CommandPoll                         = "000"
	CommandStatusReport                 = "001"
	CommandLogin                        = "005"
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
	CommandPartitionExitDelay           = "656"
	CommandPartitionEntryDelay          = "657"
	CommandPartitionBusy                = "673"
	CommandPartitionSpecialClosing      = "701"
	CommandTroubleOn                    = "840"
	CommandTroubleOff                   = "841"
	CommandCodeRequired                 = "900"
)

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
