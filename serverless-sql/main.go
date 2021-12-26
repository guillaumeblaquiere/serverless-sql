package main

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/net/http2"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	mysqlIP      = "127.0.0.1"
	mysqlPort    = "3306"
	BufferLength = 1024
)

var (
	globalCtx                context.Context // Global context to close all the connection in case ot sig TERM or INT
	isGlobalContextCancelled bool            // Has the global context been cancelled or not?
)

func main() {

	// Create HTTP/2 server
	port := "8080"
	if v := os.Getenv("PORT"); v != "" {
		port = v
	}
	addr := net.JoinHostPort("", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("impossible to listen port %s. Error %s\n", port, err)
		os.Exit(1)
	}
	fmt.Println("starting http/2 wrapper server")

	// Manage graceful termination
	var cancel context.CancelFunc
	globalCtx = context.Background()
	globalCtx, cancel = context.WithCancel(globalCtx)
	isGlobalContextCancelled = false

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go gracefulTermination(sigs, cancel)

	// Close the listener in case of application stop
	go func() {
		<-globalCtx.Done()
		lis.Close()
	}()

	// Manage Handlers
	server := http2.Server{}
	http.HandleFunc("/", ProxyListener)
	opts := &http2.ServeConnOpts{
		Handler: http.DefaultServeMux,
	}
	for {
		conn, err := lis.Accept()
		if err != nil {
			// Graceful exit. Exit
			if isGlobalContextCancelled {
				fmt.Println("Stop http2 listening.")
				break
			}
			fmt.Printf("failed to accept connection: %s\n", err)
			break
		}
		go server.ServeConn(conn, opts)
	}
}

func gracefulTermination(sigs chan os.Signal, cancel context.CancelFunc) {
	sig := <-sigs
	fmt.Printf("Signal received %s; Cancel the global context\n", sig)

	cancel()
	isGlobalContextCancelled = true
}

// Loop 30 seconds to establish a connection with the local MySQL server.
// Test the connection status every 50ms (in case of cold start, it can take time)
func establishMysqlConnection() (conn net.Conn, err error) {

	timeout := false
	go func() {
		time.Sleep(30 * time.Second)
		timeout = true
	}()

	for {
		conn, err = net.Dial("tcp", fmt.Sprintf("%s:%s", mysqlIP, mysqlPort))
		if err != nil {
			time.Sleep(50 * time.Millisecond)
		} else {
			fmt.Println("Connection Established")
			break
		}
		if timeout {
			err = errors.New("mysql connection timeout")
			fmt.Printf("error, %s\n", err)
			return
		}
	}
	return
}

func ProxyListener(w http.ResponseWriter, r *http.Request) {

	fmt.Println("New connection. Let's establish MySQL connection")
	// Try to establish the connection with the local MySQL database.
	conn, err := establishMysqlConnection()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "impossible to connect to mysql internally\n")
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
		r.Body.Close()
		fmt.Println("Connection closed. Client disconnected")
	}()

	// Bidirectional proxy connections.
	go copyChannel(r.Body, conn, cancel)
	copyChannel(conn, w, cancel)

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
