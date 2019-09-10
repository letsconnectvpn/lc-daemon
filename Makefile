_bin/lc-daemon: lc-daemon/main.go
	go build -o $@ lc-daemon/main.go

fmt:
	gofmt -s -w lc-daemon/main.go

clean:
	rm -f _bin/*
