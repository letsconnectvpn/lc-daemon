PREFIX=/usr/local

.PHONY: fmt test clean install

_bin/vpn-daemon: vpn-daemon/main.go
	go build -o $@ vpn-daemon/main.go

fmt:
	gofmt -s -w vpn-daemon/*.go

test:
	go test vpn-daemon/*.go

clean:
	rm -f _bin/*

install: _bin/vpn-daemon
	install -D _bin/vpn-daemon $(DESTDIR)$(PREFIX)/sbin/vpn-daemon
