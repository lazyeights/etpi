## Deployment to a Raspberry Pi 3

> NOTE: The EnvisaLink interface does not communicate over a secure channel such as 

It is more convenient to cross compile to the Raspberry Pi platform hardware rather than installing go on the Pi and compiling locally on that machine. Cross compiling reduces wear and tear on the Pi's SD card and is much, much quicker. To cross compile to the Raspbery Pi 3's ARM architecture:
```
$ cd etpid
$ env GOOS=linux GOARCH=arm GOARM=7 go build -v -o ./etpid-linux-arm7
```

This creates the executable `etpid-linux-arm7`. 

Alternatively, using the `GOARM=5` setting tells the compiler to use a legacy ARM architecture for older Pis that do not have hardware floating point support. It may not be necessary on newer models of Raspberry Pi. 

For more information on cross compiling Go to ARM systems, see [https://github.com/golang/go/wiki/GoArm](https://github.com/golang/go/wiki/GoArm).

The `etpid` command must be installed in a path where it has write access as it needs to create a "db" subfolder to maintain storage for the HomeKit interface. If you have not prepared the destination location for the Pi, open a secure shell to the Raspberry Pi and create it.
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
$ scp etpid-linux-arm7 pi@pi.local:etpi/etpid-linux-arm7
```

## Setting up a reliable service on the Raspberry Pi

Once you test the utilities you will want to ensure it launches anytime the Pi reboots. There are many ways to do this. I like [supervisord](http://www.http://supervisord.org). First install the package onto the Pi.

```
sudo apt-get install supervisor
```

Add the following lines to the configuration file. The default location from the Raspian package is at `/etc/supervisor/supervisord.conf`

```
[program:etpid]
command=/usr/home/pi/etpi/etpid-linux-arm7 -p 192.168.1.100:4025
user=pi
directory=/user/home/pi/etpi

[program:mqttetpi]
command=/usr/home/pi/etpi/mqttetpi-linux-arm5 -mqtt localhost:1883 -etpi -partitions 1 -zones 5 -pwd "user" -code "12345"
user=pi
```

Note above that you will need to provide the correct addresses for the EnvisaLink TPI service. You will need to provide the correct Envisalink password (default is "user") and a proper user code for your alarm panel.

`supervisord` can be controlled using a command line utility. You'll need to reload it to start the processes you just added to the configuration.

```
pi@pi:~ $ sudo supervisorctl
supervisor> avail
etpid                        in use    auto      999:999
supervisor> reload
Really restart the remote supervisord process y/N? 
supervisor> status
etpid                      RUNNING    pid 5259, uptime 0:01:58
```

`supervisord` also has a web interface that can be enabled by adding the following to `/etc/supervisor/supervisord.conf`:

```
[inet_http_server]
port = :9001
```


