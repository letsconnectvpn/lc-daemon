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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// the CA and server certificate are stored in "pkiDir", the private key is
// stored in a sub directory "private"...
var pkiDir = "."

type vpnClientInfo struct {
	commonName string
	ipFour     string // XXX use IP type?
	ipSix      string // XXX use IP type?
}

func main() {
	var hostPort = flag.String("listen", "127.0.0.1:41194", "IP:port to listen on")
	var enableTls = flag.Bool("enable-tls", false, "enable TLS")
	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	clientListener, err := getClientListener(*enableTls, *hostPort)
	fatalIfError(err)

	for {
		clientConnection, err := clientListener.Accept()
		if err != nil {
			fmt.Println(fmt.Sprintf("ERROR: %s", err))
			continue
		}

		go handleClientConnection(clientConnection)
	}
}

func getClientListener(enableTls bool, hostPort string) (net.Listener, error) {
	if enableTls {
		return tls.Listen("tcp", hostPort, getTLSConfig())
	}

	return net.Listen("tcp", hostPort)
}

func handleClientConnection(clientConnection net.Conn) {
	defer clientConnection.Close()

	managementIntPortList := []int{}
	setPortsRegExp := regexp.MustCompile(`^SET_PORTS [0-9]+( [0-9]+)*$`)
	disconnectRegExp := regexp.MustCompile(`^DISCONNECT [a-zA-Z0-9-.]+( [a-zA-Z0-9-.]+)*$`)
	writer := bufio.NewWriter(clientConnection)
	scanner := bufio.NewScanner(clientConnection)

	for scanner.Scan() {
		text := scanner.Text()
		fmt.Println(fmt.Sprintf("DEBUG: %s", text))

		// SET_PORTS
		if setPortsRegExp.MatchString(text) {
			parsedPortList, err := parseManagementPortList(strings.Fields(text)[1:])
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err))
				writer.Flush()
				continue
			}

			managementIntPortList = parsedPortList
			writer.WriteString(fmt.Sprintf("OK: 0\n"))
			writer.Flush()
			continue
		}

		// DISCONNECT
		if disconnectRegExp.MatchString(text) {
			commonNameList := strings.Fields(text)[1:]
			var wg sync.WaitGroup
			for _, managementIntPort := range managementIntPortList {
				wg.Add(1)
				go disconnectClient(managementIntPort, commonNameList, &wg)
			}
			wg.Wait()
			writer.WriteString(fmt.Sprintf("OK: 0\n"))
			writer.Flush()
			continue
		}

		// LIST
		if text == "LIST" {
			c := make(chan []*vpnClientInfo, len(managementIntPortList))
			for _, managementIntPort := range managementIntPortList {
				go obtainStatus(managementIntPort, c)
			}

			vpnClientConnectionCount := 0
			vpnClientConnectionList := ""

			for range managementIntPortList {
				vpnClientInfoList := <-c
				for _, vpnClientInfo := range vpnClientInfoList {
					vpnClientConnectionCount++
					vpnClientConnectionList += fmt.Sprintf("%s %s %s\n", vpnClientInfo.commonName, vpnClientInfo.ipFour, vpnClientInfo.ipSix)
				}
			}

			writer.WriteString(fmt.Sprintf("OK: %d\n", vpnClientConnectionCount))
			writer.WriteString(vpnClientConnectionList)
			writer.Flush()
			continue
		}

		// QUIT
		if text == "QUIT" {
			writer.WriteString(fmt.Sprintf("OK: 0\n"))
			writer.Flush()
			return
		}

		writer.WriteString(fmt.Sprintf("ERR: NOT_SUPPORTED\n"))
		writer.Flush()
	}
}

func getConnection(managementPort int) (net.Conn, error) {
	// XXX this is all quite ugly with error handling
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", managementPort), time.Second*10)
	if err != nil {
		// timeout error, port was busy with another connection
		// the port was not listening for connections
		// port refused connection
		return nil, err
	}

	// XXX figure out return value properly
	//  conn.SetReadDeadline(time.Now().Add(time.Second*3))
	//    if foo != nil {
	//        return nil, err
	//    }

	return conn, nil
}

func disconnectClient(managementPort int, commonNameList []string, wg *sync.WaitGroup) {
	defer wg.Done()
	managementConnection, err := getConnection(managementPort)
	if err != nil {
		fmt.Println(fmt.Sprintf("WARNING: %s", err))
		return
	}
	defer managementConnection.Close()

	managementPortScanner := bufio.NewScanner(managementConnection)
	for _, commonName := range commonNameList {
		// send "kill" command
		fmt.Fprintf(managementConnection, fmt.Sprintf("kill %s\n", commonName))
		for managementPortScanner.Scan() {
			// we read until we either get SUCCESS or ERROR
			text := managementPortScanner.Text()
			if 0 == strings.Index(text, "ERROR") || 0 == strings.Index(text, "SUCCESS") {
				// we are done, move on to the next commonName
				break
			}
		}
	}
}

func obtainStatus(managementPort int, c chan []*vpnClientInfo) {
	managementConnection, err := getConnection(managementPort)
	if err != nil {
		c <- []*vpnClientInfo{}
		return
	}
	defer managementConnection.Close()

	vpnClientInfoList := make([]*vpnClientInfo, 0)

	// send "status" command
	fmt.Fprintf(managementConnection, "status 2\n")

	managementPortScanner := bufio.NewScanner(managementConnection)
	for managementPortScanner.Scan() {
		text := managementPortScanner.Text()
		if 0 == strings.Index(text, "END") {
			// end reached
			break
		}
		if 0 == strings.Index(text, "CLIENT_LIST") {
			strList := strings.Split(text, ",")
			//x[0] = "CLIENT_LIST"				x[1] = {COMMON NAME}				x[2] = {Real Address}
			//x[3] = {Virtual IPv4 Address}		x[4] = {Virtual IPv6 Address}		x[5] = {Bytes Received}
			//x[6] = {Bytes Sent}				x[7] = {Connected Since}			x[8] = {Connected Since (time_t)}
			//x[9] = {Username}					x[10]= {Client ID}					x[11]= {Peer ID}
			if strList[1] == "UNDEF" {
				// ignore "UNDEF" clients, they are trying to connect, but
				// not (yet) connected...
				continue
			}

			vpnClientInfoList = append(vpnClientInfoList, &vpnClientInfo{strList[1], strList[3], strList[4]})
		}
	}

	c <- vpnClientInfoList
}

func parseManagementPortList(managementStringPortList []string) ([]int, error) {
	managementIntPortList := make([]int, 0)
	for _, managementStringPort := range managementStringPortList {
		uintPort, err := strconv.ParseUint(managementStringPort, 10, 16)
		if err != nil || uintPort == 0 {
			return nil, errors.New("INVALID_PARAMETER")
		}

		managementIntPortList = append(managementIntPortList, int(uintPort))
	}

	return managementIntPortList, nil
}

func getTLSConfig() *tls.Config {
	caFile := filepath.Join(pkiDir, "ca.crt")
	certFile := filepath.Join(pkiDir, "server.crt")
	keyFile := filepath.Join(pkiDir, "private", "server.key")

	keyPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	fatalIfError(err)

	//get PEM data from CA-certificate file
	pemCA, err := ioutil.ReadFile(caFile)
	fatalIfError(err)

	//for authenticating clients, only clients with this as CA will be allowed to connect
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(pemCA) {
		fatalIfError(errors.New("Unable to append the CA PEM to the CA-pool"))
	}

	return &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
	}
}

func fatalIfError(err error) {
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}
