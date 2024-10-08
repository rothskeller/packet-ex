package server

import (
	"fmt"
	"net"
	"net/http"

	"github.com/skip2/go-qrcode"
)

var listenURL string

// Listen listens on the server port.
func Listen(addr string) (listener net.Listener, err error) {
	if addr == "" {
		addr = ":8000"
	}
	if listener, err = net.Listen("tcp", addr); err != nil {
		return nil, fmt.Errorf("listening on server port: %w", err)
	}
	return listener, nil
}

// Start creates a new server
func Start(listener net.Listener) {
	http.Handle("/message/", http.HandlerFunc(ServeMessage))
	http.Handle("/qrcode.png", http.HandlerFunc(ServeQRCode))
	addr := listener.Addr().(*net.TCPAddr)
	if addr.IP.IsUnspecified() {
		if ifcs, err := net.InterfaceAddrs(); err == nil {
			for _, ifc := range ifcs {
				if ip, ok := ifc.(*net.IPNet); ok && !ip.IP.IsLoopback() {
					if ip4 := ip.IP.To4(); ip4 != nil {
						listenURL = fmt.Sprintf("http://%s:%d/", ip.IP, addr.Port)
						fmt.Printf("Exercise server listening on %s\n", listenURL)
					}
				}
			}
		}
	} else {
		fmt.Printf("Exercise server listening on http://%s/\n", addr)
	}
	go http.Serve(listener, nil)
}

func ServeQRCode(w http.ResponseWriter, r *http.Request) {
	if listenURL == "" {
		http.Error(w, "no listen URL available", http.StatusBadRequest)
		return
	}
	png, err := qrcode.Encode(listenURL, qrcode.Highest, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "nostore")
	w.Write(png)
}
