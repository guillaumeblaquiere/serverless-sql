package main

import (
	"crypto/tls"
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
	"strings"
	"syscall"
)

const (
	ConnHost     = ""
	ConnPort     = "3306"
	ConnType     = "tcp"
	BufferLength = 1024
)

var (
	targetUrl                *url.URL             // URL of the http service to reach.
	globalCtx                context.Context      // Global context to close all the connection in case ot sig TERM or INT
	isGlobalContextCancelled bool                 // Has the global context been cancelled or not?
	isWithoutTLS             bool                 // Does the HTTP/2 require TLS or not (h2c, for local testing)?
	client                   = http.DefaultClient // Global HTTP client definition
	ts                       oauth2.TokenSource   // Identity token source to add in the request authorization header
)

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

	// Get the isWithoutTLS environment variable, especially for local tests
	noTlSEnv := os.Getenv("NOTLS")
	if strings.ToLower(noTlSEnv) == "true" {
		isWithoutTLS = true
	}
	fmt.Printf("service URL is %s. TLS mode is %v\n", URL, isWithoutTLS)

	// Create the client
	client.Transport = &http2.Transport{}

	if isWithoutTLS {
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
	ts, err = idtoken.NewTokenSource(globalCtx, URL)
	if err != nil {
		fmt.Printf("Impossible to create an identity token source because of err %s. The process continues "+
			"without authentication\n", err)
		ts = nil
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
