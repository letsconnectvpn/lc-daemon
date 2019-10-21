_bin/lc-daemon: lc-daemon/main.go
	go build -o $@ lc-daemon/main.go

fmt:
	gofmt -s -w lc-daemon/*.go

test:
	go test lc-daemon/*.go

clean:
	rm -f _bin/*
