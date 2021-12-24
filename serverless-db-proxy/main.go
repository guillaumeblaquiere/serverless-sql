package main

import (
	"crypto/tls"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	ConnHost     = ""
	ConnPort     = "3306"
	ConnType     = "tcp"
	BufferLength = 1024
)

var targetUrl *url.URL
var globalCtx context.Context
var globalContextCancelled bool
var noTLS bool
var client *http.Client

func main() {

	// TODO use param instead
	// Get the URL to reach
	var err error
	URL := os.Getenv("URL")
	if URL == "" {
		fmt.Println("No URL set as environment variable. Considering proxy deactivation. Stop here")
		os.Exit(0)
	}

	targetUrl, err = url.Parse(URL)
	if err != nil {
		fmt.Printf("bad target URL format: %s\n", err.Error())
		os.Exit(2)
	}

	// Get the noTLS environment variable, especially for local tests
	noTlSEnv := os.Getenv("NOTLS")
	if strings.ToLower(noTlSEnv) == "true" {
		noTLS = true
	}
	fmt.Printf("service URL is %s. TLS mode is %v\n", URL, noTLS)

	// Manage graceful termination
	var cancel context.CancelFunc
	globalCtx = context.Background()
	globalCtx, cancel = context.WithCancel(globalCtx)
	globalContextCancelled = false

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go gracefulTermination(sigs, cancel)

	cred, err := google.FindDefaultCredentials(globalCtx)
	client, err = idtoken.NewClient(globalCtx, URL, option.With)
	if err != nil {
		log.Fatalln(err)
	}

	// Listen for incoming connections.
	l, err := net.Listen(ConnType, ConnHost+":"+ConnPort)
	if err != nil {
		fmt.Printf("Error listening: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Listening on %s:%s\n", ConnHost, ConnPort)

	// Close the listener in case of application stop
	go func() {
		<-globalCtx.Done()
		l.Close()
	}()

	// Wait and accept incoming connections
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			// Graceful exit. Exit
			if globalContextCancelled {
				fmt.Println("Stop http2 listening.")
				break
			}
			fmt.Printf("Error accepting: %s\n", err)
			break
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn)
	}
}

func gracefulTermination(sigs chan os.Signal, cancel context.CancelFunc) {
	sig := <-sigs
	fmt.Printf("Signal received %s; Cancel the global context\n", sig)

	cancel()
	globalContextCancelled = true
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {

	fmt.Printf("New connection to %s from %s\n", targetUrl.String(), conn.RemoteAddr())

	reader, writer := io.Pipe()

	// TODO add security header
	req := &http.Request{
		Method: "POST",
		URL:    targetUrl,
		Header: http.Header{},
		Body:   ioutil.NopCloser(reader),
	}

	//client := http.DefaultClient
	client.Transport = &http2.Transport{}

	if noTLS {
		client.Transport = &http2.Transport{
			// So http2.Transport doesn't complain the URL scheme isn't 'https'
			AllowHTTP: true,
			// Pretend we are dialing a TLS endpoint.
			// Note, we ignore the passed tls.Config
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending handshake: %s\n", err)
		return
	}

	// Manage connection lifecycle
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		select {
		case <-ctx.Done():
			fmt.Println("Connection context cancelled")
		case <-globalCtx.Done():
			fmt.Println("Global context cancelled")
		}
		conn.Close()
		resp.Body.Close()
	}()

	go copyChannel(resp.Body, conn, cancel)
	copyChannel(conn, writer, cancel)
}

func copyChannel(in io.Reader, out io.Writer, cancel context.CancelFunc) {
	for {
		buf := make([]byte, BufferLength)
		// Read the incoming connection into the buffer.
		readLen, err := in.Read(buf)
		if err == io.EOF {
			fmt.Println("Connection closed by remote. Bye!")
			cancel()
			return
		}

		if err == io.ErrClosedPipe {
			fmt.Println("Connection closed. Bye!")
			return
		}

		if err != nil {
			fmt.Printf("Error reading: %s\n", err)
			cancel()
			return
		}

		// Forward the data to the output
		out.Write(buf[:readLen])

		// Flush HTTP communication if it's the correct type
		v, ok := interface{}(out).(http.Flusher)
		if ok {
			v.Flush()
		}
	}
}
