package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

const (
	ConnHost     = ""
	ConnType     = "tcp"
	BufferLength = 1024
)

var (
	targetUrl                *url.URL             // URL of the http service to reach.
	port                     int                  // The local port to listen for database connections
	globalCtx                context.Context      // Global context to close all the connection in case ot sig TERM or INT
	isGlobalContextCancelled bool                 // Has the global context been cancelled or not?
	isWithTLS                bool                 // Does the HTTP/2 require TLS or not (h2c, for local testing)?
	ts                       oauth2.TokenSource   // Identity token source to add in the request authorization header
	client                   = http.DefaultClient // Global HTTP client definition
)

func main() {

	URL := flag.String("url", "", "Endpoint URL where is deployed the serverless database. If empty "+
		"or not set, the proxy exit gracefully")
	noTLS := flag.Bool("no-tls", false, "Deactivate TLS support for HTTP/2. False per default, only "+
		"for local tests")
	flag.IntVar(&port, "port", 3306, "The local port to listen for database connections, must be "+
		"between 1000 and 65535")
	flag.Parse()

	// Get the URL to reach
	var err error
	if *URL == "" {
		fmt.Println("No URL set. Consider the wish to not use it. Exit gracefully. Bye")
		os.Exit(0)
	}

	targetUrl, err = url.Parse(*URL)
	if err != nil {
		fmt.Printf("bad URL format: %s\n", err.Error())
		os.Exit(2)
	}

	if port > 65535 || port < 1000 {
		fmt.Printf("port is out of range: %d\n", port)
		flag.PrintDefaults()
		os.Exit(3)
	}

	// Set the TLS support mode
	isWithTLS = !*noTLS

	fmt.Printf("service URL is %s. TLS mode is %v\n", *URL, isWithTLS)

	if isWithTLS {
		client.Transport = &http2.Transport{}
	} else {
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

	// Manage graceful termination
	var cancel context.CancelFunc
	globalCtx = context.Background()
	globalCtx, cancel = context.WithCancel(globalCtx)
	isGlobalContextCancelled = false

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go gracefulTermination(sigs, cancel)

	// Manage Cloud Run authentication with identity token support
	//FIXME only service account key file and GCP metadata server. Need to several lib updates for impersonation support
	ts, err = idtoken.NewTokenSource(globalCtx, *URL)
	if err != nil {
		fmt.Printf("Impossible to create an identity token source because of err %s. The process continues "+
			"without authentication\n", err)
		ts = nil
	}

	// Listen for incoming connections.
	l, err := net.Listen(ConnType, fmt.Sprintf("%s:%d", ConnHost, port))
	if err != nil {
		fmt.Printf("Error listening: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Listening on %s:%d\n", ConnHost, port)

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
			if isGlobalContextCancelled {
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
	isGlobalContextCancelled = true
}

// Handles incoming requests.
func handleRequest(conn net.Conn) {

	fmt.Printf("New connection to %s from %s\n", targetUrl.String(), conn.RemoteAddr())

	reader, writer := io.Pipe()

	// Add, if possible authentication to the HTTP call
	headers := http.Header{}
	if ts != nil {
		token, err := ts.Token()
		if err != nil {
			fmt.Printf("Impossible to create an identity token source because of err %s. The process continues "+
				"without authentication\n", err)
		} else {
			headers.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
		}
	}

	// Make the request
	req := &http.Request{
		Method: "POST",
		URL:    targetUrl,
		Header: headers,
		Body:   ioutil.NopCloser(reader),
	}

	// Perform the request and check the status
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending handshake: %s\n", err)
		conn.Close()
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("invalid request. HTTP code: %s\n", resp.Status)
		conn.Close()
		return
	}

	// Manage context
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Manage lifecycle on context
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

	// Bidirectional proxy connections.
	go copyChannel(resp.Body, conn, cancel)
	copyChannel(conn, writer, cancel)
}

// Proxy the connection: Copy the data from the source and write them to the destination.
// Exit on channel close, and cancel the context if detected.
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
