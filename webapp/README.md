SCIONLab Go Static Tester
=========================

## Webapp Setup

Webapp is a Go application that will serve up a static web portal to make it easy to
experiment with SCIONLab test apps on a virtual machine.


### Local Infrastructure

To run the Go Web UI at default localhost 127.0.0.1:8000 run:

```shell
go run webapp.go
```

### SCIONLab Virtual Machine

Using vagrant, make sure to edit your `vagrantfile` to provision the additional port
for the Go web server by adding this line for port 8080 (for example, just choose any forwarding
port not already in use by vagrant):

```
config.vm.network "forwarded_port", guest: 8080, host: 8080, protocol: "tcp"
```

To run the Go Web UI at a specific address (-a) and port (-p) like 0.0.0.0:8080 for a SCIONLabVM use:

```shell
go run webapp.go -a 0.0.0.0 -p 8080 -r .
```

Now, open a web browser at http://127.0.0.1:8080, to begin.


## Webapp Features

This Go web server wraps several SCION test client apps and provides an interface
for any text and/or image output received.
[SCIONLab Apps](http://github.com/perrig/scionlab) are on Github.

Two functional client/server tests are included to test the networks without needing
specific sensor or camera hardware, `imagetest` and `statstest`.

### File System Browser

The File System Browser button on the front page will allow you to navigate and serve any
files on the SCIONLab node from the root (-r) directory you specified (if any) when
starting webapp.go.

### imagefetcher, sensorfetcher, bwtester

Supported client applications include `imagefetcher`, `sensorfetcher`, and `bwtester`.
For best results, ensure the desired server-side apps are running and connected to
the SCION network first. Instructions to setup the servers are
[here](https://github.com/perrig/SCIONLab/blob/master/README.md).
The web interface launched above can be used to run the client-side apps.

### statstest

This hardware-independent test will echo some remote machine stats from the Python script
`local-stats.py`, which is piped to the server for transmission to clients.
On your remote SCION server node run (substituting your own address parameters):

```shell
python3 local-stats.py | go run stats-test-server.go -s 1-15,[127.0.0.5]:35555
```

Now, from your webapp browser interface running on your virtual client SCION node,
you can enter both client and server addresses and ask the client for remote stats.

![Webapp Stats Test](static/img/statstest.png?raw=true "Webapp Stats Test")


### imagetest

This hardware-independent test will generate an image with some remote machine stats from
the Go app `local-image.go`, which will be saved locally for transmission to clients.

You may need golang.org's image package first:

```shell
go get golang.org/x/image
```

On your remote SCION server node run (substituting your own address parameters):

```shell
go run local-image.go | go run img-test-server.go -s 1-18,[127.0.0.8]:38887
```

Now, from your webapp browser interface running on your virtual client SCION node,
you can enter both client and server addresses and ask the client for the most
recently generated remote image.

![Webapp Image Test](static/img/imagetest.png?raw=true "Webapp Image Test")
