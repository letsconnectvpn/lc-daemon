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
// stored tlsKeyDir
var (
	tlsCertDir = "."
	tlsKeyDir  = "."
	dataDir    = filepath.Join(".", "data")
	logDir     = filepath.Join(".", "log")
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

type clientLogData struct {
	ProfileID        string
	CommonName       string
	TimeUnix         int
	IPFour           string
	IPSix            string
	BytesTransferred int
	TimeDuration     int
}

func main() {
	var hostPort = flag.String("listen", "127.0.0.1:41194", "IP:port to listen on")
	var enableTls = flag.Bool("enable-tls", false, "enable TLS")
	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	localListener, err := net.Listen("tcp", "127.0.0.1:41195")
	if err != nil {
		log.Fatal(err)
	}

	go handleLocalListener(localListener)

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

func handleLocalListener(localListener net.Listener) {
	for {
		localConnection, err := localListener.Accept()
		if err != nil {
			log.Printf("ERROR: %s\n", err.Error())
			continue
		}

		go handleLocalConnection(localConnection)
	}
}

func handleLocalConnection(localConnection net.Conn) {
	defer localConnection.Close()

	// CLIENT_CONNECT Profile1 9b8acc27bec2d5beb06c78bcd464d042 1234567890 10.52.58.2 fdbf:4dff:a892:1572::1000
	// CLIENT_DISCONNECT Profile1 9b8acc27bec2d5beb06c78bcd464d042 1234567890 10.52.58.2 fdbf:4dff:a892:1572::1000 605666 9777056 120
	clientConnectRegExp := regexp.MustCompile(`^CLIENT_CONNECT [a-zA-Z0-9-.]+ [a-zA-Z0-9-.]+ [0-9]+ [0-9-.]+ [a-fA-F0-9-:]+$`)
	clientDisconnectRegExp := regexp.MustCompile(`^CLIENT_DISCONNECT [a-zA-Z0-9-.]+ [a-zA-Z0-9-.]+ [0-9]+ [0-9-.]+ [a-fA-F0-9-:]+ [0-9]+ [0-9]+ [0-9]+$`)
	writer := bufio.NewWriter(localConnection)
	scanner := bufio.NewScanner(localConnection)

	for scanner.Scan() {
		text := scanner.Text()
		log.Printf("DEBUG: %s\n", text)

		// CLIENT_CONNECT
		if clientConnectRegExp.MatchString(text) {
			clientData, err := parseClientParameters(strings.Fields(text)[1:])
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err.Error()))
				writer.Flush()
				continue
			}

			err = checkClientPermission(clientData)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err.Error()))
				writer.Flush()
				continue
			}

			err = connectLogTransaction(clientData)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err.Error()))
				writer.Flush()
				continue
			}

			writer.WriteString(fmt.Sprintf("OK: 0\n"))
			writer.Flush()
			continue
		}

		// CLIENT_DISCONNECT
		if clientDisconnectRegExp.MatchString(text) {
			clientData, err := parseClientParameters(strings.Fields(text)[1:])
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err.Error()))
				writer.Flush()
				continue
			}

			err = disconnectLogTransaction(clientData)
			if err != nil {
				writer.WriteString(fmt.Sprintf("ERR: %s\n", err.Error()))
				writer.Flush()
				continue
			}

			writer.WriteString(fmt.Sprintf("OK: 0\n"))
			writer.Flush()
			continue
		}

		writer.WriteString(fmt.Sprintf("ERR: NOT_SUPPORTED\n"))
		writer.Flush()
	}
}

func parseClientParameters(parameterStringList []string) (*clientLogData, error) {
	if len(parameterStringList) != 5 && len(parameterStringList) != 8 {
		return nil, fmt.Errorf("INVALID_PARAMETERS")
	}

	var clientData clientLogData
	clientData.ProfileID = parameterStringList[0]
	clientData.CommonName = parameterStringList[1]

	timeUint, err := strconv.ParseUint(parameterStringList[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("INVALID_TIMEUNIX_`%s`", parameterStringList[2])
	}
	clientData.TimeUnix = int(timeUint)

	ip4 := net.ParseIP(parameterStringList[3])
	if ip4 == nil {
		return nil, fmt.Errorf("INVALID_IP4_`%s`", parameterStringList[3])
	}
	clientData.IPFour = ip4.String()

	ip6 := net.ParseIP(parameterStringList[4])
	if ip6 == nil {
		return nil, fmt.Errorf("INVALID_IP6_`%s`", parameterStringList[4])
	}
	clientData.IPSix = ip6.String()

	// Return for client_connect
	if len(parameterStringList) == 5 {
		clientData.BytesTransferred = 0
		clientData.TimeDuration = 0
		return &clientData, nil
	}

	bytesReceivedUint, err := strconv.ParseUint(parameterStringList[5], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("INVALID_BYTES_RECEIVED_`%s`", parameterStringList[5])
	}

	bytesSentUint, err := strconv.ParseUint(parameterStringList[6], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("INVALID_BYTES_SENT_`%s`", parameterStringList[6])
	}

	clientData.BytesTransferred = int(bytesReceivedUint) + int(bytesSentUint)

	timeDurationUint, err := strconv.ParseUint(parameterStringList[7], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("INVALID_TIME_DURATION_`%s`", parameterStringList[7])
	}
	clientData.TimeDuration = int(timeDurationUint)

	return &clientData, nil
}

func checkClientPermission(clientData *clientLogData) error {
	jsonBytes, err := ioutil.ReadFile(filepath.Join(dataDir, "c", clientData.CommonName))
	if err != nil {
		return fmt.Errorf("UNABLE_TO_READ_PERMISSIONFILE")
	}

	var commonNameJSON commonNameInfo
	err = json.Unmarshal(jsonBytes, &commonNameJSON)
	if err != nil {
		return fmt.Errorf("UNABLE_TO_UNMARSHAL_PERMISSIONFILE")
	}

	if !valueExistsInArray(commonNameJSON.ProfileList, clientData.ProfileID) {
		return fmt.Errorf("CLIENT_NOT_ALLOWED_TO_CONNECT")
	}
	return nil
}

func valueExistsInArray(array []string, value string) bool {
	for _, item := range array {
		if item == value {
			return true
		}
	}

	return false
}

func connectLogTransaction(LogData *clientLogData) error {
	b, err := json.Marshal(LogData)
	if err != nil {
		return errors.New("JSON_MARSHAL_ERROR")
	}

	if nil != os.MkdirAll(filepath.Join(logDir, LogData.IPFour), 0700) {
		return errors.New("DIR_CREATE_ERROR")
	}

	fileName := filepath.Join(logDir, LogData.IPFour, strconv.Itoa(LogData.TimeUnix))
	_, err = os.Stat(fileName)
	if err == nil {
		return errors.New("LOG_FILE_ALREADY_EXISTS")
	}

	logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errors.New("FILE_CREATE_ERROR")
	}

	defer logFile.Close()

	_, err = logFile.Write(b)
	if err != nil {
		//try to remove the file if writing to file fails
		_ = os.Remove(fileName)
		return errors.New("FILE_WRITING_ERROR")
	}

	return nil
}

func disconnectLogTransaction(LogData *clientLogData) error {
	/*
		***If the file really does not exist, just create a new file with the updated fields???, no need to search and match the file/contents***
			_, err := os.Stat(fileName)
			if err != nil {
				return errors.New("LOGFILE_NOT_ACCESSIBLE")
			}
	*/
	fileName := filepath.Join(logDir, LogData.IPFour, strconv.Itoa(LogData.TimeUnix))
	jsonBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.New("UNABLE_TO_READ_LOGFILE")
	}

	var JSONContents clientLogData
	err = json.Unmarshal(jsonBytes, &JSONContents)
	if err != nil {
		return errors.New("UNABLE_TO_UNMARSHAL_LOGFILE")
	}

	if JSONContents.CommonName != LogData.CommonName || JSONContents.ProfileID != LogData.ProfileID {
		return errors.New("CONFLICT_LOGFILE_AND_DISCONNECT-DATA")
	}

	b, err := json.Marshal(LogData)
	if err != nil {
		return errors.New("UNABLE_TO_GET_JSONFORMAT_FROM_DATA")
	}

	logFile, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errors.New("UNABLE_TO_OPEN_LOGFILE")
	}

	defer logFile.Close()

	_, err = logFile.Write(b)
	if err != nil {
		return errors.New("UNABLE_TO_WRITE_LOGFILE")
	}

	return nil
}
