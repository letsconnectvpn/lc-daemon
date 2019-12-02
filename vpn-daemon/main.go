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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// the CA and server certificate are stored in "tlsCertDir", the private key is
// stored tlsKeyDir. The application state data is stored in dataDir.
var (
	tlsCertDir = "."
	tlsKeyDir  = "."
	dataDir    = filepath.Join(".", "data")
)

type vpnClientInfo struct {
	commonName string
	ipFour     string
	ipSix      string
}

type commonNameInfo struct {
	Version     int
	ProfileList []string
}

func main() {
	var hostPort = flag.String("listen", "127.0.0.1:41194", "IP:port to listen on")
	var enableTls = flag.Bool("enable-tls", false, "enable TLS")
	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	clientListener, err := getClientListener(*enableTls, *hostPort)
	if err != nil {
		log.Fatal(err)
	}

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
		return tls.Listen("tcp", hostPort, getTlsConfig())
	}

	return net.Listen("tcp", hostPort)
}

func handleClientConnection(clientConnection net.Conn) {
	defer clientConnection.Close()

	managementIntPortList := []int{}
	setPortsRegExp := regexp.MustCompile(`^SET_PORTS [0-9]+( [0-9]+)*$`)
	disconnectRegExp := regexp.MustCompile(`^DISCONNECT [a-zA-Z0-9-.]+( [a-zA-Z0-9-.]+)*$`)
	setupRegExp := regexp.MustCompile(`^SETUP [a-zA-Z0-9-.]+( [a-zA-Z0-9-.]+)*$`)
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
			// as "DISCONNECT" has no need to pass information back here, we
			// use a WaitGroup instead of a channel...
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
			vpnClientInfoChannel := make(chan []*vpnClientInfo, len(managementIntPortList))
			for _, managementIntPort := range managementIntPortList {
				go obtainStatus(managementIntPort, vpnClientInfoChannel)
			}

			vpnClientConnectionCount := 0
			vpnClientConnectionList := ""

			for range managementIntPortList {
				vpnClientInfoList := <-vpnClientInfoChannel
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

		// SETUP
		if setupRegExp.MatchString(text) {
			commonName := strings.Fields(text)[1]

			// create the directory that contains all the "CN" files, luckily
			// if the directory already exists, this call does nothing
			if nil != os.MkdirAll(filepath.Join(dataDir, "c"), 0700) {
				writer.WriteString(fmt.Sprintf("ERR: DIR_CREATE_ERROR\n"))
				writer.Flush()
				continue
			}

			// open the file to which to write the list of profiles...
			// we MUST set the O_TRUNC flag as well to make sure old data does
			// not remain after the data we write if that happens to be less
			// than is in the current file...
			commonNameFile, err := os.OpenFile(filepath.Join(dataDir, "c", commonName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: FILE_OPEN_ERROR\n"))
				writer.Flush()
				continue
			}

			cI := commonNameInfo{1, strings.Fields(text)[2:]}
			b, err := json.Marshal(cI)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: JSON_MARSHALL_ERROR\n"))
				writer.Flush()
				continue
			}

			_, err = commonNameFile.Write(b)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: FILE_WRITE_ERROR\n"))
				writer.Flush()
				continue
			}

			// we cannot defer file closing, as there is no "return" here on
			// which the Close() could possibly be triggered...
			if nil != commonNameFile.Close() {
				writer.WriteString(fmt.Sprintf("ERR: FILE_CLOSE_ERROR\n"))
				writer.Flush()
				continue
			}

			writer.WriteString(fmt.Sprintf("OK: 0\n"))
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

func getManagementConnection(managementPort int) (net.Conn, error) {
	managementConnection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", managementPort), time.Second*10)
	if err != nil {
		// problem establishing connection (timeout, closed, ...)
		fmt.Println(fmt.Sprintf("WARNING: %s", err))
		return nil, err
	}

	// make sure the connection does not hang forever reading/writing
	managementConnection.SetReadDeadline(time.Now().Add(time.Second * 3))

	return managementConnection, nil
}

func disconnectClient(managementPort int, commonNameList []string, wg *sync.WaitGroup) {
	defer wg.Done()
	managementConnection, err := getManagementConnection(managementPort)
	if err != nil {
		return
	}
	defer managementConnection.Close()

	managementPortScanner := bufio.NewScanner(managementConnection)
	// disconnect all CNs one-by-one
	for _, commonName := range commonNameList {
		fmt.Fprintf(managementConnection, "kill %s\n", commonName)
		for managementPortScanner.Scan() {
			text := managementPortScanner.Text()
			if strings.Index(text, "ERROR") == 0 || strings.Index(text, "SUCCESS") == 0 {
				// move on to next CN...
				break
			}
		}
	}
}

func obtainStatus(managementPort int, vpnClientInfoChannel chan []*vpnClientInfo) {
	managementConnection, err := getManagementConnection(managementPort)
	if err != nil {
		vpnClientInfoChannel <- []*vpnClientInfo{}
		return
	}
	defer managementConnection.Close()

	vpnClientInfoList := []*vpnClientInfo{}
	fmt.Fprintf(managementConnection, "status 2\n")
	managementPortScanner := bufio.NewScanner(managementConnection)
	for managementPortScanner.Scan() {
		text := managementPortScanner.Text()
		if strings.Index(text, "END") == 0 {
			break
		}
		if strings.Index(text, "CLIENT_LIST") == 0 {
			// HEADER,CLIENT_LIST,Common Name,Real Address,Virtual Address,
			//      Virtual IPv6 Address,Bytes Received,Bytes Sent,
			//      Connected Since,Connected Since (time_t),Username,
			//      Client ID,Peer ID
			strList := strings.Split(text, ",")
			if strList[1] != "UNDEF" {
				// only add clients with CN != "UNDEF"
				vpnClientInfoList = append(vpnClientInfoList, &vpnClientInfo{strList[1], strList[3], strList[4]})
			}
		}
	}

	vpnClientInfoChannel <- vpnClientInfoList
}

func parseManagementPortList(managementStringPortList []string) ([]int, error) {
	managementIntPortList := []int{}
	for _, managementStringPort := range managementStringPortList {
		uintPort, err := strconv.ParseUint(managementStringPort, 10, 16)
		if err != nil || uintPort == 0 {
			return nil, errors.New("INVALID_PORT")
		}

		managementIntPortList = append(managementIntPortList, int(uintPort))
	}

	return managementIntPortList, nil
}

func getTlsConfig() *tls.Config {
	caCertFile := filepath.Join(tlsCertDir, "ca.crt")
	certFile := filepath.Join(tlsCertDir, "server.crt")
	keyFile := filepath.Join(tlsKeyDir, "server.key")

	keyPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}

	caCertPem, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatal(err)
	}

	trustedCaPool := x509.NewCertPool()
	if !trustedCaPool.AppendCertsFromPEM(caCertPem) {
		log.Fatal("unable to add CA certificate to CA pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    trustedCaPool,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
	}
}
