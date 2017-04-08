package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/service"
	"github.com/lazyeights/etpi"
)

var panel etpi.Panel

var pwd string
var code string
var etpiAddr string
var pollDuration time.Duration

type SecuritySystem struct {
	*accessory.Accessory
	Security *service.SecuritySystem
}

var acc *SecuritySystem

func init() {
	flag.StringVar(&pwd, "pwd", "user", "the password which is the same password to log into the Envisalink's local web page")
	flag.StringVar(&code, "code", "12345", "the user code that will be supplied to the security panel (e.g., to arm/disarm)")
	flag.StringVar(&etpiAddr, "etpi", "localhost:4025", "Envisalink 4 address (typically on port 4025)")
	flag.DurationVar(&pollDuration, "poll", 10*time.Minute, "polling status frequency")
}

func main() {
	flag.Parse()
	fmt.Println("etpid version", version, "type \"-h\" for help")

	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	// Setup HomeKit accessory
	info := accessory.Info{
		Name:         "EVL4 Partition 1",
		Manufacturer: "EyezOn",
		Model:        "Envisalink4",
	}

	acc = &SecuritySystem{
		Accessory: accessory.New(info, accessory.TypeAlarmSystem),
		Security:  service.NewSecuritySystem(),
	}
	acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateDisarmed)
	acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateDisarm)
	acc.Security.SecuritySystemTargetState.OnValueRemoteUpdate(updateTargetState)
	acc.AddService(acc.Security.Service)

	config := hc.Config{
		Pin:         "32191123",
		StoragePath: "db",
	}
	t, err := hc.NewIPTransport(config, acc.Accessory)
	if err != nil {
		log.Fatal(err)
	}
	defer t.Stop()

	fmt.Println("HomeKit accessory:", info.Name)
	fmt.Println("Pin:", config.Pin)

	// Connect to Envisalink
	panel = etpi.NewPanel()
	panel.OnPartitionEvent(handlePartition)
	fmt.Println("Connecting to Envisalink connection to security panel at", etpiAddr)
	if err := panel.Connect(etpiAddr, pwd, code); err != nil {
		fmt.Println("error: could not connect to Envisalink at", err)
		os.Exit(1)
	}
	defer panel.Disconnect()

	// Update initial status
	status := panel.Status()
	handlePartition(1, status.Partition[0])

	timer := time.NewTimer(pollDuration)

	fmt.Println("Envisalink connection to security panel online")
	fmt.Println("Polling for panel status every", pollDuration)
	fmt.Println("Hit Ctrl+C to terminate")
	for {
		select {
		case <-sigch:
			return
		case <-timer.C:
			fmt.Println("Polling panel status...")
			if err := panel.Poll(); err != nil {
				fmt.Println("error:", err)
			}
			timer.Reset(pollDuration)
		}
	}
}

func handlePartition(partition int, status etpi.PartitionStatus) {
	if acc == nil {
		return
	}
	if partition != 1 {
		return
	}
	switch status {
	case etpi.PartitionStatusExitDelay:
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateDisarmed)
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateAwayArm)
	case etpi.PartitionStatusReady,
		etpi.PartitionStatusDisarmed:
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateDisarmed)
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateDisarm)
	case etpi.PartitionStatusArmedAway,
		etpi.PartitionStatusArmedZeroEntryAway:
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateAwayArm)
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateAwayArm)
	case etpi.PartitionStatusArmedStay,
		etpi.PartitionStatusArmedZeroEntryStay:
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateStayArm)
		acc.Security.SecuritySystemTargetState.SetValue(characteristic.SecuritySystemTargetStateStayArm)
	case etpi.PartitionStatusAlarm:
		acc.Security.SecuritySystemCurrentState.SetValue(characteristic.SecuritySystemCurrentStateAlarmTriggered)
	}
}

func updateTargetState(state int) {
	switch state {
	case characteristic.SecuritySystemTargetStateStayArm:
		if err := panel.Arm(1, etpi.ArmStay); err != nil {
			fmt.Println("error:", err)
		}
	case characteristic.SecuritySystemTargetStateAwayArm:
		if err := panel.Arm(1, etpi.ArmAway); err != nil {
			fmt.Println("error:", err)
		}
	case characteristic.SecuritySystemTargetStateDisarm:
		if err := panel.Disarm(1); err != nil {
			fmt.Println("error:", err)
		}
	}
}
