[![Go Report Card](https://goreportcard.com/badge/github.com/pepa65/mvimap)](https://goreportcard.com/report/github.com/pepa65/mvimap)
[![GoDoc](https://godoc.org/github.com/pepa65/mvimap?status.svg)](https://godoc.org/github.com/pepa65/mvimap)
[![GitHub](https://img.shields.io/github/license/pepa65/mvimap.svg)](LICENSE)
# mvimap v0.1.0
**Move (or copy) messages from IMAP to IMAP**

* Leveraging [go-imap](https://github.com/emersion/go-imap)
* Licence: MIT
* After: https://github.com/delthas/imap-forward
* Copying messages one-time or as a daemon (using IDLE for low-latency syncing)

## Usage
### Keep moving IMAP Inbox to IMAP Inbox
`mvimap example.com:993 foo@example.com himom other.example.com:993 bar@other.example.com hidad`

### Move IMAP Inbox to IMAP Inbox Once
`mvimap --once example.com:993 foo@example.com himom other.example.com:993 bar@other.example.com hidad`

### Keep copying IMAP Inbox to IMAP Inbox
`mvimap --copy example.com:993 foo@example.com himom other.example.com:993 bar@other.example.com hidad`

### Copy IMAP Inbox to IMAP Inbox Once
`mvimap --copy --once example.com:993 foo@example.com himom other.example.com:993 bar@other.example.com hidad`

### Help
```
mvimap v0.1.0 - Move (or copy) messages from IMAP to IMAP
Usage:  mvimap [Options] FROM_SERVER FROM_USER FROM_PW TO_SERVER TO_USER TO_PW
  If FROM_SERVER or TO_SERVER does not use port 993, then append :PORT
  Options:
    --once:              Only do this once [default: keep running every minute]
    --copy:              Copy messages [default: move messages]
    --verbose:           Print verbose debug logs to stdout
    --frombox FROM_BOX:  The source folder [default: INBOX]
    --tobox TO_BOX:      The destination folder [default: INBOX]
    -h|--help:           Only show this help text
    -V|--version:        Only show the app version
```

## Install
### Go install (if Golang is installed properly)
`go install github.com/pepa65/mvimap@latest`

# Go clone/install (if Golang is installed properly)
`git clone https://github.com/pepa65/mvimap; cd mvimap; CGO_ENABLED=0 go install`

# Smaller binaries
```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w"
CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o mvimap_pi
CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags="-s -w" -o mvimap_freebsd
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o mvimap_osx
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o mvimap.exe
```

# More extreme shrinking:
`upx --best --lzma mvimap*`

# Move them to the local binary directory (if in your PATH):
`mv mvimap* ~/bin/`

# Or move to a manually managed binaries location:
`sudo mv mvimap* /usr/local/bin/`

