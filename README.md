# EnvisaLink Alarm Server

![](https://travis-ci.org/lazyeights/etpi.svg?branch=master)

This is a libary to communicate with commercial alarm panels using the [EnvisaLink](http://www.eyezon.com) TPI interface. It currently only works with DSC alarm panels

This is currently alpha-grade software. It has only been tested with an EnvisaLink4 module. 

Documentation for the Envisalink TPI interface can be found [here](http://forum.eyez-on.com/FORUM/viewtopic.php?f=6&t=301).

## Apple HomeKit integration

The library was designed with the goal of integrating a DSC panel with Apple's HomeKit using an EnvisaLink4 module. There are more capabilities of the TPI interface that were not necessary for that goal, and have been left uncompleted so far.

![](doc/home_screenshot.png)

The repository includes one command that employ the library, `etpid`. Instructions for one way to set up a Raspberry Pi with these utilities can be found here: [Deployment to a Raspberry Pi](cmd/etpid/README.md)

### [cmd/etpid](cmd/etpid)

`etpid` utility is a bridge between the EnvisaLink panel and Apple HomeKit to implement a SecuritySystem servicee.

It has a dependency on [the HomeControl library for HomeKit by brutella](https://github.com/brutella/hc).

Once running, `etpid` advertises a new accessory named "EnvisaLink". It is paired with a manual code "32191123".

## Usage

```go
import "github.com/lazyeights/etpi"

etpiAddr := "192.168.1.24:4025"
pwd := "user"
code := "12345"

panel := etpi.NewPanel()
panel.OnZoneEvent(handleZone)
panel.OnPartitionEvent(handlePartition)
panel.OnKeypadEvent(handleKeypad)

// Connect to Envisalink. Connect will login to the Envisalink module, set the current date and time, and poll for the panel's current status.
if err := panel.Connect(etpiAddr, pwd, code); err != nil {
    panic("could not connect to Envisalink")
}
defer panel.Disconnect()

// Print the initial panel status
status := panel.Status()
fmt.Printf("%+v\n", status)
```

The library emits events for partitions, zones, and the keyboard that are handled by callback functions:

```go
func handlePartition(partition int, status etpi.PartitionStatus) {
	fmt.Println("partition=%d, status=%+v", partition, status)
    if status := etpi.PartitionStatusAlarm {
        fmt.Println("ALARM TRIGGERED!")
    }
}
```
