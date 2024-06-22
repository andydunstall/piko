# Install

Piko consists of a single `piko` binary for both the server and agent, using
`piko server` and `piko agent` respectively.

## Download the binary

Visit the [releases](https://github.com/andydunstall/piko/releases) page (under
'Assets') to download the Piko binary for your architecture and host.

For convenience you can rename the downloaded binary to `piko` and place it
into your `PATH`, such as `/usr/bin/piko` or `/usr/local/bin/piko`.

Run `piko -h` to verify the installation was successful.

## Build from source

Building Piko from source requires [Go](https://golang.org/doc/install) 1.22 or
higher.

Start by cloning the Piko repository from GitHub:
```
git clone https://github.com/andydunstall/piko.git
cd piko
```

Then build the `piko` binary with `make piko`, which outputs the binary to
`bin/piko`.

You can also build the Docker image with `make image`.

Run `piko -h` to verify the installation was successful.

## Docker

Visit the [packages](https://github.com/andydunstall/piko/pkgs/container/piko)
page to see the available Docker images. Each image is tagged with `latest` and
the Piko version:
```
# Latest version.
docker pull ghcr.io/andydunstall/piko:latest

# v0.5.0
docker pull ghcr.io/andydunstall/piko:v0.5.0
```

Run `docker run ghcr.io/andydunstall/piko:latest` to verify the installation
was successful.
