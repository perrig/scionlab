# scionlab

This repo contains software for supporting SCIONLab.

The repo currently contains two applications: camerapp and sensorapp. Both applications are written in Go, with some supporting code in Python. A SCION Internet connection (for instance via SCIONLab) is required to run these applications.

More information on [SCION](https://www.scion-architecture.net/), and [tutorials on how to set up SCION and SCIONLab](https://netsec-ethz.github.io/scion-tutorials/).

***

## camerapp

Camerapp contains image fetcher and server applications, using the SCION network.

### imagefetcher

To install imagefetcher:
```shell
go get github.com/perrig/scionlab/camerapp/imagefetcher
```

To use the image fetcher, you will need to express your local host's address as a SCION address (in the format `ISD-AS,[IPv4]:port`) and know the address of an image server, for instance `1-1011,[192.33.93.166]:42002`

The client address is passed with `-c` and the server address with `-s`:
```shell
imagefetcher -s 1-1011,[192.33.93.166]:42002 -c 1-1006,[10.0.2.15]:42001
```

The fetched image is then saved in the local directory.

### imageserver

To install imageserver:
```shell
go get github.com/perrig/scionlab/camerapp/imageserver
```

The `imageserver` application keeps looking for `.jpg` files in the current directory, and offers them for download to clients on the SCION network. The assumption is that the application is used in conjunction with an application that periodically writes an image to the file system. After an amount of time (currently 10 minutes), the image files are deleted to limit the amount of storage needed.

Included is a simple `paparazzi.py` application, which reads the camera image on a Raspberry Pi. The system is launched as follows:
```shell
python3 ${GOPATH}/src/github.com/perrig/scionlab/camerapp/imageserver/paparazzi.py > /dev/null &
imageserver -s 1-1011,[192.33.93.166]:42002 &
```

***

## sensorapp

Sensorapp contains fetcher and server applications for sensor readings, using the SCION network.

### sensorfetcher

To install sensorfetcher:
```shell
go get github.com/perrig/scionlab/sensorapp/sensorfetcher
```

The `sensorfetcher` application sends a 0-length SCION UDP packet to the `sensorserver` application to fetch the sensor readings. A string is returned containing all the sensor readings. To keep the application as simple as possible, no reliability is built in -- in case of packet loss, the user needs to abort and re-try. An example server is at `1-6,[192.33.93.173]:42003`, its readings can be fetched as follows (need to replace client address with actual client address, with an arbitrary free port):

```shell
sensorfetcher -s 1-6,[192.33.93.173]:42003 -c 1-1006,[10.0.2.15]:42001
```

### sensorserver

To install sensorserver:
```shell
go get github.com/perrig/scionlab/sensorapp/sensorserver
```

We use sensors from Tinkerforge, and the `sensorreader.py` Python application fetches the sensor values and writes them to `stdout`. The `sensorserver` application collects the readings, and serves them as a string to client requests. To start, we use the following command:

```shell
python3 ${GOPATH}/src/github.com/perrig/scionlab/sensorapp/sensorserver/sensorreader.py | sensorserver -s 1-6,[192.33.93.173]:42003 &
```
