## Deployment to a Raspberry Pi

It is more convenient to cross compile to the Raspberry Pi platform hardware rather than installing go on the Pi and compiling locally on that machine. Cross compiling reduces wear and tear on the Pi's SD card and is much, much quicker. To cross compile to the Raspbery Pi's ARM architecture:
```
$ cd mqttetpi
$ env GOOS=linux GOARCH=arm GOARM=5 go build -v -o ./mqttetpi-linux-arm5
```

This creates two executable `mqttetpi-linux-arm5`. Repeat for the `hkmqtt` utility to generate `hkmqtt-linux-arm5`.

Note that the `GOARM=5` setting tells the compiler to use a legacy ARM architecture for older Pis that do not have hardware floating point support. It may not be necessary on newer models of Raspberry Pi. 

For more information on cross compiling Go to ARM systems, see [https://github.com/golang/go/wiki/GoArm](https://github.com/golang/go/wiki/GoArm).

If you have not prepared the destination location for the Pi, open a secure shell to the Raspberry Pi and create it.
(Note: replace `pi@pi.local` with the appropriate `<user account>:<IP address>` for your machine).

```
$ ssh pi@pi.local

Debian GNU/Linux comes with ABSOLUTELY NO WARRANTY, to the extent
permitted by applicable law.
Last login: Mon Feb 15 17:17:15 2016
pi@pi:~ $ mkdir -p ~/etpi
```

You can then deploy the executable to the Raspberry Pi with the secure copy command:

```
$ cd ..
$ scp mqttetpi-linux-arm5 pi@pi.local:etpi/mqttetpi-linux-arm5
```

Repeat for `hkmqtt-linux-arm5`.

## Set up an MQTT broker

You must point the utilities to an MQTT broker. Although there are some cloud services that provide this, I want to keep mine on my local network so I set one up on the Raspberry Pi. I like [mosquitto](mosquitto.org).

See [Installing Mosquitto on a Raspberry Pi](http://jpmens.net/2013/09/01/installing-mosquitto-on-a-raspberry-pi/) for instructions.

## Setting up a reliable service on the Raspberry Pi

Once you test the utilities you will want to ensure it launches anytime the Pi reboots. There are many ways to do this. I like [supervisord](http://www.http://supervisord.org). First install the package onto the Pi.

```
sudo apt-get install supervisor
```

Add the following lines to the configuration file. The default location from the Raspian package is at `/etc/supervisor/supervisord.conf`

```
[program:hkmqtt]
command=/usr/home/pi/etpi/hkmqtt-linux-arm5 -mqtt localhost:1883
user=pi

[program:mqttetpi]
command=/usr/home/pi/etpi/mqttetpi-linux-arm5 -mqtt localhost:1883 -etpi -partitions 1 -zones 5 -pwd "user" -code "12345"
user=pi
```

Note above that you will need to provide the correct addresses for the MQTT and Envisalink services. You will also need to provide the number of zones and partitions in your system, as well as the correct Envisalink password (default is "user") and a user code for your alarm panel.

`supervisord` can be controlled using a command line utility. You'll need to reload it to start the processes you just added to the configuration.

```
pi@pi:~ $ sudo supervisorctl
supervisor> avail
hkmqtt                        in use    auto      999:999
mqttetpi                        in use    auto      999:999
supervisor> reload
Really restart the remote supervisord process y/N? 
supervisor> status
hkmqtt                        RUNNING    pid 5258, uptime 0:01:58
mqttetpi                      RUNNING    pid 5259, uptime 0:01:58
```


