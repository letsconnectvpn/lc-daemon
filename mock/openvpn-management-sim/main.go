package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

//common name, real address, internal ipv4, internal ipv6
var user0 = "edu,95.196.100.50:63410,10.0.0.2,f385:1abd:fda0:a9b7:7dda:a7a7:e982:8fc1,random"
var user1 = "vpn,95.196.100.51:340,10.0.0.3,f385:1abd:fda0:a9b7:7dda:a7a7:e982:8fc2,random"
var user2 = "net,95.196.100.52:62410,10.0.0.4,f385:1abd:fda0:a9b7:7dda:a7a7:e982:8fc3,random"

//3 users array
var connectedUsers = []string{
	user0, user1, user2,
}

func main() {
	// OpenVPN Management Interface Emulator #1
	address := "127.0.0.1:11940"
	lnManagement1, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(address + ": " + err.Error())
	} else {
		go handleManagementInterface(lnManagement1)
	}

	// OpenVPN Management Interface Emulator #2
	address = "127.0.0.1:11941"
	lnManagement2, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(address + ": " + err.Error())
	} else {
		go handleManagementInterface(lnManagement2)
	}

	// OpenVPN Management Interface Emulator #3
	address = "127.0.0.1:9090"
	lnManagement3, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(address + ": " + err.Error())
	} else {
		go handleManagementInterface(lnManagement3)
	}

	// OpenVPN Management Interface Emulator #4
	address = "127.0.0.1:9091"
	lnManagement4, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(address + ": " + err.Error())
	} else {
		go handleManagementInterface(lnManagement4)
	}

	// Listener will accept all connections, but if data is send, it will close the connection
	address = "127.0.0.1:9092"
	lnReject, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(address + ": " + err.Error())
	} else {
		go handleRejectListener(lnReject)
	}

	// Listener will accept all connections, keep connection alive and do nothing
	address = "127.0.0.1:9093"
	lnIdle, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println(address + ": " + err.Error())
	} else {
		go handleIdleListener(lnIdle)
	}

	time.Sleep(time.Minute * 60)

}

func handleManagementInterface(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		handleConnectionManagementInterface(conn)
	}
}

func handleRejectListener(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		//assume only one connection per time, otherwise add go before the function
		handleConnectionReject(conn)
	}
}

func handleIdleListener(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		//assume only one connection per time, otherwise add go before the function
		handleConnectionIdle(conn)
	}
}

func handleConnectionManagementInterface(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		if 0 == strings.Index(msg, "status 2") {
			//Normal output
			writer.WriteString(fmt.Sprintf(">LOG: status 2....\n"))
			writer.WriteString(fmt.Sprintf(">LOG: log was left on, should not matter\n"))
			writer.WriteString(fmt.Sprintf("Garbage information\n"))
			writer.WriteString(fmt.Sprintf("HEADER,CLIENT_LIST,.....\n"))

			//3 users for each management port
			for _, user := range connectedUsers {
				writer.WriteString(fmt.Sprintf("CLIENT_LIST,%s\n", user))
			}

			//Continue with output
			writer.WriteString(fmt.Sprintf("HEADER,ROUTING_TABLE,...\n"))
			writer.WriteString(fmt.Sprintf("ROUTING_TABLE,....\n"))
			writer.WriteString(fmt.Sprintf("ROUTING_TABLE,....\n"))
			writer.WriteString(fmt.Sprintf("ROUTING_TABLE,....\n"))
			writer.WriteString(fmt.Sprintf("GLOBAL_STATS\n"))
			writer.WriteString(fmt.Sprintf("END\n"))
			writer.Flush()
			continue
		}

		if 0 == strings.Index(msg, "kill") {

			rand.Seed(time.Now().UnixNano())
			nmber := rand.Intn(2)
			if nmber == 0 {
				writer.WriteString(fmt.Sprintf("SUCCESS: common name 'foo' found, %d client(s) killed\n", rand.Intn(1000)))
				writer.Flush()
			} else {
				writer.WriteString(fmt.Sprintf("ERROR: common name 'foo' not found\n"))
				writer.Flush()
			}

			continue
		}

		if 0 == strings.Index(msg, "quit") || 0 == strings.Index(msg, "exit") {
			return
		}
	}
}

func handleConnectionReject(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		return
	}
}

func handleConnectionIdle(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			return
		}
	}
}
