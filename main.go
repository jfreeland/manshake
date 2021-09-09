package main

import (
	"github.com/refraction-networking/utls"
)

func main() {
	conn := utls.UClient("tcp", "1.1.1.1:443")
	defer conn.Close()
	// conn = tls.Client(conn, config)
	// err = conn.(*tls.Conn).Handshake()
	// if err != nil {
	// 	log.Fatal(err)
	// }
}
