## Deployment to a Raspberry Pi

It is more convenient to cross compile to the Raspberry Pi platform hardware rather than installing go on the Pi and compiling locally on that machine. Cross compiling reduces wear and tear on the Pi's SD card and is much, much quicker. To cross compile to the Raspbery Pi's ARM architecture:
```
env GOOS=linux GOARCH=arm GOARM=5 go build -v -o ./mqttetpi-linux-arm5
```

This creates an executable `mqttetpi-linux-arm5`. 

Note that the `GOARM=5` setting tells the compiler to use a legacy ARM architecture for older Pis that do not have hardware floating point support. It may not be necessary on newer models of Raspberry Pi. 

For more information on cross compiling Go to ARM systems, see https://github.com/golang/go/wiki/GoArm.

If you have not prepared the destination location for the Pi, open a secure shell to the Raspberry Pi and create it.
(Note: replace `pi@pi.local` with the appropriate `<user account>:<IP address>` for your machine).

```
ssh pi@pi.local

Debian GNU/Linux comes with ABSOLUTELY NO WARRANTY, to the extent
permitted by applicable law.
Last login: Mon Feb 15 17:17:15 2016
pi@pi:~ $ mkdir -p ~/etpi
```

You can then deploy the executable to the Raspberry Pi with the secure copy command:

```
scp ./mqttetpi-linux-arm5 pi@pi.local:etpi/mqttetpi-linux-arm5
```