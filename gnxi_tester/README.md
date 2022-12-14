# gNXI Test Client

A CLI tool for orchestrating tests against a gNXI target.

## How it works

- `gnxi_tester` communicates with you local docker socket via the Docker SDK. 
- It will read `~/.gnxi.yml` which defines which service binaries will be put in lightweight containers on the host system as well as a set of commands and desired outputs.
- It will execute the predefined commands for each binary, passing in files and and user input when necessary
- If all tests pass, it will notify the user and exit gracefully.

## Web UI

![](assets/web_ui.png?raw=true)

### Dependencies
- `docker`
- `docker-compose`

### Running

We have provided a shell script to both install and run the Web UI and API. 

Then run this command:
```sh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/google/gnxi/master/gnxi_tester/web.sh)"
```

It will install gNxI in `./gnxi`. You can subsequently launch the API and Web UI by:
```sh
cd ./gnxi/gnxi_tester && docker-compose up -d
```

### Development
If you have cloned the repository, in order to run the development environment for both the Web UI and API, 
you have to run the following command in this directory:
```sh
docker-compose up -f docker-compose.dev.yml
```

## CLI

### Installing

```
go get github.com/google/gnxi/gnxi_tester
go install github.com/google/gnxi/gnxi_tester
```

### Running

Tests are provided in `~/.gnxi.yml`. By default, the following processes & clients will be run: 
- `provision`
- `gnoi_os`
- `gnoi_cert`
- `gnoi_reset`

For example, the auto generated `~/.gnxi.yml` will look like:
```yml
docker:
  build: golang:1.14-alpine
  runtime: alpine:latest
order:
- gnoi_os
- gnoi_cert
- gnoi_reset
tests:
  gnoi_cert:
  - args:
      op: get
    doesntwant: ""
    mustfail: false
    name: Get certs
    prompt: []
    wait: 10
    wants: GetCertificates:\n{\w*
    [...]
  gnoi_os:
  - args:
      op: install
      os: '&<os>'
      version: '&<os_version>' # escaped variables the user will be prompted for
    doesntwant: ""
    mustfail: false
    name: Compatible OS with Good Hash Install
    prompt:
    - os_version
    - os
    wait: 0
    wants: ^$
    [...]
  gnoi_reset:
  - args: {}
    doesntwant: ""
    mustfail: false
    name: Resetting a Target Successfully
    prompt: []
    wait: 0
    wants: ^$
  provision:
  - args:
      cert_id: '&<cert_id>'
      op: provision
    doesntwant: ""
    mustfail: false
    name: Provision Bootstrapping Target
    prompt:
    - cert_id
    wait: 0
    wants: Install success
```

If `[test_names]` are provided, only those tests are ran.
```
gnxi_tester run [test_names] \ 
--ca certs/ca.crt \
--ca_key certs/ca.key \
--target_name target.com \
--target_address localhost:9339 \
--files "os:/path/to/image other_file:/path_to_file"
```

### Files required by service
Files to be passed in the `--files` flag:
#### `gnoi_os`
- `os`: Path to OS file used in `install` operation.
- `new_os`: Path to another OS file used in `install_another_os` operation.

