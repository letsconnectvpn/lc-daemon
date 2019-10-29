_bin/vpn-daemon: vpn-daemon/main.go
	go build -o $@ vpn-daemon/main.go

fmt:
	gofmt -s -w vpn-daemon/*.go

test:
	go test vpn-daemon/*.go

clean:
	rm -f _bin/*
