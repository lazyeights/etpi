package main

import (
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
	"github.com/urfave/cli"
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

func main() {
	cli.HelpFlag = cli.BoolFlag{
		Name:  "help",
		Usage: "show help",
	}
	app := cli.NewApp()
	app.Name = "etpid"
	app.Usage = "EnvisaLink Third Party Interface (TPI) gateway"
	app.Version = version
	app.Commands = []cli.Command{
		{
			Name:   "run",
			Action: handleRun,
			Usage:  "start an EvisaLink TPI gateway server",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "pwd, p",
					Usage:       "Password to log into the Envisalink's local web page",
					Value:       "user",
					Destination: &pwd,
				},
				cli.StringFlag{
					Name:        "code, c",
					Usage:       "User code that will be supplied to the security panel (e.g., to arm/disarm)",
					Value:       "12345",
					Destination: &code,
				},
				cli.StringFlag{
					Name:        "host, h",
					Usage:       "Envisalink 4 IP address",
					Value:       "localhost:4025",
					Destination: &etpiAddr,
				},
				cli.DurationFlag{
					Name:        "zones, z",
					Usage:       "Number of zones to enable",
					Value:       10 * time.Minute,
					Destination: &pollDuration,
				},
				cli.DurationFlag{
					Name:        "poll",
					Usage:       "Polling status frequency",
					Value:       10 * time.Minute,
					Destination: &pollDuration,
				},
			},
		},
	}

	app.Run(os.Args)

}

func handleRun(c *cli.Context) error {

	// Setup for SIGINT or SIGTERM.
	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	// Redirect log to STDOUT
	log.SetOutput(os.Stdout)

	// Connect to EnvisaLink panel
	panel = etpi.NewPanel()
	panel.OnPartitionEvent(handlePartition)
	log.Println("Connecting to Envisalink connection to security panel at", etpiAddr)
	if err := panel.Connect(etpiAddr, pwd, code); err != nil {
		log.Println("error: could not connect to Envisalink at", err)
		os.Exit(1)
	}
	defer panel.Disconnect()
	status := panel.Status()
	handlePartition(1, status.Partition[0])

	// Setup HomeKit accessory
	acc = &SecuritySystem{
		Accessory: accessory.New(accessory.Info{
			Name:         "EnvisaLink",
			SerialNumber: etpiAddr,
			Manufacturer: "EyezOn",
			Model:        "EnvisaLink3/4",
		}, accessory.TypeSecuritySystem),
		Security: service.NewSecuritySystem(),
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

	log.Println("HomeKit pin:", config.Pin)

	go t.Start()

	log.Println("Envisalink connection to security panel online")
	log.Println("Polling for panel status every", pollDuration)
	log.Println("Hit Ctrl+C to terminate")
	timer := time.NewTimer(pollDuration)
	for {
		select {
		case <-sigch:
			return nil
		case <-timer.C:
			log.Println("Polling panel status...")
			if err := panel.Poll(); err != nil {
				log.Println("error:", err)
				os.Exit(1)
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
	log.Println("SecuritySystemTargetState.OnValueRemoteUpdate:", state)
	switch state {
	case characteristic.SecuritySystemTargetStateStayArm,
		characteristic.SecuritySystemTargetStateNightArm:
		if err := panel.Arm(1, etpi.ArmStay); err != nil {
			log.Println("error:", err)
		}
	case characteristic.SecuritySystemTargetStateAwayArm:
		if err := panel.Arm(1, etpi.ArmAway); err != nil {
			log.Println("error:", err)
		}
	case characteristic.SecuritySystemTargetStateDisarm:
		if err := panel.Disarm(1); err != nil {
			log.Println("error:", err)
		}
	}
}
