/*
 * Copyright (c) 2019 François Kooman <fkooman@tuxed.net>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//struct used in LIST
type connectionInfo struct {
	commonName  string
	virtualIPv4 string
	virtualIPv6 string
}

func main() {
	var listenHostPort = flag.String("listen", "localhost:41194", "IP:port to listen on")
	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()
	//Get server cert and key
	cert, err := tls.LoadX509KeyPair("./server/lc-daemon.crt", "./server/lc-daemon.key")
	if err != nil {
		fmt.Println(err)
		return
	}

	//Get CA for client auth
	clientPool := x509.NewCertPool()
	pemCA, err := ioutil.ReadFile("./ca.crt")
	if err != nil {
		fmt.Println(err)
		return
	}

	if !clientPool.AppendCertsFromPEM(pemCA) {
		fmt.Println("Unable to add CA certificate to daemon")
		return
	}

	config := &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		ClientAuth:               tls.RequireAndVerifyClientCert,
		ClientCAs:                clientPool,
		CipherSuites:             []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384},
		PreferServerCipherSuites: true}
	ln, err := tls.Listen("tcp", *listenHostPort, config)
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	intPortList := make([]int, 0)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			// unable to read string, possibly the client left
			return
		}
		if 0 == strings.Index(msg, "SET_PORTS") {
			fmt.Println("SET_PORTS")
			newPortList, err := parsePortCommand(msg)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err))
				writer.Flush()
				continue
			}

			intPortList = newPortList
			writer.WriteString(fmt.Sprintf("OK: 0\n"))
			writer.Flush()
			continue
		}

		if 0 == strings.Index(msg, "DISCONNECT") {
			fmt.Println("DISCONNECT")
			commonName, err := parseDisconnectCommand(msg)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err))
				writer.Flush()
				continue
			}

			c := make(chan int, len(intPortList))
			var wgDisc sync.WaitGroup

			for _, p := range intPortList {
				wgDisc.Add(1)
				go disconnectClient(c, p, commonName, &wgDisc)
			}

			// wait for all routines to finish...
			wgDisc.Wait()

			// close channel, we do not expect any data anymore, this is needed
			// because otherwise "range c" below is still waiting for more data on the
			// channel...
			close(c)

			// below we basically count all the "trues" in the channel populated by the
			// routines...
			clientDisconnectCount := 0
			for clientDisconnected := range c {
				clientDisconnectCount += clientDisconnected
			}

			writer.WriteString(fmt.Sprintf("OK: 1\n"))
			writer.WriteString(fmt.Sprintf("%d\n", clientDisconnectCount))
			writer.Flush()
			continue
		}

		if 0 == strings.Index(msg, "LIST") {
			fmt.Println("LIST")

			// we are not interested in the parameters (if they are entered), they are simply disregared here
			// check if the command is truly "LIST" and not "LIST*"
			if strings.Fields(msg)[0] != "LIST" {
				writer.WriteString(fmt.Sprintf("ERR: NOT_SUPPORTED\n"))
				writer.Flush()
				continue
			}

			c := make(chan []*connectionInfo, len(intPortList))
			var wgList sync.WaitGroup

			for _, p := range intPortList {
				wgList.Add(1)
				go obtainStatus(c, p, &wgList)
			}

			// wait for all routines to finish...
			wgList.Wait()

			// close channel, we do not expect any data anymore, this is needed
			// because otherwise "range c" below is still waiting for more data on the
			// channel...
			close(c)

			connectionCount := 0
			rtnConnList := ""
			for connections := range c {
				if connections != nil {
					for _, conn := range connections {
						if conn.commonName != "UNDEF" {
							connectionCount++
							rtnConnList = rtnConnList + fmt.Sprintf("%s %s %s\n", conn.commonName, conn.virtualIPv4, conn.virtualIPv6)
						}
					}
				}
			}

			writer.WriteString(fmt.Sprintf("OK: %d\n", connectionCount))
			writer.WriteString(rtnConnList)
			writer.Flush()
			continue
		}

		if 0 == strings.Index(msg, "QUIT") {
			fmt.Println("QUIT")

			// we are not interested in the parameters (if they are entered), they are simply disregared here
			// check if the command is truly "QUIT" and and not "QUIT*"
			if strings.Fields(msg)[0] != "QUIT" {
				writer.WriteString(fmt.Sprintf("ERR: NOT_SUPPORTED\n"))
				writer.Flush()
				continue
			}

			writer.WriteString(fmt.Sprintf("OK: 0\n"))
			writer.Flush()
			return
		}

		writer.WriteString(fmt.Sprintf("ERR: NOT_SUPPORTED\n"))
		writer.Flush()
	}
}

func disconnectClient(c chan int, port int, commonName string, wg *sync.WaitGroup) {
	fmt.Println(fmt.Sprintf("disconnect client on port [%d]", port))

	defer wg.Done()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second*10)
	if err != nil {
		// timeout error, port was busy with another connection
		// the port was not listening for connections
		// port refused connection
		c <- 0
		return
	}

	defer conn.Close()

	reader := bufio.NewReader(conn)

	// disconnect the client
	fmt.Fprintf(conn, fmt.Sprintf("kill %s\n", commonName))

	text, err := "", nil
	conn.SetReadDeadline(time.Now().Add(time.Second * 3))

	// read till the proper response is found, assuming interleaving does happen
	for 0 != strings.Index(text, "SUCCESS: common name") && 0 != strings.Index(text, "ERROR: common name") {
		text, err = reader.ReadString('\n')
		if err != nil {
			fmt.Printf("DISCONNECT: Port[%v] %s\n", port, err.Error())
			c <- 0
			return
		}
	}

	if 0 == strings.Index(text, "SUCCESS") {
		clientString := regexp.MustCompile(`[0-9]+`).FindString(text[strings.Index(text, ","):])
		if clientsDisconnected, err := strconv.Atoi(clientString); err == nil {
			c <- clientsDisconnected
			return
		}
	}
	c <- 0
}

func obtainStatus(c chan []*connectionInfo, port int, wg *sync.WaitGroup) {
	fmt.Println(fmt.Sprintf("Obtain status on port [%d]", port))

	defer wg.Done()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second*10)
	if err != nil {
		// timeout error, port was busy with another connection
		// the port was not listening for connections
		// port refused connection
		c <- nil
		return
	}

	defer conn.Close()

	reader := bufio.NewReader(conn)

	// send status command
	fmt.Fprintf(conn, "status 2\n")

	text, err := "", nil
	conn.SetReadDeadline(time.Now().Add(time.Second * 3))

	for 0 != strings.Index(text, "CLIENT_LIST") {
		// walk until we find CLIENT_LIST
		// exit loop if no clients are found -> if not infinite loop searching for "CLIENT_LIST"
		if 0 == strings.Index(text, "END") {
			c <- nil
			return
		}
		text, err = reader.ReadString('\n')
		if err != nil {
			fmt.Printf("LIST: Port[%v] %s\n", port, err.Error())
			c <- nil
			return
		}
	}

	connections := make([]*connectionInfo, 0)
	//can continue to iterate through the msg till END is found
	//can continue even when interleaving msgs are present
	for 0 != strings.Index(text, "END") {
		if 0 == strings.Index(text, "CLIENT_LIST") {
			strList := strings.Split(strings.TrimSpace(text), ",")
			//x[0] = "CLIENT_LIST"				x[1] = {COMMON NAME}				x[2] = {Real Address}
			//x[3] = {Virtual IPv4 Address}		x[4] = {Virtual IPv6 Address}		x[5] = {Bytes Received}
			//x[6] = {Bytes Sent}				x[7] = {Connected Since}			x[8] = {Connetected Since (time_t)}
			//x[9] = {Username}					x[10]= {Client ID}					x[11]= {Peer ID}
			newConnection := connectionInfo{strList[1], strList[3], strList[4]}
			connections = append(connections, &newConnection)
		}
		text, err = reader.ReadString('\n')
		if err != nil {
			fmt.Printf("LIST: Port[%v] %s\n", port, err.Error())
			c <- connections
			return
		}
	}
	c <- connections
}

func parsePortCommand(msg string) ([]int, error) {
	// strings.Fields() will handle/remove any whitespace in between other chars incl CRLF/LF
	portList := strings.Fields(msg)
	if portList[0] != "SET_PORTS" {
		return nil, errors.New("NOT_SUPPORTED")
	}

	if len(portList) == 1 {
		return nil, errors.New("MISSING_PARAMETER")
	}

	newPortList := make([]int, 0)
	for _, port := range portList[1:] {
		uintPort, err := strconv.ParseUint(port, 10, 16)
		if err != nil || uintPort == 0 {
			return nil, errors.New("INVALID_PARAMETER")
		}

		intPort := int(uintPort)
		i := sort.Search(len(newPortList), func(i int) bool { return newPortList[i] >= intPort })
		// only add the port if its not there yet, disregard if duplicate is found
		if i >= len(newPortList) || newPortList[i] != intPort {
			newPortList = append(newPortList, intPort)
			sort.Ints(newPortList)
		}
	}

	return newPortList, nil
}

func parseDisconnectCommand(msg string) (string, error) {
	// strings.Fields() will handle/remove any whitespace in between other chars incl CRLF/LF
	disconnectList := strings.Fields(msg)
	if disconnectList[0] != "DISCONNECT" {
		return "", errors.New("NOT_SUPPORTED")
	}

	if len(disconnectList) == 1 {
		return "", errors.New("MISSING_PARAMETER")
	}

	//parsing commonName
	validCommonName := regexp.MustCompile(`^[a-zA-Z0-9-.]+$`)
	if !validCommonName.MatchString(disconnectList[1]) {
		return "", errors.New("INVALID_PARAMETER")
	}

	return disconnectList[1], nil
}
