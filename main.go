package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	tls "github.com/jfreeland/utls"
	"golang.org/x/net/http2"
)

// Bits borrowed from github.com/rakyll/hey and my ever-so-slightly modified
// version of https://github.com/refraction-networking/utls.

const headerRegexp = `^([\w-]+):\s*(.+)`

type headerSlice []string

func (h *headerSlice) String() string {
	return fmt.Sprintf("%s", *h)
}

func (h *headerSlice) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func parseInputWithRegexp(input, regx string) ([]string, error) {
	re := regexp.MustCompile(regx)
	matches := re.FindStringSubmatch(input)
	if len(matches) < 1 {
		return nil, fmt.Errorf("could not parse the provided input; input = %v", input)
	}
	return matches, nil
}

func main() {
	var (
		ip, host, path string
		delay, portNum int
		hs             headerSlice
	)
	flag.StringVar(&ip, "ip", "", "ip address of the host to target")
	flag.StringVar(&host, "host", "", "http host header to request")
	flag.StringVar(&path, "path", "/", "http path to request")
	flag.IntVar(&delay, "delay", 0, "delay after ClientHello in seconds")
	flag.IntVar(&portNum, "port", 443, "tcp port to target")
	flag.Var(&hs, "H", "")
	flag.Parse()

	headers := make(map[string]string)
	for _, h := range hs {
		match, err := parseInputWithRegexp(h, headerRegexp)
		if err != nil {
			log.Fatal("invalid header")
		}
		headers[match[1]] = match[2]
	}

	log.Println(ip)
	log.Println(host)
	if ip == "" || host == "" {
		log.Fatal("must provide ip and host")
	}

	port := strconv.Itoa(portNum)
	remoteAddress := strings.Join([]string{ip, port}, ":")
	tcpConn, err := net.Dial("tcp", remoteAddress)
	defer tcpConn.Close()
	if err != nil {
		log.Fatal(err)
	}

	uTlsConn := tls.UClient(tcpConn, &tls.Config{ServerName: host}, tls.HelloRandomized)
	defer uTlsConn.Close()
	log.Println("connection created")

	log.Println("building handshake")
	err = uTlsConn.BuildHandshakeState()

	if err != nil {
		log.Fatal(err)
	}

	cRandom := []byte{
		100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
		110, 111, 112, 113, 114, 115, 116, 117, 118, 119,
		120, 121, 122, 123, 124, 125, 126, 127, 128, 129,
		130, 131,
	}
	uTlsConn.SetClientRandom(cRandom)
	uTlsConn.Delays.AfterClientHello = delay
	err = uTlsConn.Handshake()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("sending http request")
	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Host: host + path},
		Header: make(http.Header),
		Host:   host,
	}
	for header, value := range headers {
		req.Header.Set(header, value)
	}

	switch uTlsConn.HandshakeState.ServerHello.AlpnProtocol {
	case "h2":
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0

		tr := http2.Transport{}
		cConn, err := tr.NewClientConn(uTlsConn)
		if err != nil {
			log.Fatal(err)
		}
		resp, err := cConn.RoundTrip(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		log.Println(resp)
		log.Println(resp.Body)
	case "http/1.1", "":
		req.Proto = "HTTP/1.1"
		req.ProtoMajor = 1
		req.ProtoMinor = 1

		err := req.Write(uTlsConn)
		if err != nil {
			log.Fatal(err)
		}
		resp, err := http.ReadResponse(bufio.NewReader(uTlsConn), req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		log.Println(resp)
		log.Println(resp.Body)
	}
}
